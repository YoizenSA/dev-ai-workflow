import { describe, expect, test } from "bun:test"
import {
  hasFailedTool,
  hasMutatingTool,
  isMutatingTool,
  primaryArg,
  renderText,
  renderToolCall,
  renderTools,
} from "../src/render"

// An advisor that only reads prose is reviewing the agent's account of itself.
// These pin what it can see about what actually ran — and what it is spared.

const tool = (name: string, input: Record<string, unknown>, status = "completed", output = "") => ({
  type: "tool" as const,
  tool: name,
  state: { status, input, output },
})

describe("renderToolCall", () => {
  test("names the call and how it ended", () => {
    expect(renderToolCall(tool("read", { filePath: "src/checkout.ts" }, "completed", "a\nb\nc"))).toBe(
      "→ read(src/checkout.ts) ⇒ ok · 3 lines",
    )
  })

  test("a failure carries its first line — the highest-signal thing in a turn", () => {
    const line = renderToolCall(
      tool("bash", { command: "npm test" }, "error", "FAIL src/checkout.test.ts\n  2 failing\n  ..."),
    )
    expect(line).toContain("bash(npm test)")
    expect(line).toContain("error")
    expect(line).toContain("FAIL src/checkout.test.ts")
  })

  test("output never travels — only its size", () => {
    // The judgment needs to know a command ran and how it ended. Shipping the
    // body would cost hundreds of tokens per call to answer what the summary
    // already answers.
    const huge = Array.from({ length: 500 }, (_, i) => `line ${i} of noisy build output`).join("\n")
    const line = renderToolCall(tool("bash", { command: "npm run build" }, "completed", huge))
    expect(line).toBe("→ bash(npm run build) ⇒ ok · 500 lines")
    expect(line.length).toBeLessThan(120)
  })

  test("a pending call reads as pending, not as success", () => {
    expect(renderToolCall(tool("bash", { command: "sleep 30" }, "running"))).toContain("running")
  })

  test("survives a call with no arguments or output", () => {
    expect(renderToolCall({ type: "tool", tool: "todowrite" })).toBe("→ todowrite() ⇒ ok · 0 lines")
  })
})

describe("primaryArg", () => {
  test("picks the argument that identifies the call", () => {
    // read(src/a.ts) says what happened; read(offset: 0, limit: 200) does not.
    expect(primaryArg("read", { offset: 0, limit: 200, filePath: "src/a.ts" })).toBe("src/a.ts")
    expect(primaryArg("bash", { command: "go test ./...", timeout: 5000 })).toBe("go test ./...")
  })

  test("grep shows pattern and scope together", () => {
    expect(primaryArg("grep", { pattern: "TODO", path: "src/" })).toBe("TODO @ src/")
  })

  test("clips a long argument instead of carrying it whole", () => {
    const long = primaryArg("bash", { command: "x".repeat(400) })
    expect(long.length).toBeLessThan(140)
    expect(long).toEndWith("…")
  })

  test("no recognizable argument yields nothing rather than a JSON blob", () => {
    expect(primaryArg("mystery", { count: 3, flag: true })).toBe("")
  })
})

describe("isMutatingTool", () => {
  test.each(["edit", "write", "bash", "patch", "task", "ast_edit"])("%s changes things", (name) => {
    expect(isMutatingTool(name)).toBe(true)
  })

  test.each(["read", "grep", "glob", "list", "webfetch"])("%s only looks", (name) => {
    expect(isMutatingTool(name)).toBe(false)
  })

  test("namespaced plugin tools are judged by their verb", () => {
    expect(isMutatingTool("ado_pr_create")).toBe(true)
    expect(isMutatingTool("codemod_run")).toBe(true)
    expect(isMutatingTool("context7_query_docs")).toBe(false)
  })

  test("an unnamed tool is not assumed to mutate", () => {
    expect(isMutatingTool(undefined)).toBe(false)
  })
})

describe("message-level signals", () => {
  const parts = [
    { type: "text", text: "shipped it" },
    tool("read", { filePath: "a.ts" }),
    tool("edit", { filePath: "a.ts" }),
    tool("bash", { command: "npm test" }, "error", "2 failing"),
  ]

  test("detects a change", () => {
    expect(hasMutatingTool(parts)).toBe(true)
    expect(hasMutatingTool([{ type: "text", text: "hi" }, tool("read", { filePath: "a.ts" })])).toBe(false)
  })

  test("detects a failed call", () => {
    expect(hasFailedTool(parts)).toBe(true)
    expect(hasFailedTool([tool("read", { filePath: "a.ts" })])).toBe(false)
  })

  test("renders every call, one line each", () => {
    expect(renderTools(parts).split("\n")).toHaveLength(3)
  })

  test("text rendering ignores tool and reasoning parts", () => {
    expect(renderText([...parts, { type: "reasoning", text: "thinking out loud" }])).toBe("shipped it")
  })
})
