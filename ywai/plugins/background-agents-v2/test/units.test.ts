import { describe, expect, test } from "bun:test"
import { effortToVariant, formatModelRef, parseModelString } from "../src/adapter"
import {
	deserializeDelegation,
	serializeDelegation,
	type DelegationRecord,
} from "../src/state"
import { Transcript } from "../src/transcript"
import { generateReadableId, generateUniqueId } from "../src/id"

describe("parseModelString", () => {
	test("parses provider/model", () => {
		expect(parseModelString("anthropic/claude-haiku-4-5")).toEqual({
			providerID: "anthropic",
			id: "claude-haiku-4-5",
		})
	})
	test("parses provider/model with variant (effort)", () => {
		expect(parseModelString("anthropic/claude-sonnet-4-5#high")).toEqual({
			providerID: "anthropic",
			id: "claude-sonnet-4-5",
			variant: "high",
		})
	})
	test("model ids with slashes survive", () => {
		expect(parseModelString("openai/org/model-x")).toEqual({
			providerID: "openai",
			id: "org/model-x",
		})
	})
	test("garbage returns undefined", () => {
		expect(parseModelString("nomodel")).toBeUndefined()
		expect(parseModelString("")).toBeUndefined()
	})
	test("formatModelRef round-trips", () => {
		const ref = parseModelString("anthropic/claude-sonnet-4-5#high")
		expect(formatModelRef(ref!)).toBe("anthropic/claude-sonnet-4-5#high")
	})
})

describe("effortToVariant", () => {
	test("aliases normalize", () => {
		expect(effortToVariant("max")).toBe("high")
		expect(effortToVariant("min")).toBe("low")
		expect(effortToVariant("minimal")).toBe("low")
	})
	test("known efforts pass through", () => {
		expect(effortToVariant("high")).toBe("high")
		expect(effortToVariant("Medium")).toBe("medium")
	})
})

describe("state", () => {
	const record: DelegationRecord = {
		id: "swift-otter-tiger",
		parentSessionID: "ses_parent",
		childSessionID: "ses_child",
		agent: "dev",
		mode: "async",
		prompt: "do the thing",
		status: "running",
		createdAt: 1,
		startedAt: 1,
		lastActivityAt: 1,
		deadlineAt: 0,
		steerCount: 0,
		toolCalls: 0,
		notified: false,
	}

	test("serialize/deserialize round-trips", () => {
		const restored = deserializeDelegation(serializeDelegation(record))
		expect(restored).toEqual(record)
	})
	test("corrupt json yields undefined, not a throw", () => {
		expect(deserializeDelegation("{not json")).toBeUndefined()
		expect(deserializeDelegation('{"version":2}')).toBeUndefined()
	})
})

describe("Transcript", () => {
	test("accumulates deltas per message and picks the latest as result", () => {
		const t = new Transcript()
		t.appendText("msg1", "first answer")
		t.appendText("msg2", "PO")
		t.appendText("msg2", "NG")
		expect(t.resultText()).toBe("PONG")
		expect(t.fullText()).toBe("first answer\n\nPONG")
	})
	test("tool events feed the digest and counter", () => {
		const t = new Transcript()
		t.recordTool("read", "called")
		t.recordTool("read", "success")
		t.recordTool("bash", "called")
		t.recordTool("bash", "failed")
		const digest = t.peekDigest()
		expect(digest).toContain("`read`")
		expect(digest).toContain("✗ `bash`")
		expect(t.toolCallCount()).toBe(2)
	})
})

describe("ids", () => {
	test("readable id shape", () => {
		expect(generateReadableId()).toMatch(/^[a-z]+-[a-z]+-[a-z]+$/)
	})
	test("unique id avoids taken candidates", () => {
		const taken = new Set<string>()
		let first = ""
		for (let i = 0; i < 5; i++) {
			const id = generateUniqueId((c) => taken.has(c))
			expect(taken.has(id)).toBe(false)
			taken.add(id)
			if (i === 0) first = id
		}
		expect(taken.size).toBe(5)
		expect(first).toBeTruthy()
	})
})
