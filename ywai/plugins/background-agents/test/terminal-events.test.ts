import { afterEach, describe, expect, test } from "bun:test"
import * as fs from "node:fs/promises"
import * as os from "node:os"
import * as path from "node:path"
import { DelegationManager } from "../src/plugin/delegation-manager"
import type { Logger } from "../src/plugin/logger"
import type { OpencodeClient } from "../src/plugin/primitives/types"
import {
	BackgroundAgentsTerminalRpc,
	TERMINAL_EVENT_NAME,
	TERMINAL_RPC_ID,
	tryRegisterTerminalRpc,
	type BackgroundAgentsTerminalEvent,
} from "../src/plugin/terminal-events"

const noopLogger: Logger = {
	debug: () => Promise.resolve(),
	info: () => Promise.resolve(),
	warn: () => Promise.resolve(),
	error: () => Promise.resolve(),
}

async function waitFor(predicate: () => boolean, timeoutMs = 3_000): Promise<void> {
	const start = Date.now()
	for (;;) {
		if (predicate()) return
		if (Date.now() - start > timeoutMs) throw new Error("waitFor timed out")
		await new Promise((resolve) => setTimeout(resolve, 5))
	}
}

const cleanups: Array<() => Promise<void>> = []
afterEach(async () => {
	while (cleanups.length > 0) {
		await cleanups.pop()?.()
	}
})

/** Minimal fake client: just enough surface for delegate → complete → notifyParent. */
function createFakeClient() {
	const state = {
		sessionCounter: 0,
		promptResolvers: new Map<string, { resolve: (value: unknown) => void; reject: (e: Error) => void }>(),
		promptAsyncCalls: [] as Array<{ sessionID: string; body: unknown }>,
	}
	const client = {
		app: {
			agents: async () => ({ data: [{ name: "researcher", description: "research", mode: "subagent" }] }),
			log: async () => ({}),
		},
		config: {
			get: async () => ({
				data: { agent: { researcher: { permission: { edit: "deny", write: "deny", bash: "deny" } } } },
			}),
		},
		tui: { showToast: async () => ({}) },
		session: {
			get: async (input: { path: { id: string } }) => ({ data: { id: input.path.id } }),
			create: async () => {
				state.sessionCounter += 1
				return { data: { id: `ses_child_${state.sessionCounter}` } }
			},
			status: async () => ({ data: {} }),
			prompt: (input: { path: { id: string } }) =>
				new Promise((resolve, reject) => {
					state.promptResolvers.set(input.path.id, { resolve, reject })
				}),
			promptAsync: async (input: { path: { id: string }; body: unknown }) => {
				state.promptAsyncCalls.push({ sessionID: input.path.id, body: input.body })
				return {}
			},
			abort: async () => ({}),
			delete: async () => ({}),
			messages: async () => ({
				data: [{ info: { role: "assistant" }, parts: [{ type: "text", text: "FINAL RESULT" }] }],
			}),
		},
	}
	return { client: client as unknown as OpencodeClient, state }
}

async function setup(events: BackgroundAgentsTerminalEvent[]) {
	const dir = await fs.mkdtemp(path.join(os.tmpdir(), "bg-agents-terminal-"))
	const { client, state } = createFakeClient()
	let idCounter = 0
	const manager = new DelegationManager(client, dir, noopLogger, {
		completeDebounceMs: 10,
		readPollIntervalMs: 10,
		terminalWaitGraceMs: 20,
		allCompleteQuietPeriodMs: 5,
		readWaitUnlimitedMs: 150,
		idGenerator: () => `task-${++idCounter}`,
		metadataGenerator: async () => ({ title: "Stub title", description: "Stub description" }),
		terminalEventSink: (event) => {
			events.push(event)
		},
	})
	cleanups.push(async () => {
		manager.dispose()
		await fs.rm(dir, { recursive: true, force: true }).catch(() => {})
	})
	return { manager, state }
}

describe("terminal-event RPC contract", () => {
	test("definition is a portable {id, methods, events} literal with an object-schema event", () => {
		expect(BackgroundAgentsTerminalRpc.id).toBe("ywai-background-agents")
		expect(TERMINAL_RPC_ID).toBe("ywai-background-agents")
		expect(TERMINAL_EVENT_NAME).toBe("delegation_terminal")
		const eventDef = (BackgroundAgentsTerminalRpc.events as Record<string, { schema: Record<string, unknown> }>)[
			TERMINAL_EVENT_NAME
		]
		expect(eventDef).toBeDefined()
		expect(eventDef.schema.type).toBe("object")
		expect(eventDef.schema.required).toContain("kind")
		expect(eventDef.schema.required).toContain("parentSessionID")
		expect(eventDef.schema.additionalProperties).toBe(false)
	})

	test("tryRegisterTerminalRpc degrades to undefined without a host rpc surface", async () => {
		await expect(tryRegisterTerminalRpc(undefined)).resolves.toBeUndefined()
		await expect(tryRegisterTerminalRpc({})).resolves.toBeUndefined()
		await expect(tryRegisterTerminalRpc({ rpc: {} })).resolves.toBeUndefined()
	})

	test("tryRegisterTerminalRpc registers and returns the host registration", async () => {
		const emitted: Array<{ name: string; data: unknown }> = []
		const registration = {
			events: {
				emit: async (name: string, data: unknown) => {
					emitted.push({ name, data })
				},
			},
		}
		let seenDef: unknown
		let seenHandlers: unknown
		const ctx = {
			rpc: {
				register: async (def: unknown, handlers: unknown) => {
					seenDef = def
					seenHandlers = handlers
					return registration
				},
			},
		}
		await expect(tryRegisterTerminalRpc(ctx)).resolves.toBe(registration)
		expect(seenDef).toBe(BackgroundAgentsTerminalRpc)
		expect(seenHandlers).toEqual({})
	})
})

