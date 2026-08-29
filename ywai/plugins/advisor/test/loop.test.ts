import { beforeEach, describe, expect, test } from "bun:test"
import * as fs from "node:fs/promises"
import * as os from "node:os"
import * as path from "node:path"
import AdvisorPlugin from "../src/index"

// End-to-end over a stubbed OpenCode client: event in, note out. The units are
// covered elsewhere; what these pin is the wiring between them — including the
// two ways this feature fails badly in the wild: reviewing its own advice, and
// talking too much.

type Msg = { info: { id: string; role: string }; parts: { type: string; text: string }[] }

const msg = (id: string, role: string, text: string): Msg => ({
  info: { id, role },
  parts: [{ type: "text", text }],
})

class FakeClient {
  history: Msg[] = []
  /** Replies the advisor model will give, in order. */
  replies: string[] = []
  /** Notes delivered into the primary session. */
  delivered: { text: string; noReply: boolean }[] = []
  toasts: { variant: string; message: string }[] = []
  createdSessions: string[] = []
  deletedSessions: string[] = []
  advisorPrompts: string[] = []
  #seq = 0

  session = {
    messages: async () => ({ data: this.history }),
    create: async () => {
      const id = `advisor-${++this.#seq}`
      this.createdSessions.push(id)
      return { data: { id } }
    },
    delete: async ({ path: p }: any) => {
      this.deletedSessions.push(p.id)
      return {}
    },
    prompt: async ({ path: p, body }: any) => {
      const text = body?.parts?.[0]?.text ?? ""
      if (p.id.startsWith("advisor-")) {
        this.advisorPrompts.push(text)
        const reply = this.replies.shift() ?? "SEVERITY: silent"
        return { data: { parts: [{ type: "text", text: reply }] } }
      }
      this.delivered.push({ text, noReply: body?.noReply === true })
      // A delivered note becomes part of the primary transcript, exactly as it
      // would in OpenCode — this is what lets the recursion test be real.
      this.history.push(msg(`advisory-${this.delivered.length}`, "user", text))
      return {}
    },
  }

