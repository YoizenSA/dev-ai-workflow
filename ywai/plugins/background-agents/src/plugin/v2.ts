/**
 * OpenCode v2 wiring for background-agents.
 *
 * Launching is native: the built-in `subagent` tool with `background: true`
 * creates the child, resolves agent/model, runs it and notifies the parent.
 * This plugin only hooks that tool to TRACK background children and exposes
 * four supervision tools over them.
 *
 *   timeout   `timeout_minutes` is advertised on the native schema through
 *             the context hook and stripped in execute.before, so native
 *             input decoding never sees it; default from
 *             BACKGROUND_AGENTS_TIMEOUT_MINUTES (15, 0 = none)
 *   tracking  execute.after on `subagent` whose result status is "running"
 */

import * as os from "node:os"
import * as path from "node:path"
import { createJsonErrorRecoveryHook } from "./json-error-recovery"
import { Supervisor } from "./supervisor"
import { TERMINAL_EVENT_NAME, tryRegisterTerminalRpc } from "./terminal-events"
import { createToolLoopGuardHook } from "./tool-loop-guard"
import type { V2PluginContext } from "../../../shared/v2"

const DEFAULT_TIMEOUT_MINUTES = (() => {
	const minutes = Number(process.env.BACKGROUND_AGENTS_TIMEOUT_MINUTES)
	return Number.isFinite(minutes) && minutes >= 0 ? minutes : 15
})()

const TIMEOUT_SCHEMA = {
	type: "number",
	minimum: 0,
	description: `Background runs only: abort the subagent after this many minutes (default ${DEFAULT_TIMEOUT_MINUTES}, 0 = no timeout). A subagent_steer restarts the window.`,
}

const ID = { type: "string", description: "The subagent's sessionID (from the launch result or its <subagent> notification)." }

const TOOLS: Array<{
	name: string
	description: string
	input: Record<string, any>
	run: (sup: Supervisor, caller: string, input: any) => Promise<string>
}> = [
	{
		name: "subagent_status",
		description: `Show background subagents of this session.
Without an id: every one — running and finished — with status, elapsed time and timeout.
With an id: that subagent's live transcript digest, to decide whether to steer, stop, or let it run.
Read-only. Do NOT poll: completion arrives as a <subagent> notification.`,
		input: { type: "object", properties: { id: { ...ID, description: "Optional sessionID. Omit for the session-wide list." } }, additionalProperties: false },
		run: (sup, caller, input) => sup.status(caller, input?.id),
	},
	{
		name: "subagent_read",
		description: "Read a background subagent's final (or partial) output by sessionID — use after compaction when the <subagent> notification is gone.",
		input: { type: "object", properties: { id: ID }, required: ["id"], additionalProperties: false },
		run: (sup, caller, input) => sup.read(caller, input.id),
	},
	{
		name: "subagent_steer",
		description: "Send an extra instruction into a RUNNING background subagent's current run (native steer). Resets its timeout window.",
		input: {
			type: "object",
			properties: { id: ID, message: { type: "string", description: "The instruction to inject. Must be in English." } },
			required: ["id", "message"],
			additionalProperties: false,
		},
		run: (sup, caller, input) => sup.steer(caller, input.id, input.message),
	},
	{
		name: "subagent_stop",
		description: "Abort a RUNNING background subagent and return its partial output.",
		input: { type: "object", properties: { id: ID }, required: ["id"], additionalProperties: false },
		run: (sup, caller, input) => sup.stop(caller, input.id),
	},
]

export async function setupV2(ctx: V2PluginContext): Promise<() => void> {
	const project = ctx.location?.project?.id ?? "global"
	const terminalRpc = await tryRegisterTerminalRpc(ctx)
	const supervisor = new Supervisor({
		session: ctx.session as any,
		indexFile: path.join(os.homedir(), ".local", "share", "opencode", "delegations", project, "subagents.json"),
		defaultTimeoutMs: DEFAULT_TIMEOUT_MINUTES * 60_000,
		onTerminal: terminalRpc
			? (event) => void terminalRpc.events.emit(TERMINAL_EVENT_NAME, event).catch(() => {})
			: undefined,
	})
	supervisor.restore()

	await ctx.tool?.transform?.((editor) => {
		for (const def of TOOLS) {
			editor.add({
				name: def.name,
				description: def.description,
				input: def.input,
				execute: async (input: any, toolCtx: any) => {
					if (!toolCtx?.sessionID) return { content: `❌ ${def.name} requires sessionID. This is a system error.` }
					return { content: await def.run(supervisor, toolCtx.sessionID, input) }
				},
			})
		}
	})

	// Per-call timeout overrides, keyed by tool call id, from before → after.
	const timeouts = new Map<string, number>()
	const loopGuard = createToolLoopGuardHook()
	const jsonRecovery = createJsonErrorRecoveryHook()

	await ctx.tool?.hook?.("execute.before", async (event: any) => {
		if (event.tool === "subagent" && event.input && typeof event.input === "object" && "timeout_minutes" in event.input) {
			const { timeout_minutes, ...rest } = event.input
			const minutes = Number(timeout_minutes)
			if (Number.isFinite(minutes) && minutes >= 0) timeouts.set(event.id, minutes * 60_000)
			event.input = rest
		}
		const output = { args: event.input }
		await loopGuard["tool.execute.before"](event, output as never)
		event.input = output.args
	})

	await ctx.tool?.hook?.("execute.after", async (event: any) => {
		if (event.tool === "subagent") {
			const timeoutMs = timeouts.get(event.id)
			timeouts.delete(event.id)
			const meta = event.result?.metadata
			if (event.status === "completed" && meta?.status === "running" && typeof meta.sessionID === "string") {
				supervisor.track({
					sessionID: meta.sessionID,
					parentSessionID: event.sessionID,
					agent: String(event.input?.agent ?? ""),
					description: String(event.input?.description ?? ""),
					timeoutMs,
				})
			}
		}
		const output = { output: typeof event.result?.content === "string" ? event.result.content : undefined }
		await jsonRecovery["tool.execute.after"](event, output as never)
		await loopGuard["tool.execute.after"](event, output as never)
		if (typeof output.output === "string" && event.result) event.result.content = output.output
	})

	await ctx.session.hook?.("context", (event: any) => {
		const input = event.tools?.subagent?.input
		if (input && typeof input === "object") {
			input.properties = { ...(input.properties ?? {}), timeout_minutes: TIMEOUT_SCHEMA }
		}
	})

	return () => {
		supervisor.dispose()
		void Promise.resolve(terminalRpc?.dispose?.()).catch(() => {})
	}
}
