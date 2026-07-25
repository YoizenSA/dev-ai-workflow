import { describe, expect, test } from "bun:test"
import * as fs from "node:fs/promises"
import * as os from "node:os"
import * as path from "node:path"
import AdvisorPlugin from "../src/index"
import { loadConfig, loadWatchdog, parseModelRef, readTopLevelFields, toastVariant } from "../src/config"
import { extractText, toTracked } from "../src/messages"

async function tmpdir(): Promise<string> {
  return fs.mkdtemp(path.join(os.tmpdir(), "advisor-test-"))
}

describe("parseModelRef", () => {
  test("accepts provider/model", () => {
    expect(parseModelRef("anthropic/claude-sonnet-5")).toEqual({
      providerID: "anthropic",
      modelID: "claude-sonnet-5",
    })
  })

  // A bare id cannot be resolved without the catalog. Guessing a provider means
  // silently billing a model the user did not choose.
  test.each(["claude-sonnet-5", "", "   ", "/leading", "trailing/", 42, null, undefined])(
    "rejects what it cannot resolve: %p",
    (input) => {
      expect(parseModelRef(input)).toBeUndefined()
    },
  )

  test("keeps slashes inside the model id", () => {
    expect(parseModelRef("openrouter/meta/llama-4")).toEqual({
      providerID: "openrouter",
      modelID: "meta/llama-4",
    })
  })
})

async function writeConfig(body: string): Promise<string> {
  const dir = await tmpdir()
  const cfg = path.join(dir, "config.yaml")
  await fs.writeFile(cfg, body)
  return cfg
}

describe("loadConfig", () => {
  // ywai's config is config.yaml, not JSON. Reading the wrong file is silent:
  // the plugin loads, finds nothing, and disables itself forever.
  test("reads the ywai YAML config", async () => {
    const cfg = await writeConfig("log_level: info\nadvisor_enabled: true\nadvisor_model: anthropic/claude-sonnet-5\n")
    const loaded = await loadConfig(cfg)
    expect(loaded.enabled).toBe(true)
    expect(loaded.model).toEqual({ providerID: "anthropic", modelID: "claude-sonnet-5" })
  })

  test("stays disabled without a model, even when the flag is on", async () => {
    expect((await loadConfig(await writeConfig("advisor_enabled: true\n"))).enabled).toBe(false)
  })

  test("stays disabled with a model but the flag off", async () => {
    expect((await loadConfig(await writeConfig("advisor_model: anthropic/claude-sonnet-5\n"))).enabled).toBe(false)
  })

  test("tolerates quoted values", async () => {
    const cfg = await writeConfig('advisor_enabled: true\nadvisor_model: "anthropic/claude-sonnet-5"\n')
    expect((await loadConfig(cfg)).model?.modelID).toBe("claude-sonnet-5")
  })

  test("a missing or unreadable config disables rather than throws", async () => {
    const dir = await tmpdir()
    expect((await loadConfig(path.join(dir, "nope.yaml"))).enabled).toBe(false)
  })
})

describe("readTopLevelFields", () => {
  test("ignores nested keys that share a name", async () => {
    // A profile block could carry its own advisor_model; only the global
    // setting at column zero may enable a second model on every turn.
    const fields = readTopLevelFields(
      "orchestrator_profiles:\n  fast:\n    advisor_model: sneaky/model\nadvisor_model: real/model\n",
      ["advisor_model"],
    )
    expect(fields.advisor_model).toBe("real/model")
  })

  test("returns nothing for keys that are absent", () => {
    expect(readTopLevelFields("log_level: info\n", ["advisor_model"])).toEqual({})
  })
})

describe("plugin activation", () => {
  test("registers no hooks when disabled", async () => {
    // Without this, every session pays event-hook overhead for a feature that
    // cannot produce anything. The config path is pinned at a file that does
    // not exist: reading the developer's real config would make this pass or
    // fail depending on whether they happen to have the advisor turned on.
    const dir = await tmpdir()
    const hooks = await AdvisorPlugin({ client: {}, directory: "/tmp" } as never, {
      configPath: path.join(dir, "absent.yaml"),
    })
    expect(hooks.event).toBeUndefined()
  })

  test("registers the event hook when a model is configured", async () => {
    const cfg = await writeConfig("advisor_enabled: true\nadvisor_model: test/model\n")
    const hooks = await AdvisorPlugin({ client: {}, directory: "/tmp" } as never, { configPath: cfg })
    expect(typeof hooks.event).toBe("function")
  })
})

describe("loadWatchdog", () => {
  test("reads WATCHDOG.md from the project root", async () => {
    const dir = await tmpdir()
    await fs.writeFile(path.join(dir, "WATCHDOG.md"), "Watch the durable queue.")
    expect(await loadWatchdog(dir)).toContain("durable queue")
  })

  test("falls back to .ywai/WATCHDOG.md", async () => {
    const dir = await tmpdir()
    await fs.mkdir(path.join(dir, ".ywai"))
    await fs.writeFile(path.join(dir, ".ywai", "WATCHDOG.md"), "Nested priorities.")
    expect(await loadWatchdog(dir)).toContain("Nested priorities")
  })

  test("absent or empty yields nothing", async () => {
    const dir = await tmpdir()
    expect(await loadWatchdog(dir)).toBeUndefined()
    await fs.writeFile(path.join(dir, "WATCHDOG.md"), "   \n  ")
    expect(await loadWatchdog(dir)).toBeUndefined()
  })
})

describe("toastVariant", () => {
  test("a nit must not look like a blocker", () => {
    expect(toastVariant("nit")).toBe("info")
    expect(toastVariant("concern")).toBe("warning")
    expect(toastVariant("blocker")).toBe("error")
  })
})

describe("message flattening", () => {
  test("reads id, role, text and tool calls out of an OpenCode message", () => {
    const t = toTracked({
      info: { id: "m1", role: "assistant" },
      parts: [
        { type: "text", text: "rewrote the parser" },
        { type: "tool", tool: "edit", state: { status: "completed", input: { filePath: "src/parse.ts" }, output: "ok" } },
      ],
    })
    expect(t.id).toBe("m1")
    expect(t.role).toBe("assistant")
    expect(t.text).toBe("rewrote the parser")
    // What the agent DID has to reach the reviewer, not only what it says.
    expect(t.tools).toContain("edit(src/parse.ts)")
    expect(t.mutating).toBe(true)
    expect(t.failed).toBe(false)
  })

  test("survives a message with no parts", () => {
    expect(toTracked({ info: { id: "m2", role: "user" } }).text).toBe("")
  })

  test("extractText joins text parts and ignores the rest", () => {
    expect(extractText([{ type: "text", text: " a " }, { type: "reasoning" }, { type: "text", text: "b" }])).toBe("a\nb")
    expect(extractText([])).toBe("")
    expect(extractText(null as never)).toBe("")
  })
})
