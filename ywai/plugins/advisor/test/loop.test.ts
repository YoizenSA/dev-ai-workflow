import { beforeEach, describe, expect, test } from "bun:test"
import * as fs from "node:fs/promises"
import * as os from "node:os"
import * as path from "node:path"
import AdvisorPlugin from "../src/index"

// End-to-end over a stubbed OpenCode v2 context: event in, synthetic note out.

type Msg = { info: { id: string; role: string }; parts: { type: string; text: string }[] }

const msg = (id: string, role: string, text: string): Msg => ({
  info: { id, role },
  parts: [{ type: "text", text }],
})

class FakeCtx {
  history: Msg[] = []
  replies: string[] = []
  delivered: { text: string; synthetic: boolean }[] = []
  toasts: { variant: string; message: string }[] = []
  createdSessions: string[] = []
  deletedSessions: string[] = []
  advisorPrompts: string[] = []
  tools: { name: string; input?: unknown; execute: Function }[] = []
  eventHandler?: (event: { type: string; data?: Record<string, unknown> }) => Promise<void> | void
  contextHook?: (session: { sessionID: string; system: unknown[] }) => void
  #seq = 0
  directory: string
  options: { configPath: string }

  constructor(directory: string, configPath: string) {
    this.directory = directory
    this.options = { configPath }
  }

  session = {
    context: async ({ sessionID: _id }: { sessionID: string }) => this.history,
    create: async () => {
      const id = `advisor-${++this.#seq}`
      this.createdSessions.push(id)
      return { id }
    },
    remove: async ({ sessionID }: { sessionID: string }) => {
      this.deletedSessions.push(sessionID)
    },
    prompt: async ({ sessionID, text }: { sessionID: string; text?: string }) => {
      if (sessionID.startsWith("advisor-")) {
        this.advisorPrompts.push(text ?? "")
        const reply = this.replies.shift() ?? "SEVERITY: silent"
        return { text: reply, parts: [{ type: "text", text: reply }] }
      }
      return {}
    },
    synthetic: async ({ sessionID: _id, text }: { sessionID: string; text: string }) => {
      this.delivered.push({ text, synthetic: true })
      this.history.push(msg(`advisory-${this.delivered.length}`, "user", text))
    },
    hook: async (name: string, fn: (session: { sessionID: string; system: unknown[] }) => void) => {
      if (name === "context") this.contextHook = fn
    },
  }

  event = {
    subscribe: async (fn: (event: { type: string; data?: Record<string, unknown> }) => Promise<void> | void) => {
      this.eventHandler = fn
    },
  }

  tool = {
    transform: async (fn: (draft: { add: (name: string, spec: any) => void }) => void) => {
      fn({
        add: (name: string, spec: any) => {
          this.tools.push({ name, input: spec.input, execute: spec.execute })
        },
      })
    },
  }

  tui = {
    showToast: async ({ body }: any) => {
      this.toasts.push({ variant: body.variant, message: body.message })
    },
  }
}

let configPath: string

beforeEach(async () => {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), "advisor-loop-"))
  configPath = path.join(dir, ".ywai", "config.yaml")
  await fs.mkdir(path.dirname(configPath), { recursive: true })
  await fs.writeFile(configPath, "advisor_enabled: true\nadvisor_model: test/advisor-model\n")
})

async function boot(ctx: FakeCtx) {
  expect(AdvisorPlugin.id).toBe("advisor")
  await AdvisorPlugin.setup(ctx)
}

const idle = (sessionID: string) => ({ type: "session.idle", data: { sessionID } })

