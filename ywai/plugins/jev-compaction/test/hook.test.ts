import { expect, test } from "bun:test"
import { compactEvent, requestCompaction, threshold } from "../src/index"
import type { V2Message } from "../src/adapter"

process.env.JEV_COMPACTION_THRESHOLD_TOKENS = "10"

function history(): V2Message[] {
	const msgs: V2Message[] = [{ role: "user", content: [{ type: "text", text: "go" }] }]
	for (const id of ["a", "b"]) {
		msgs.push({ role: "assistant", content: [{ type: "tool-call", id, name: "read", input: {} }] })
		msgs.push({ role: "tool", content: [{ type: "tool-result", id, name: "read", result: { type: "text", value: "x".repeat(200) } }] })
	}
	for (let i = 0; i < 6; i++) msgs.push({ role: "user", content: [{ type: "text", text: `t${i}` }] })
	return msgs
}

test("asks Jev once, then reapplies the cached plan", async () => {
	let calls = 0
	const fake: any = async (lib: any[]) => {
		calls++
		return { messages: lib.map((m) => ({ ...m, toolUses: m.toolUses.filter((t: any) => t.tool_use_id !== "a"), toolResults: m.toolResults?.filter((r: any) => r.tool_use_id !== "a") })) }
	}
	for (let i = 0; i < 2; i++) {
		const event = { sessionID: "s1", messages: history() }
		await compactEvent(event, "key", fake)
		expect(JSON.stringify(event.messages)).not.toContain('"id":"a"')
		expect(JSON.stringify(event.messages)).toContain('"id":"b"')
	}
	expect(calls).toBe(1)
})

test("fails open when Jev throws or there is no key", async () => {
	const boom: any = async () => {
		throw new Error("down")
	}
	const event = { sessionID: "s2", messages: history() }
	await compactEvent(event, "key", boom)
	expect(event.messages).toEqual(history())
	const noKey = { sessionID: "s3", messages: history() }
	await compactEvent(noKey, undefined, boom)
	expect(noKey.messages).toEqual(history())
})

test("skips Jev while the compacted view stays under the threshold", async () => {
	process.env.JEV_COMPACTION_THRESHOLD_TOKENS = "150"
	let calls = 0
	const dropAll: any = async (lib: any[]) => {
		calls++
		return { messages: lib.map((m) => ({ ...m, toolUses: [], toolResults: [] })) }
	}
	const first = { sessionID: "s4", messages: history() }
	await compactEvent(first, "key", dropAll)
	expect(calls).toBe(1)
	// A new tool call arrives, but the compacted history is small again.
	const next = history()
	next.splice(5, 0, { role: "assistant", content: [{ type: "tool-call", id: "c", name: "read", input: {} }] })
	await compactEvent({ sessionID: "s4", messages: next }, "key", dropAll)
	expect(calls).toBe(1)
	process.env.JEV_COMPACTION_THRESHOLD_TOKENS = "10"
})

test("requestCompaction forces Jev on the next request even under the threshold", async () => {
	process.env.JEV_COMPACTION_THRESHOLD_TOKENS = "1000000"
	let calls = 0
	const fake: any = async (lib: any[]) => {
		calls++
		return { messages: lib }
	}
	await compactEvent({ sessionID: "s5", messages: history() }, "key", fake)
	expect(calls).toBe(0)
	requestCompaction("s5")
	await compactEvent({ sessionID: "s5", messages: history() }, "key", fake)
	expect(calls).toBe(1)
	// One-shot: the next request is back to the threshold rule.
	await compactEvent({ sessionID: "s5", messages: history() }, "key", fake)
	expect(calls).toBe(1)
	process.env.JEV_COMPACTION_THRESHOLD_TOKENS = "10"
})

test("threshold defaults to 200k and honours the env override", () => {
	const saved = process.env.JEV_COMPACTION_THRESHOLD_TOKENS
	delete process.env.JEV_COMPACTION_THRESHOLD_TOKENS
	const saveHome = process.env.HOME
	process.env.HOME = "/nonexistent-jev-compaction-test"
	expect(threshold()).toBe(200_000)
	process.env.HOME = saveHome
	process.env.JEV_COMPACTION_THRESHOLD_TOKENS = "5000"
	expect(threshold()).toBe(5000)
	process.env.JEV_COMPACTION_THRESHOLD_TOKENS = saved
})
