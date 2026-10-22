import { afterEach, describe, expect, test } from "bun:test"
import * as fs from "node:fs"
import * as os from "node:os"
import * as path from "node:path"
import { Supervisor, digest, type SupervisorSession } from "../src/plugin/supervisor"
import type { DelegationTerminalEventData } from "../src/plugin/terminal-events"

const PARENT = "ses_parent"
const CHILD = "ses_child"

function assistant(text: string, extra: Record<string, unknown> = {}) {
	return {
		type: "assistant",
		content: [{ type: "text", text }],
		time: { created: 1, completed: 2 },
		...extra,
	}
}

/** Fake v2 session domain: wait() blocks until release(); context() returns `messages`. */
function fakeSession() {
	let release: () => void = () => {}
	const calls = { interrupt: [] as string[], prompt: [] as any[] }
	const state = {
		messages: [{ type: "user", text: "do it" }] as any[],
		release: () => release(),
	}
	const session: SupervisorSession = {
		wait: () => new Promise<void>((resolve) => (release = resolve)),
		context: async () => state.messages,
		interrupt: async ({ sessionID }) => {
			calls.interrupt.push(sessionID)
			state.release()
		},
		prompt: async (input) => {
			calls.prompt.push(input)
		},
	}
	return { session, state, calls }
}

const dirs: string[] = []
afterEach(() => {
	for (const dir of dirs.splice(0)) fs.rmSync(dir, { recursive: true, force: true })
})
function indexFile() {
	const dir = fs.mkdtempSync(path.join(os.tmpdir(), "ba-sup-"))
	dirs.push(dir)
	return path.join(dir, "subagents.json")
}

async function until(predicate: () => boolean) {
	for (let i = 0; i < 200 && !predicate(); i++) await new Promise((r) => setTimeout(r, 5))
	expect(predicate()).toBe(true)
}

function make(opts: { defaultTimeoutMs?: number; file?: string } = {}) {
	const fake = fakeSession()
	const events: DelegationTerminalEventData[] = []
	const sup = new Supervisor({
		session: fake.session,
		indexFile: opts.file ?? indexFile(),
		defaultTimeoutMs: opts.defaultTimeoutMs ?? 0,
		settleDelayMs: 1,
		onTerminal: (e) => events.push(e),
	})
	return { sup, events, ...fake }
}

describe("Supervisor", () => {
	test("tracks a native child and marks it complete when the session settles", async () => {
		const { sup, state, events } = make()
		sup.track({ sessionID: CHILD, parentSessionID: PARENT, agent: "core/finder", description: "find" })
		expect(await sup.status(PARENT)).toContain(CHILD)
		expect(await sup.status(PARENT)).toContain("running")

		state.messages.push(assistant("RESULT"))
		state.release()
		await until(() => events.length === 1)

		expect(events[0]).toMatchObject({ kind: "terminal", status: "complete", sessionID: CHILD, remaining: 0 })
		expect(await sup.read(PARENT, CHILD)).toContain("RESULT")
		sup.dispose()
	})

	test("an assistant error settles as error", async () => {
		const { sup, state, events } = make()
		sup.track({ sessionID: CHILD, parentSessionID: PARENT, agent: "a", description: "d" })
		state.messages.push(assistant("", { error: { type: "api", message: "bad key" } }))
		state.release()
		await until(() => events.length === 1)
		expect(events[0]).toMatchObject({ status: "error", error: "bad key" })
		sup.dispose()
	})

	test("stop interrupts the child and returns its partial output", async () => {
		const { sup, state, calls } = make()
		sup.track({ sessionID: CHILD, parentSessionID: PARENT, agent: "a", description: "d" })
		state.messages.push({ type: "assistant", content: [{ type: "text", text: "halfway" }], time: { created: 1 } })

		const out = await sup.stop(PARENT, CHILD)
		expect(calls.interrupt).toEqual([CHILD])
		expect(out).toContain("halfway")
		expect(await sup.status(PARENT)).toContain("stopped")
		sup.dispose()
	})

	test("watchdog interrupts a child that outlives its timeout", async () => {
		const { sup, calls, events } = make()
		sup.track({ sessionID: CHILD, parentSessionID: PARENT, agent: "a", description: "d", timeoutMs: 20 })
		await until(() => calls.interrupt.length === 1)
		await until(() => events.length === 1)
		expect(events[0].status).toBe("timeout")
		sup.dispose()
	})

	test("default timeout applies when no override is given; 0 means none", async () => {
		const a = make({ defaultTimeoutMs: 20 })
		a.sup.track({ sessionID: CHILD, parentSessionID: PARENT, agent: "a", description: "d" })
		await until(() => a.calls.interrupt.length === 1)
		a.sup.dispose()

		const b = make({ defaultTimeoutMs: 20 })
		b.sup.track({ sessionID: CHILD, parentSessionID: PARENT, agent: "a", description: "d", timeoutMs: 0 })
		await new Promise((r) => setTimeout(r, 60))
		expect(b.calls.interrupt).toEqual([])
		b.sup.dispose()
	})

	test("steer goes through native prompt delivery:steer", async () => {
		const { sup, calls } = make()
		sup.track({ sessionID: CHILD, parentSessionID: PARENT, agent: "a", description: "d" })
		expect(await sup.steer(PARENT, CHILD, "focus on X")).toContain("Steer sent")
		expect(calls.prompt).toEqual([{ sessionID: CHILD, text: "[SUPERVISOR STEER] focus on X", delivery: "steer" }])
		sup.dispose()
	})

	test("children are scoped to their parent session", async () => {
		const { sup } = make()
		sup.track({ sessionID: CHILD, parentSessionID: PARENT, agent: "a", description: "d" })
		expect(await sup.read("ses_other", CHILD)).toContain("not found")
		expect(await sup.stop("ses_other", CHILD)).toContain("not found")
		expect(await sup.status("ses_other")).not.toContain(CHILD)
		sup.dispose()
	})

	test("index persists across instances so list/read survive a restart", async () => {
		const file = indexFile()
		const first = make({ file })
		first.sup.track({ sessionID: CHILD, parentSessionID: PARENT, agent: "a", description: "persisted" })
		first.sup.dispose()

		const second = make({ file })
		second.sup.restore()
		second.state.messages.push(assistant("AFTER RESTART"))
		expect(await second.sup.status(PARENT)).toContain("persisted")
		expect(await second.sup.read(PARENT, CHILD)).toContain("AFTER RESTART")
		second.sup.dispose()
	})
})

describe("digest", () => {
	test("summarises assistant text, tool activity and steers", () => {
		const out = digest([
			{ type: "user", text: "[SUPERVISOR STEER] narrow it" },
			{
				type: "assistant",
				content: [
					{ type: "text", text: "looking" },
					{ type: "tool", name: "grep", state: { status: "completed", title: "foo" } },
					{ type: "tool", name: "read", state: { status: "error", error: { message: "nope" } } },
				],
			},
		])
		expect(out).toContain(">> [SUPERVISOR STEER] narrow it")
		expect(out).toContain("looking")
		expect(out).toContain("[tool] grep (completed): foo")
		expect(out).toContain("[tool] read (error) — nope")
	})
})