describe("the review loop", () => {
  test("first idle only seeds the cursor — no review of pre-existing history", async () => {
    const ctx = new FakeCtx("/tmp/project", configPath)
    ctx.history = [msg("1", "user", "old"), msg("2", "assistant", "old reply")]
    ctx.replies = ["SEVERITY: blocker\nNOTE: should never be asked"]

    await boot(ctx)
    await ctx.eventHandler!(idle("s1"))

    expect(ctx.advisorPrompts).toHaveLength(0)
    expect(ctx.delivered).toHaveLength(0)
  })

  test("a concern reaches the transcript and the toast", async () => {
    const ctx = new FakeCtx("/tmp/project", configPath)
    await boot(ctx)
    await ctx.eventHandler!(idle("s1"))

    ctx.history.push(msg("3", "assistant", "deleted the migration and moved on"))
    ctx.replies = ["SEVERITY: concern\nNOTE: The migration runs before the column exists."]
    await ctx.eventHandler!(idle("s1"))

    expect(ctx.delivered).toHaveLength(1)
    expect(ctx.delivered[0]!.synthetic).toBe(true)
    expect(ctx.delivered[0]!.text).toContain('severity="concern"')
    expect(ctx.delivered[0]!.text).toContain("column exists")
    expect(ctx.toasts[0]!.variant).toBe("warning")
  })

  test("silence delivers nothing", async () => {
    const ctx = new FakeCtx("/tmp/project", configPath)
    await boot(ctx)
    await ctx.eventHandler!(idle("s1"))

    ctx.history.push(msg("3", "assistant", "renamed a variable"))
    ctx.replies = ["SEVERITY: silent"]
    await ctx.eventHandler!(idle("s1"))

    expect(ctx.delivered).toHaveLength(0)
    expect(ctx.toasts).toHaveLength(0)
  })

  test("the advisor never reviews its own note", async () => {
    const ctx = new FakeCtx("/tmp/project", configPath)
    await boot(ctx)
    await ctx.eventHandler!(idle("s1"))

    ctx.history.push(msg("3", "assistant", "shipped it"))
    ctx.replies = ["SEVERITY: concern\nNOTE: Verification is thinner than the risk."]
    await ctx.eventHandler!(idle("s1"))

    ctx.history.push(msg("4", "assistant", "another step"))
    ctx.replies = ["SEVERITY: silent"]
    await ctx.eventHandler!(idle("s1"))

    const secondPrompt = ctx.advisorPrompts[1] ?? ""
    expect(secondPrompt).toContain("another step")
    expect(secondPrompt).not.toContain("Verification is thinner")
  })

  test("repeated advice lands once", async () => {
    const ctx = new FakeCtx("/tmp/project", configPath)
    await boot(ctx)
    await ctx.eventHandler!(idle("s1"))

    for (let i = 0; i < 3; i++) {
      ctx.history.push(msg(`w${i}`, "assistant", `work step ${i}`))
      ctx.replies = ["SEVERITY: blocker\nNOTE: The retry loop has no backoff."]
      await ctx.eventHandler!(idle("s1"))
    }

    expect(ctx.delivered).toHaveLength(1)
  })

  test("content-free notes never reach the transcript", async () => {
    const ctx = new FakeCtx("/tmp/project", configPath)
    await boot(ctx)
    await ctx.eventHandler!(idle("s1"))

    for (const noise of ["SEVERITY: blocker\nNOTE: Stop.", "SEVERITY: blocker\nNOTE: Done", "SEVERITY: concern\nNOTE: LGTM"]) {
      ctx.history.push(msg(`n${ctx.delivered.length}${noise.length}`, "assistant", "more work"))
      ctx.replies = [noise]
      await ctx.eventHandler!(idle("s1"))
    }

    expect(ctx.delivered).toHaveLength(0)
  })

  test("an idle with nothing new does not call the advisor model", async () => {
    const ctx = new FakeCtx("/tmp/project", configPath)
    await boot(ctx)
    await ctx.eventHandler!(idle("s1"))

    await ctx.eventHandler!(idle("s1"))
    await ctx.eventHandler!(idle("s1"))

    expect(ctx.advisorPrompts).toHaveLength(0)
  })

  test("the advisor's own child session is cleaned up", async () => {
    const ctx = new FakeCtx("/tmp/project", configPath)
    await boot(ctx)
    await ctx.eventHandler!(idle("s1"))

    ctx.history.push(msg("3", "assistant", "work"))
    ctx.replies = ["SEVERITY: nit\nNOTE: A map would be simpler here."]
    await ctx.eventHandler!(idle("s1"))

    expect(ctx.createdSessions).toHaveLength(1)
    expect(ctx.deletedSessions).toEqual(ctx.createdSessions)
  })

  test("events from the advisor's own session are ignored", async () => {
    const ctx = new FakeCtx("/tmp/project", configPath)
    await boot(ctx)
    await ctx.eventHandler!(idle("advisor-1"))
    expect(ctx.advisorPrompts).toHaveLength(0)
  })
})