describe("manager terminal events (human channel)", () => {
	test("a completed delegation emits one terminal event with its identity", async () => {
		const events: BackgroundAgentsTerminalEvent[] = []
		const { manager, state } = await setup(events)
		const record = await manager.delegate({
			parentSessionID: "ses_parent",
			parentMessageID: "msg_parent",
			parentAgent: "build",
			prompt: "Research the topic",
			agent: "researcher",
		})

		state.promptResolvers.get(record.sessionID)?.resolve({})
		await waitFor(() => record.status === "complete")
		await waitFor(() => events.some((e) => e.kind === "terminal"))

		const terminal = events.filter((e) => e.kind === "terminal")
		expect(terminal).toHaveLength(1)
		const evt = terminal[0]
		expect(evt.kind).toBe("terminal")
		if (evt.kind !== "terminal") throw new Error("narrow")
		expect(evt.delegationID).toBe(record.id)
		expect(evt.agent).toBe("researcher")
		expect(evt.status).toBe("complete")
		expect(evt.sessionID).toBe(record.sessionID)
		expect(evt.parentSessionID).toBe("ses_parent")
		expect(evt.remaining).toBe(0)
		expect(evt.title).toBe("Stub title")
		// The model-facing channel still fired alongside.
		expect(state.promptAsyncCalls.length).toBeGreaterThan(0)
	})

	test("the batch close emits an all-complete event after the terminal one", async () => {
		const events: BackgroundAgentsTerminalEvent[] = []
		const { manager, state } = await setup(events)
		const record = await manager.delegate({
			parentSessionID: "ses_parent",
			parentMessageID: "msg_parent",
			parentAgent: "build",
			prompt: "Research the topic",
			agent: "researcher",
		})

		state.promptResolvers.get(record.sessionID)?.resolve({})
		await waitFor(() => events.some((e) => e.kind === "all-complete"))

		const kinds = events.map((e) => e.kind)
		expect(kinds[0]).toBe("terminal")
		const done = events.find((e) => e.kind === "all-complete")
		expect(done).toMatchObject({ kind: "all-complete", parentSessionID: "ses_parent" })
	})

	test("no sink configured means no events and no interference", async () => {
		const dir = await fs.mkdtemp(path.join(os.tmpdir(), "bg-agents-terminal-nosink-"))
		const { client, state } = createFakeClient()
		const manager = new DelegationManager(client, dir, noopLogger, {
			completeDebounceMs: 10,
			readPollIntervalMs: 10,
			terminalWaitGraceMs: 20,
			allCompleteQuietPeriodMs: 5,
			readWaitUnlimitedMs: 150,
			idGenerator: () => "task-1",
			metadataGenerator: async () => ({ title: "t", description: "d" }),
		})
		cleanups.push(async () => {
			manager.dispose()
			await fs.rm(dir, { recursive: true, force: true }).catch(() => {})
		})
		const record = await manager.delegate({
			parentSessionID: "ses_parent",
			parentMessageID: "msg_parent",
			parentAgent: "build",
			prompt: "Research the topic",
			agent: "researcher",
		})
		state.promptResolvers.get(record.sessionID)?.resolve({})
		// Parent notification unaffected: the human channel is purely additive.
		await waitFor(() => record.status === "complete")
		await waitFor(() => state.promptAsyncCalls.length > 0)
	})
})

describe("TUI sidecar mirror pin", () => {
	async function readSidecar(): Promise<string> {
		const sidecarPath = path.join(import.meta.dir, "..", "..", "tui", "background-agents-notify.tsx")
		return await fs.readFile(sidecarPath, "utf8")
	}

	test("the sidecar carries the same RPC id and event name as the server contract", async () => {
		const sidecar = await readSidecar()
		expect(sidecar).toContain(`"${TERMINAL_RPC_ID}"`)
		expect(sidecar).toContain(`"${TERMINAL_EVENT_NAME}"`)
	})

	test("the sidecar subscribes on the data bus under rpc.<id>.<event>", async () => {
		const sidecar = await readSidecar()
		// The host publishes plugin-RPC events as `rpc.<rpcID>.<event>` on the
		// normal data bus; the client-side RPC surface these once reached for
		// does not exist. A sidecar that reaches for one subscribes to nothing
		// and — because the failure is a silent TypeError — notifies no one.
		expect(sidecar).toContain("`rpc.${TERMINAL_RPC_ID}.${TERMINAL_EVENT_NAME}`")
		expect(sidecar).toContain("ctx.data.on(TERMINAL_EVENT_TYPE")

		// Assert absence against code only: comments legitimately name the
		// rejected APIs, and matching those would pin the prose, not the code.
		const code = sidecar
			.replace(/\/\*[\s\S]*?\*\//g, "")
			.replace(/^\s*\/\/.*$/gm, "")
		expect(code).not.toContain("client.rpc")
		expect(code).not.toContain("events.on(")
	})
})
