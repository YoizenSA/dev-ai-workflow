import { describe, expect, test } from "bun:test"
import { mapV2EventToV1, usageToMessageUpdated } from "../src/plugin/event-adapter"

const usageProps = {
	sessionID: "ses_1",
	timestamp: 1700000000,
	tokens: { input: 100, output: 20, reasoning: 5, cache: { read: 50, write: 10 } },
}

describe("raw event passthrough", () => {
	test("always returns the raw event first, unmodified", () => {
		const raw = { type: "session.status", data: { sessionID: "ses_1", status: { type: "busy" } } }
		const out = mapV2EventToV1(raw)
		expect(out.length).toBe(1)
		expect(out[0]).toBe(raw)
	})

	test("never mutates the raw event when synthesizing", () => {
		const raw = {
			type: "session.status",
			data: { sessionID: "ses_1", status: { type: "idle" } },
		}
		mapV2EventToV1(raw)
		expect((raw.data as any).status.type).toBe("idle")
		expect(Object.keys(raw)).toEqual(["type", "data"])
	})

	test("unknown event types pass through with no synthesis", () => {
		const raw = { type: "something.new", data: { sessionID: "ses_1" } }
		const out = mapV2EventToV1(raw)
		expect(out.length).toBe(1)
	})
})

describe("session.status idle synthesis", () => {
	test("idle status synthesizes a v1 session.idle event", () => {
		const raw = { type: "session.status", data: { sessionID: "ses_1", status: { type: "idle" } } }
		const out = mapV2EventToV1(raw)
		expect(out.length).toBe(2)
		expect(out[1].type).toBe("session.idle")
		expect((out[1].properties as any).sessionID).toBe("ses_1")
	})

	test("non-idle status synthesizes nothing", () => {
		const raw = { type: "session.status", data: { sessionID: "ses_1", status: { type: "busy" } } }
		const out = mapV2EventToV1(raw)
		expect(out.length).toBe(1)
	})

	test("missing sessionID synthesizes nothing", () => {
		const raw = { type: "session.status", data: { status: { type: "idle" } } }
		const out = mapV2EventToV1(raw)
		expect(out.length).toBe(1)
	})

	test("reads from the v1 properties envelope too", () => {
		const raw = { type: "session.status", properties: { sessionID: "ses_1", status: { type: "idle" } } }
		const out = mapV2EventToV1(raw)
		expect(out.length).toBe(2)
		expect(out[1].type).toBe("session.idle")
	})
})

describe("usage telemetry synthesis", () => {
	test("session.usage.updated synthesizes a v1 message.updated heartbeat", () => {
		const raw = { type: "session.usage.updated", data: usageProps }
		const out = mapV2EventToV1(raw)
		expect(out.length).toBe(2)
		const evt = out[1]
		expect(evt.type).toBe("message.updated")
		const info = (evt.properties as any).info
		expect(info.sessionID).toBe("ses_1")
		expect(info.role).toBe("assistant")
		expect(info.time.completed).toBe(1700000000)
		expect(info.tokens.input).toBe(100)
		expect(info.tokens.cache.read).toBe(50)
	})

	test("session.step.ended synthesizes the same heartbeat", () => {
		const raw = { type: "session.step.ended", data: usageProps }
		const out = mapV2EventToV1(raw)
		expect(out.length).toBe(2)
		expect(out[1].type).toBe("message.updated")
	})

	test("identical token snapshots produce a stable fingerprint", () => {
		const first = usageToMessageUpdated(usageProps)
		const second = usageToMessageUpdated(usageProps)
		expect(first).toBeDefined()
		expect((first as any).properties.info.id).toBe((second as any).properties.info.id)
	})

	test("distinct token snapshots stay distinct", () => {
		const first = usageToMessageUpdated(usageProps)
		const second = usageToMessageUpdated({ ...usageProps, tokens: { ...usageProps.tokens, input: 101 } })
		expect((first as any).properties.info.id).not.toBe((second as any).properties.info.id)
	})

	test("incomplete token blocks are dropped, never half-mapped", () => {
		const props = { sessionID: "ses_1", tokens: { input: 100 } }
		expect(usageToMessageUpdated(props)).toBeUndefined()
	})

	test("missing sessionID drops the heartbeat", () => {
		const props = { tokens: { input: 100, output: 1, cache: { read: 1, write: 1 } } }
		expect(usageToMessageUpdated(props)).toBeUndefined()
	})

	test("missing timestamp falls back to zero completed time", () => {
		const props = { sessionID: "ses_1", tokens: { input: 1, output: 1, cache: { read: 1, write: 1 } } }
		const mapped = usageToMessageUpdated(props)
		expect((mapped as any).properties.info.time.completed).toBe(0)
	})
})
