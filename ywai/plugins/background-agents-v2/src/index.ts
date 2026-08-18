/**
 * background-agents v2 — sync + async delegation for OpenCode v2.
 *
 * Ported from the v1 plugin to the v2 plugin API (Plugin.define). v2 natives
 * replaced the v1 machinery: steer = prompt with delivery:"steer", stop =
 * session.interrupt, parent notification = queued synthetic message, sync
 * completion = event-driven (the beta's session.wait is 503 "not available
 * yet"). The runtime import of @opencode-ai/plugin is deliberately avoided —
 * local-file plugins cannot resolve node_modules — so `define` is a local
 * identity and only type imports are used.
 */

import {
	createSessionIO,
	effortToVariant,
	formatModelRef,
	parseModelString,
} from "./adapter"
import { DelegationManager, defaultRecordsDir } from "./manager"
import { DELEGATION_RULES } from "./rules"

/**
 * Structural stand-in for the v2 plugin contract ({id, setup}). The real
 * Plugin.define is an identity function at runtime, and avoiding the value
 * import keeps the bundle loadable from a local file (no node_modules).
 */
interface V2Plugin {
	id: string
	setup: (ctx: any) => Promise<(() => Promise<void>) | void> | (() => Promise<void>) | void
}
const define = (plugin: V2Plugin): V2Plugin => plugin

const plugin = define({
	id: "background-agents",
	async setup(ctx: any) {
		const log = (msg: string) => {
			// v2 exposes no app.log to server plugins; stderr lands in the
			// server log, which is where operators look anyway.
			console.error(`[background-agents-v2] ${msg}`)
		}

		// Project-keyed records dir (directory-scoped like the v1 plugin).
		const projectKey = sanitizeKey(ctx.directory ?? process.cwd())
		const manager = new DelegationManager(
			createSessionIO(ctx.session),
			defaultRecordsDir(projectKey),
			log,
		)
		await manager.start()

		// ── event feed: transcripts, completion, watchdog inputs ──────────
		const subscribe = ctx.event?.subscribe?.bind(ctx.event)
		if (subscribe) {
			await subscribe((event: { type: string; data?: Record<string, unknown> }) => {
				try {
					manager.handleEvent(event)
				} catch (error) {
					log(`event handling failed: ${String(error)}`)
				}
			})
		} else {
			log("ctx.event.subscribe unavailable — delegations will rely on timeouts only")
		}

		// ── delegation rules in every session's system prompt ─────────────
		try {
			await ctx.session?.hook?.("context", (session: { system: unknown[] }) => {
				session.system.push({ type: "text", text: DELEGATION_RULES })
			})
		} catch (error) {
			log(`session context hook unavailable: ${String(error)}`)
		}

		// ── tools ──────────────────────────────────────────────────────────
		await ctx.tool.transform((draft: { add: (tool: unknown) => void }) => {
			draft.add(delegateTool(manager))
			draft.add(readTool(manager))
			draft.add(listTool(manager))
			draft.add(peekTool(manager))
			draft.add(steerTool(manager))
			draft.add(stopTool(manager))
			draft.add(statusTool(manager))
		})

		return async () => {
			await manager.stop()
		}
	},
})

function sanitizeKey(dir: string): string {
	return dir.replaceAll(/[^a-zA-Z0-9_-]/g, "_").slice(-80) || "default"
}

// ── tool definitions ───────────────────────────────────────────────────────

interface V2ToolContext {
	sessionID: string
	messageID: string
	agent: string
}