  tui = {
    showToast: async ({ body }: any) => {
      this.toasts.push({ variant: body.variant, message: body.message })
      return {}
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

/** Boots the plugin against the test config. */
async function boot(client: FakeClient, directory = "/tmp/project") {
  return await AdvisorPlugin({ client, directory } as never, { configPath })
}

const idle = (sessionID: string) => ({ event: { type: "session.idle", properties: { sessionID } } }) as never

describe("the review loop", () => {
  test("first idle only seeds the cursor — no review of pre-existing history", async () => {
    const client = new FakeClient()
    client.history = [msg("1", "user", "old"), msg("2", "assistant", "old reply")]
    client.replies = ["SEVERITY: blocker\nNOTE: should never be asked"]

    const hooks = await boot(client)
    await hooks.event!(idle("s1"))

    expect(client.advisorPrompts).toHaveLength(0)
    expect(client.delivered).toHaveLength(0)
  })

  test("a concern reaches the transcript and the toast", async () => {
    const client = new FakeClient()
    const hooks = await boot(client)
    await hooks.event!(idle("s1")) // seed

    client.history.push(msg("3", "assistant", "deleted the migration and moved on"))
    client.replies = ["SEVERITY: concern\nNOTE: The migration runs before the column exists."]
    await hooks.event!(idle("s1"))

    expect(client.delivered).toHaveLength(1)
    expect(client.delivered[0]!.noReply).toBe(true)
    expect(client.delivered[0]!.text).toContain('severity="concern"')
    expect(client.delivered[0]!.text).toContain("column exists")
    expect(client.toasts[0]!.variant).toBe("warning")
  })

  test("silence delivers nothing", async () => {
    const client = new FakeClient()
    const hooks = await boot(client)
    await hooks.event!(idle("s1"))

    client.history.push(msg("3", "assistant", "renamed a variable"))
    client.replies = ["SEVERITY: silent"]
    await hooks.event!(idle("s1"))

    expect(client.delivered).toHaveLength(0)
    expect(client.toasts).toHaveLength(0)
  })

  test("the advisor never reviews its own note", async () => {
    // The failure this prevents: the note lands in the transcript, the next
    // idle feeds it back, and the advisor comments on its own advice forever.
    const client = new FakeClient()
    const hooks = await boot(client)
    await hooks.event!(idle("s1"))

    client.history.push(msg("3", "assistant", "shipped it"))
    client.replies = ["SEVERITY: concern\nNOTE: Verification is thinner than the risk."]
    await hooks.event!(idle("s1"))

    client.history.push(msg("4", "assistant", "another step"))
    client.replies = ["SEVERITY: silent"]
    await hooks.event!(idle("s1"))

    const secondPrompt = client.advisorPrompts[1] ?? ""
    expect(secondPrompt).toContain("another step")
    expect(secondPrompt).not.toContain("Verification is thinner")
  })

  test("repeated advice lands once", async () => {
    const client = new FakeClient()
    const hooks = await boot(client)
    await hooks.event!(idle("s1"))

    for (let i = 0; i < 3; i++) {
      client.history.push(msg(`w${i}`, "assistant", `work step ${i}`))
      client.replies = ["SEVERITY: blocker\nNOTE: The retry loop has no backoff."]
      await hooks.event!(idle("s1"))
    }

    expect(client.delivered).toHaveLength(1)
  })

  test("content-free notes never reach the transcript", async () => {
    const client = new FakeClient()
    const hooks = await boot(client)
    await hooks.event!(idle("s1"))

    for (const noise of ["SEVERITY: blocker\nNOTE: Stop.", "SEVERITY: blocker\nNOTE: Done", "SEVERITY: concern\nNOTE: LGTM"]) {
      client.history.push(msg(`n${client.delivered.length}${noise.length}`, "assistant", "more work"))
      client.replies = [noise]
      await hooks.event!(idle("s1"))
    }

    expect(client.delivered).toHaveLength(0)
  })

  test("an idle with nothing new does not call the advisor model", async () => {
    const client = new FakeClient()
    const hooks = await boot(client)
    await hooks.event!(idle("s1"))

    await hooks.event!(idle("s1"))
    await hooks.event!(idle("s1"))

    expect(client.advisorPrompts).toHaveLength(0)
  })

  test("the advisor's own child session is cleaned up", async () => {
    const client = new FakeClient()
    const hooks = await boot(client)
    await hooks.event!(idle("s1"))

    client.history.push(msg("3", "assistant", "work"))
    client.replies = ["SEVERITY: nit\nNOTE: A map would be simpler here."]
    await hooks.event!(idle("s1"))

    expect(client.createdSessions).toHaveLength(1)
    expect(client.deletedSessions).toEqual(client.createdSessions)
  })

  test("events from the advisor's own session are ignored", async () => {
    const client = new FakeClient()
    const hooks = await boot(client)
    await hooks.event!(idle("advisor-1"))
    // Seeding an advisor session would make it a reviewed session of its own.
    expect(client.advisorPrompts).toHaveLength(0)
  })
})

describe("failure containment", () => {
  test("an advisor model error never breaks the watched session", async () => {
    const client = new FakeClient()
    client.session.create = async () => {
      throw new Error("provider is down")
    }
    const hooks = await boot(client)
    await hooks.event!(idle("s1"))
    client.history.push(msg("3", "assistant", "work"))

    await expect(hooks.event!(idle("s1"))).resolves.toBeUndefined()
    expect(client.delivered).toHaveLength(0)
  })

  test("a missing TUI still delivers the transcript block", async () => {
    const client = new FakeClient()
    client.tui.showToast = async () => {
      throw new Error("headless")
    }
    const hooks = await boot(client)
    await hooks.event!(idle("s1"))

    client.history.push(msg("3", "assistant", "work"))
    client.replies = ["SEVERITY: concern\nNOTE: The lock is held across the await."]
    await hooks.event!(idle("s1"))

    expect(client.delivered).toHaveLength(1)
  })
})

describe("watchdog", () => {
  test("project priorities reach the advisor prompt", async () => {
    const dir = await fs.mkdtemp(path.join(os.tmpdir(), "advisor-wd-"))
    await fs.writeFile(path.join(dir, "WATCHDOG.md"), "Watch writes that bypass the durable queue.")

    const client = new FakeClient()
    const hooks = await boot(client, dir)
    await hooks.event!(idle("s1"))

    client.history.push(msg("3", "assistant", "wrote directly to the table"))
    client.replies = ["SEVERITY: silent"]
    await hooks.event!(idle("s1"))

    expect(client.advisorPrompts[0]).toContain("durable queue")
  })
})

describe("system prompt isolation", () => {
  test("the advisor's own session gets only the advisor prompt", async () => {
    // `system` on session.prompt appends rather than replaces, so without this
    // the review inherits the user's whole AGENTS.md persona, SDD rules and MCP
    // instructions — thousands of tokens per review, aimed at a different job.
    const client = new FakeClient()
    const hooks = await boot(client)
    await hooks.event!(idle("s1"))

    client.history.push(msg("3", "assistant", "work"))
    client.replies = ["SEVERITY: silent"]

    let captured: string[] | undefined
    const original = client.session.prompt
    client.session.prompt = async (opts: any) => {
      if (opts.path.id.startsWith("advisor-")) {
        const system = ["You are a Senior Architect. Use CAPS.", "Strict TDD Mode: enabled"]
        await (hooks as any)["experimental.chat.system.transform"](
          { sessionID: opts.path.id },
          { system },
        )
        captured = system
      }
      return original(opts)
    }

    await hooks.event!(idle("s1"))

    expect(captured).toBeDefined()
    expect(captured).toHaveLength(1)
    expect(captured![0]).toContain("peer reviewer")
    expect(captured!.join(" ")).not.toContain("Senior Architect")
  })

  test("other sessions are left exactly as OpenCode built them", async () => {
    const client = new FakeClient()
    const hooks = await boot(client)
    const system = ["the user's real system prompt", "and its second block"]
    await (hooks as any)["experimental.chat.system.transform"]({ sessionID: "s1" }, { system })
    expect(system).toHaveLength(2)
    expect(system[0]).toBe("the user's real system prompt")
  })
})
