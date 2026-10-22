import { describe, expect, test } from "bun:test"
import { applyPlan, planFrom, toLib, type V2Message } from "../src/adapter"

class Msg {
	constructor(fields: V2Message) {
		Object.assign(this, fields)
	}
}
const m = (f: V2Message) => new Msg(f) as unknown as V2Message

function conversation(): V2Message[] {
	return [
		m({ role: "system", content: [{ type: "text", text: "sys" }] }),
		m({ role: "user", content: [{ type: "text", text: "fix it" }] }),
		m({
			role: "assistant",
			content: [
				{ type: "reasoning", text: "think" },
				{ type: "text", text: "reading" },
				{ type: "tool-call", id: "a", name: "read", input: { path: "x" } },
				{ type: "tool-call", id: "b", name: "grep", input: { q: "y" } },
			],
		}),
		m({
			role: "tool",
			content: [
				{ type: "tool-result", id: "a", name: "read", result: { type: "text", value: "AAAA" } },
			],
		}),
		m({
			role: "tool",
			content: [
				{ type: "tool-result", id: "b", name: "grep", result: { type: "error", value: { msg: "no" } } },
			],
		}),
	]
}

describe("toLib", () => {
	test("skips system, maps tool role to user, stringifies results", () => {
		const lib = toLib(conversation())
		expect(lib.map((x) => x.role)).toEqual(["user", "assistant", "user", "user"])
		expect(lib[1]).toMatchObject({
			text: "reading",
			toolUses: [
				{ tool_use_id: "a", tool: "read", input: { path: "x" } },
				{ tool_use_id: "b", tool: "grep", input: { q: "y" } },
			],
		})
		expect(lib[2].toolResults).toEqual([{ tool_use_id: "a", text: "AAAA", isError: false }])
		expect(lib[3].toolResults).toEqual([{ tool_use_id: "b", text: '{"msg":"no"}', isError: true }])
	})
})

describe("planFrom + applyPlan", () => {
	test("drops a call with its result, truncates another, leaves the rest", () => {
		const before = toLib(conversation())
		const after = [
			before[0],
			{ ...before[1], toolUses: [before[1].toolUses[0]] },
			{ role: "user" as const, text: "", toolUses: [], toolResults: [{ tool_use_id: "a", text: "AA [truncated]" }] },
		]
		const plan = planFrom(before, after)
		expect(plan).toEqual(new Map([["b", { action: "drop" }], ["a", { action: "truncate", text: "AA [truncated]" }]]))

		const input = conversation()
		const out = applyPlan(input, plan)
		expect(out).toHaveLength(4) // tool message holding only b's result is gone
		expect(out[0]).toBe(input[0])
		expect(out[1]).toBe(input[1])
		expect(out[2]).toBeInstanceOf(Msg)
		expect(out[2].content.map((p) => p.type)).toEqual(["reasoning", "text", "tool-call"])
		expect(out[3].content[0].result).toEqual({ type: "text", value: "AA [truncated]" })
		expect(input[3].content[0].result.value).toBe("AAAA") // originals not mutated
	})

	test("empty plan returns the same array contents", () => {
		const input = conversation()
		const out = applyPlan(input, new Map())
		out.forEach((msg, i) => expect(msg).toBe(input[i]))
	})
})
