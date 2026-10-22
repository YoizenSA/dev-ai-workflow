import { afterEach, describe, expect, test } from "bun:test"
import * as fs from "node:fs"
import * as os from "node:os"
import * as path from "node:path"
import plugin from "../src/plugin/background-agents"

const homes: string[] = []
const realHome = process.env.HOME
afterEach(() => {
	process.env.HOME = realHome
	for (const dir of homes.splice(0)) fs.rmSync(dir, { recursive: true, force: true })
})

/** Fake v2 ctx that records registrations so the wiring can be driven by hand. */
function fakeCtx() {
	const tools = new Map<string, any>()
	const toolHooks = new Map<string, (e: any) => Promise<void>>()
	const sessionHooks = new Map<string, (e: any) => Promise<void> | void>()
	const interrupts: string[] = []
	const ctx: any = {
		location: { directory: "/tmp/x", project: { id: "proj", directory: "/tmp/x", canonical: "/tmp/x" } },
		session: {
			hook: async (name: string, cb: any) => {
				sessionHooks.set(name, cb)
				return { dispose() {} }
			},
			wait: () => new Promise(() => {}),
			context: async () => [],
			interrupt: async ({ sessionID }: { sessionID: string }) => {
				interrupts.push(sessionID)
			},
			prompt: async () => {},
		},
		tool: {
			transform: async (cb: any) => {
				cb({ add: (def: any) => tools.set(def.name, def) })
				return { dispose() {} }
			},
			hook: async (name: string, cb: any) => {
				toolHooks.set(name, cb)
				return { dispose() {} }
			},
		},
		event: { subscribe: async function* () {} },
	}
	return { ctx, tools, toolHooks, sessionHooks, interrupts }
}

async function setup() {
	const home = fs.mkdtempSync(path.join(os.tmpdir(), "ba-v2-"))
	homes.push(home)
	process.env.HOME = home
	const f = fakeCtx()
	const dispose = await plugin.setup(f.ctx)
	return { ...f, dispose }
}

describe("v2 setup", () => {
	test("registers only the supervision tools and never overrides native subagent", async () => {
		const { tools, dispose } = await setup()
		expect([...tools.keys()].sort()).toEqual(["subagent_read", "subagent_status", "subagent_steer", "subagent_stop"])
		dispose?.()
	})

	test("advertises timeout_minutes on the native subagent schema", async () => {
		const { sessionHooks, dispose } = await setup()
		const event = {
			sessionID: "ses_p",
			system: [],
			tools: { subagent: { description: "native", input: { type: "object", properties: { agent: {} } } } },
		}
		await sessionHooks.get("context")!(event)
		expect(event.tools.subagent.input.properties).toHaveProperty("timeout_minutes")
		dispose?.()
	})

	test("tracks a background child from the native tool result and applies its timeout", async () => {
		const { toolHooks, tools, interrupts, dispose } = await setup()
		const before = { tool: "subagent", sessionID: "ses_p", id: "call_1", input: { agent: "a", prompt: "p", timeout_minutes: 0.0005 } as any }
		await toolHooks.get("execute.before")!(before)
		// The extra field never reaches native input decoding.
		expect(before.input.timeout_minutes).toBeUndefined()

		await toolHooks.get("execute.after")!({
			tool: "subagent",
			sessionID: "ses_p",
			id: "call_1",
			input: { agent: "a", description: "find stuff", prompt: "p", background: true },
			status: "completed",
			result: { content: "bg", metadata: { sessionID: "ses_c", status: "running" } },
		})

		const status = await tools.get("subagent_status").execute({}, { sessionID: "ses_p" })
		expect(status.content).toContain("ses_c")
		for (let i = 0; i < 100 && interrupts.length === 0; i++) await new Promise((r) => setTimeout(r, 5))
		expect(interrupts).toEqual(["ses_c"])
		dispose?.()
	})

	test("foreground (completed) subagent calls are not tracked", async () => {
		const { toolHooks, tools, dispose } = await setup()
		await toolHooks.get("execute.after")!({
			tool: "subagent",
			sessionID: "ses_p",
			id: "call_2",
			input: {},
			status: "completed",
			result: { content: "done", metadata: { sessionID: "ses_fg", status: "completed" } },
		})
		const status = await tools.get("subagent_status").execute({}, { sessionID: "ses_p" })
		expect(status.content).not.toContain("ses_fg")
		dispose?.()
	})
})