describe("failure containment", () => {
  test("an advisor model error never breaks the watched session", async () => {
    const ctx = new FakeCtx("/tmp/project", configPath)
    ctx.session.create = async () => {
      throw new Error("provider is down")
    }
    await boot(ctx)
    await ctx.eventHandler!(idle("s1"))
    ctx.history.push(msg("3", "assistant", "work"))

    await expect(ctx.eventHandler!(idle("s1"))).resolves.toBeUndefined()
    expect(ctx.delivered).toHaveLength(0)
  })

  test("a missing TUI still delivers the transcript block", async () => {
    const ctx = new FakeCtx("/tmp/project", configPath)
    ctx.tui.showToast = async () => {
      throw new Error("headless")
    }
    await boot(ctx)
    await ctx.eventHandler!(idle("s1"))

    ctx.history.push(msg("3", "assistant", "work"))
    ctx.replies = ["SEVERITY: concern\nNOTE: The lock is held across the await."]
    await ctx.eventHandler!(idle("s1"))

    expect(ctx.delivered).toHaveLength(1)
  })
})

describe("watchdog", () => {
  test("project priorities reach the advisor prompt", async () => {
    const dir = await fs.mkdtemp(path.join(os.tmpdir(), "advisor-wd-"))
    await fs.writeFile(path.join(dir, "WATCHDOG.md"), "Watch writes that bypass the durable queue.")

    const ctx = new FakeCtx(dir, configPath)
    await boot(ctx)
    await ctx.eventHandler!(idle("s1"))

    ctx.history.push(msg("3", "assistant", "wrote directly to the table"))
    ctx.replies = ["SEVERITY: silent"]
    await ctx.eventHandler!(idle("s1"))

    expect(ctx.advisorPrompts[0]).toContain("durable queue")
  })
})

describe("system prompt isolation", () => {
  test("the advisor's own session gets only the advisor prompt", async () => {
    const ctx = new FakeCtx("/tmp/project", configPath)
    await boot(ctx)
    await ctx.eventHandler!(idle("s1"))

    ctx.history.push(msg("3", "assistant", "work"))
    ctx.replies = ["SEVERITY: silent"]

    let captured: unknown[] | undefined
    const original = ctx.session.prompt
    ctx.session.prompt = async (opts: any) => {
      if (String(opts.sessionID).startsWith("advisor-")) {
        const system = [
          { type: "text", text: "You are a Senior Architect. Use CAPS." },
          { type: "text", text: "Strict TDD Mode: enabled" },
        ]
        ctx.contextHook?.({ sessionID: opts.sessionID, system })
        captured = system
      }
      return original(opts)
    }

    await ctx.eventHandler!(idle("s1"))

    expect(captured).toBeDefined()
    expect(captured).toHaveLength(1)
    const text = JSON.stringify(captured)
    expect(text).toContain("peer reviewer")
    expect(text).not.toContain("Senior Architect")
  })

  test("other sessions are left exactly as OpenCode built them", async () => {
    const ctx = new FakeCtx("/tmp/project", configPath)
    await boot(ctx)
    const system = [{ type: "text", text: "the user's real system prompt" }, { type: "text", text: "and its second block" }]
    ctx.contextHook?.({ sessionID: "s1", system })
    expect(system).toHaveLength(2)
    expect((system[0] as { text: string }).text).toBe("the user's real system prompt")
  })
})