function delegateTool(manager: DelegationManager) {
	return {
		name: "delegate",
		description: `Delegate a task to an agent, sync or async.

Modes:
- mode="sync": blocks until the sub-agent finishes and returns its result inline. Use when the next step depends on the answer.
- mode="async" (default): returns an ID immediately; the task runs supervised in the background and a <task-notification> arrives on completion. Do NOT poll.

Selectors: agent (specialist), model ("provider/model-id" override), effort ("high"|"medium"|"low" reasoning), timeout_minutes (0 = no timeout; a steer restarts the window).
Supervise with delegation_peek / delegation_steer / delegation_stop; read full output with delegation_read.`,
		input: {
			type: "object",
			properties: {
				prompt: {
					type: "string",
					description: "The full detailed prompt for the agent. Must be in English.",
				},
				agent: {
					type: "string",
					description: "Agent to delegate to (any configured sub-agent).",
				},
				mode: {
					type: "string",
					enum: ["sync", "async"],
					description: 'sync = wait for the result inline; async (default) = run in background.',
				},
				model: {
					type: "string",
					description:
						'Optional model override for THIS delegation as "provider/model-id" (e.g. "anthropic/claude-haiku-4-5"). Omitted = the agent\'s configured model.',
				},
				effort: {
					type: "string",
					enum: ["high", "medium", "low"],
					description:
						"Optional reasoning effort for THIS delegation (model variant). Omitted = the model's default.",
				},
				timeout_minutes: {
					type: "number",
					description:
						"Optional max runtime in minutes (0 = NO timeout). Default 15. A delivered steer restarts the window.",
				},
			},
			required: ["prompt", "agent"],
		},
		async execute(
			args: {
				prompt: string
				agent: string
				mode?: string
				model?: string
				effort?: string
				timeout_minutes?: number
			},
			toolCtx: V2ToolContext,
		) {
			if (!toolCtx?.sessionID) return "❌ delegate requires sessionID. This is a system error."

			const mode = args.mode === "sync" ? "sync" : "async"

			let model
			if (args.model !== undefined) {
				model = parseModelString(args.model)
				if (!model) {
					return `❌ Invalid model "${args.model}". Expected "provider/model-id" (e.g. "anthropic/claude-haiku-4-5"), or omit it.`
				}
			}
			if (args.effort !== undefined) {
				// v2 hangs effort on the model ref (variant); without a model
				// selector there is nothing to attach it to.
				if (!model) {
					return `❌ effort requires a model selector: pass model="provider/model-id" together with effort.`
				}
				model = { ...model, variant: effortToVariant(args.effort) }
			}

			try {
				const outcome = await manager.delegate({
					parentSessionID: toolCtx.sessionID,
					parentMessageID: toolCtx.messageID,
					parentAgent: toolCtx.agent,
					prompt: args.prompt,
					agent: args.agent,
					mode,
					model,
					maxRunTimeMs:
						args.timeout_minutes !== undefined
							? args.timeout_minutes * 60_000
							: undefined,
				})

				if (outcome.mode === "sync") {
					const label = outcome.status === "completed" ? "COMPLETED" : outcome.status.toUpperCase()
					const header = `Delegation ${outcome.id} (${args.agent}) ${label}`
					const error = outcome.error ? `\nError: ${outcome.error}` : ""
					return {
						title: `${args.agent} · ${outcome.id}`,
						output: `${header}${error}\n\n${outcome.result || "_(no output)_"}`,
					}
				}

				const modelPart = model ? `\nModel: ${formatModelRef(model)}` : ""
				const timeoutLabel =
					outcome.timeoutMs > 0
						? `${Math.round(outcome.timeoutMs / 60_000)}min (a steer resets the window)`
						: "none (steer/stop it whenever needed)"
				let response =
					`Delegation started: ${outcome.id}\nAgent: ${args.agent}${modelPart}\nMode: async\nTimeout: ${timeoutLabel}`
				response += `\nYou WILL be notified when complete. Do NOT poll — use delegation_peek only when a mid-run decision needs evidence.`
				return { title: `${args.agent} · ${outcome.id}`, output: response }
			} catch (error) {
				return `❌ Delegation failed:\n\n${error instanceof Error ? error.message : "Unknown error"}`
			}
		},
	}
}

