import { describe, expect, test } from "bun:test"
import { DelegationManager } from "../src/manager"
import type { SessionIO } from "../src/adapter"

/**
 * Fake SessionIO that records calls and simulates a child session run.
 * onPrompt lets a test replay events in the same position the runtime would
 * emit them: only after delegate() has registered the record (the real
 * server never emits child events before prompt() returns control).
 */
class FakeIO implements SessionIO {
	created: { agent?: string; model?: unknown; title?: string }[] = []
	prompts: { sessionID: string; text: string; delivery?: string }[] = []
	synthetics: { sessionID: string; text: string }[] = []
	interrupts: string[] = []
	nextSessionID = 1
	onPrompt?: (sessionID: string) => void

	async create(input: { agent?: string; model?: unknown; title?: string }): Promise<string> {
		this.created.push(input)
		return `ses_child_${this.nextSessionID++}`
	}
	async prompt(sessionID: string, text: string, delivery?: "steer" | "queue"): Promise<void> {
		this.prompts.push({ sessionID, text, delivery })
		if (!delivery && this.onPrompt) this.onPrompt(sessionID)
	}
	async synthetic(sessionID: string, text: string): Promise<void> {
		this.synthetics.push({ sessionID, text })
	}
	async interrupt(sessionID: string): Promise<void> {
		this.interrupts.push(sessionID)
	}
}

function textDelta(sessionID: string, messageID: string, delta: string) {
	return { type: "session.next.text.delta", data: { sessionID, assistantMessageID: messageID, delta } }
}
function stepEnded(sessionID: string, finish = "stop") {
	return { type: "session.next.step.ended", data: { sessionID, finish } }
}
function idle(sessionID: string) {
	return { type: "session.idle", data: { sessionID } }
}

async function newManager(io: FakeIO) {
	const manager = new DelegationManager(
		io,
		`${import.meta.dir}/tmp-records-${Date.now()}-${Math.random().toString(36).slice(2)}`,
		() => {},
		{ stepQuietMs: 60 }, // fast quiet period for tests
	)
	await manager.start()
	return manager
}

describe("DelegationManager (v2)", () => {
	test("sync delegation returns the result inline via step completion + quiet", async () => {
		const io = new FakeIO()
		const manager = await newManager(io)
		io.onPrompt = (sessionID) => {
			// Replay the child turn as the runtime would, after registration.
			manager.handleEvent(textDelta(sessionID, "msg1", "PON"))
			manager.handleEvent(textDelta(sessionID, "msg1", "G"))
			manager.handleEvent(stepEnded(sessionID))
		}

		const pending = manager.delegate({
			parentSessionID: "ses_parent",
			prompt: "say PONG",
			agent: "dev",
			mode: "sync",
			maxRunTimeMs: 5_000,
		})

		const child = io.created[0]
		expect(child?.agent).toBe("dev")

		const outcome = await pending
		expect(outcome.mode).toBe("sync")
		if (outcome.mode === "sync") {
			expect(outcome.status).toBe("completed")
			expect(outcome.result).toBe("PONG")
		}
		// sync delegations notify nobody — the caller already has the result.
		expect(io.synthetics).toHaveLength(0)
		await manager.stop()
	})

	test("async delegation notifies the parent with a synthetic message", async () => {
		const io = new FakeIO()
		const manager = await newManager(io)

		const outcome = await manager.delegate({
			parentSessionID: "ses_parent",
			prompt: "research",
			agent: "finder",
			mode: "async",
		})
		if (outcome.mode !== "async") throw new Error("expected async")

		manager.handleEvent(textDelta(outcome.childSessionID, "msg1", "found it"))
		manager.handleEvent(stepEnded(outcome.childSessionID))

		// Completion fires after the quiet period (60ms) + notification debounce (750ms).
		await new Promise((r) => setTimeout(r, 900))

		expect(io.synthetics.length).toBe(1)
		expect(io.synthetics[0].sessionID).toBe("ses_parent")
		expect(io.synthetics[0].text).toContain("<task-notification>")
		expect(io.synthetics[0].text).toContain(outcome.id)
		await manager.stop()
	})

	test("model + effort (variant) are passed to session.create", async () => {
		const io = new FakeIO()
		const manager = await newManager(io)
		await manager.delegate({
			parentSessionID: "ses_parent",
			prompt: "x",
			agent: "dev",
			mode: "async",
			model: { providerID: "anthropic", id: "claude-haiku-4-5", variant: "low" },
		})
		expect(io.created[0].model).toEqual({
			id: "claude-haiku-4-5",
			providerID: "anthropic",
			variant: "low",
		})
		await manager.stop()
	})

	test("steer uses native delivery and restarts the deadline window", async () => {
		const io = new FakeIO()
		const manager = await newManager(io)
		const outcome = await manager.delegate({
			parentSessionID: "ses_parent",
			prompt: "x",
			agent: "dev",
			mode: "async",
			maxRunTimeMs: 60_000,
		})
		if (outcome.mode !== "async") throw new Error("expected async")

		const before = Date.now()
		await new Promise((r) => setTimeout(r, 5))
		await manager.steer("ses_parent", outcome.id, "also check tests")

		expect(io.prompts.some((p) => p.delivery === "steer" && p.text === "also check tests")).toBe(true)
		const record = (manager as any).records.get(outcome.id)
		expect(record.deadlineAt).toBeGreaterThan(before)
		await manager.stop()
	})

	test("stop interrupts the child and finalizes as stopped", async () => {
		const io = new FakeIO()
		const manager = await newManager(io)
		const outcome = await manager.delegate({
			parentSessionID: "ses_parent",
			prompt: "x",
			agent: "dev",
			mode: "async",
		})
		if (outcome.mode !== "async") throw new Error("expected async")

		manager.handleEvent(textDelta(outcome.childSessionID, "msg1", "partial"))
		const message = await manager.stopDelegation("ses_parent", outcome.id)

		expect(message).toContain("Stopped")
		expect(io.interrupts).toContain(outcome.childSessionID)
		const read = await manager.read("ses_parent", outcome.id)
		expect(read).toContain("partial")
		await manager.stop()
	})

	test("timeout expires the delegation and interrupts the child", async () => {
		const io = new FakeIO()
		const manager = await newManager(io)
		const outcome = await manager.delegate({
			parentSessionID: "ses_parent",
			prompt: "x",
			agent: "dev",
			mode: "sync",
			maxRunTimeMs: 30, // expire almost immediately
		})
		expect(outcome.mode).toBe("sync")
		if (outcome.mode === "sync") {
			expect(outcome.status).toBe("timeout")
		}
		expect(io.interrupts.length).toBe(1)
		await manager.stop()
	})

	test("unknown delegation ids produce guidance, not throws", async () => {
		const io = new FakeIO()
		const manager = await newManager(io)
		expect(await manager.steer("ses_parent", "nope", "x")).toContain("❌")
		expect(await manager.stopDelegation("ses_parent", "nope")).toContain("❌")
		expect(await manager.read("ses_parent", "nope")).toContain("❌")
		await manager.stop()
	})
})
