import { describe, expect, test } from "bun:test"
import * as fs from "node:fs/promises"
import * as path from "node:path"
import {
	BackgroundAgentsTerminalRpc,
	TERMINAL_EVENT_NAME,
	TERMINAL_RPC_ID,
	tryRegisterTerminalRpc,
} from "../src/plugin/terminal-events"

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