function readTool(manager: DelegationManager) {
	return {
		name: "delegation_read",
		description:
			"Read the output of a delegation by its ID — full persisted result, also after compaction or restart.",
		input: {
			type: "object",
			properties: { id: { type: "string", description: "The delegation ID." } },
			required: ["id"],
		},
		async execute(args: { id: string }, toolCtx: V2ToolContext) {
			if (!toolCtx?.sessionID) return "❌ delegation_read requires sessionID."
			return { title: args.id, output: await manager.read(toolCtx.sessionID, args.id) }
		},
	}
}

function listTool(manager: DelegationManager) {
	return {
		name: "delegation_list",
		description: "List all delegations for the current session (running and completed).",
		input: { type: "object", properties: {} },
		async execute(_args: unknown, toolCtx: V2ToolContext) {
			if (!toolCtx?.sessionID) return "❌ delegation_list requires sessionID."
			const items = await manager.list(toolCtx.sessionID)
			if (items.length === 0) return "No delegations found for this session."
			const lines = items.map(
				(d) =>
					`- **${d.id}**${d.title ? ` | ${d.title}` : ""} [${d.status}]${d.unread ? " [unread]" : ""}` +
					(d.description ? `\n  → ${d.description}` : ""),
			)
			return `## Delegations\n\n${lines.join("\n")}`
		},
	}
}

function peekTool(manager: DelegationManager) {
	return {
		name: "delegation_peek",
		description:
			"Peek at the LIVE transcript of a RUNNING delegation without blocking or stopping it. Digest of assistant text + tool activity. Do NOT call in a polling loop.",
		input: {
			type: "object",
			properties: { id: { type: "string", description: "The delegation ID to peek at." } },
			required: ["id"],
		},
		async execute(args: { id: string }, toolCtx: V2ToolContext) {
			if (!toolCtx?.sessionID) return "❌ delegation_peek requires sessionID."
			return { title: args.id, output: await manager.peek(toolCtx.sessionID, args.id) }
		},
	}
}

function steerTool(manager: DelegationManager) {
	return {
		name: "delegation_steer",
		description:
			"Send an extra instruction to a RUNNING delegation without stopping it (native v2 steering). Adds a constraint, redirects focus, or supplies missing context mid-run. Finished tasks reject.",
		input: {
			type: "object",
			properties: {
				id: { type: "string", description: "The delegation ID to steer." },
				message: { type: "string", description: "The additional instruction to inject." },
			},
			required: ["id", "message"],
		},
		async execute(args: { id: string; message: string }, toolCtx: V2ToolContext) {
			if (!toolCtx?.sessionID) return "❌ delegation_steer requires sessionID."
			return {
				title: args.id,
				output: await manager.steer(toolCtx.sessionID, args.id, args.message),
			}
		},
	}
}

function stopTool(manager: DelegationManager) {
	return {
		name: "delegation_stop",
		description:
			"Stop a RUNNING delegation (native interrupt). Aborts its session and saves partial output — read it afterwards with delegation_read(id).",
		input: {
			type: "object",
			properties: { id: { type: "string", description: "The delegation ID to stop." } },
			required: ["id"],
		},
		async execute(args: { id: string }, toolCtx: V2ToolContext) {
			if (!toolCtx?.sessionID) return "❌ delegation_stop requires sessionID."
			return { title: args.id, output: await manager.stopDelegation(toolCtx.sessionID, args.id) }
		},
	}
}

function statusTool(manager: DelegationManager) {
	return {
		name: "delegation_status",
		description:
			"Live status of running delegations: elapsed, deadline, tool calls, steers. Cheap, non-blocking — does NOT wait for completion.",
		input: { type: "object", properties: {} },
		async execute(_args: unknown, toolCtx: V2ToolContext) {
			if (!toolCtx?.sessionID) return "❌ delegation_status requires sessionID."
			return await manager.statusReport(toolCtx.sessionID)
		},
	}
}

export default plugin
export { DelegationManager }
