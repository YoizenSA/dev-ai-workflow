/**
 * OpenCode v2 plugin surface for background-agents.
 *
 * v2 loads only plugins whose default export is an object carrying an id and a
 * setup() (or effect()); the v1 hooks object is rejected at load with
 * "Plugin must export a default definition with an id and an effect or setup
 * function". This module is the v2 half of the dual export recommended by the
 * v2 plugins guide ("Support V1"): setup() rebuilds the delegation surface on
 * the v2 context —
 *
 *   v1 hook                     v2 equivalent
 *   --------------------------  -------------------------------------------
 *   tool map                    ctx.tool.transform(editor.add(...))
 *   "tool.execute.before"       not ported: v2's built-in subagent tool is
 *                               replaced wholesale, so there is no task
 *                               tool left to guard
 *   system transform            ctx.session.hook("context")
 *   "chat.message"              ctx.session.hook("prompt") admission
 *   compacting hook             the same "context" hook — v2 runs it for
 *                               compaction calls too
 *   event hook                  ctx.event.subscribe() loop
 *
 * The DelegationManager is reused untouched: it is constructed against a
 * v1-client-shaped facade that maps the calls it makes onto the v2 context
 * (promptAsync → prompt(delivery "steer"), abort → interrupt, messages →
 * context, app.agents → agent.list; session.status and session.delete have no
 * v2 equivalent and degrade to "unknown" and a no-op respectively).
 */

import * as fs from "node:fs/promises"
import * as os from "node:os"
import * as path from "node:path"
import { formatDelegationContext } from "./context"
import { createJsonErrorRecoveryHook } from "./json-error-recovery"
import { createToolLoopGuardHook } from "./tool-loop-guard"
import { DelegationManager } from "./delegation-manager"
import { createLogger } from "./logger"
import { getProjectId } from "./primitives/get-project-id"
import { DELEGATION_RULES } from "./rules"
import {
	createSubagent,
	createSubagentStatus,
	createDelegationRead,
	createDelegationSteer,
	createDelegationStop,
} from "./tools"
import { DEFAULT_MAX_RUN_TIME_MS } from "./types"
import {
	createV1ShapedClient,
	mapV2EventToV1,
	type V2PluginContext,
} from "../../../shared/v2"

export { createV1ShapedClient }

/** v1 tool defs return plain strings or {title, output}; v2 wants content. */
function toV2Result(result: any): { content: string; metadata?: Record<string, string> } {
	if (typeof result === "string") return { content: result }
	if (result && typeof result === "object" && typeof result.output === "string") {
		return result.title
			? { content: result.output, metadata: { title: result.title } }
			: { content: result.output }
	}
	return { content: String(result ?? "") }
}

/**
 * Plain JSON Schema mirrors of the v1 `args` builders. The v1 defs build
 * schemas through the v1-only `tool.schema` helper, which the v2 editor does
 * not accept; the descriptions here are kept in sync with tools.ts by hand.
 */
const V2_INPUT_SCHEMAS: Record<string, Record<string, any>> = {
	subagent: {
		type: "object",
		properties: {
			agent: { type: "string", description: "The specialized agent to run the task in." },
			prompt: {
				type: "string",
				description: "The full detailed task instructions. Must be in English.",
			},
			description: {
				type: "string",
				description: "Optional 3-5 word task label, as accepted by the built-in subagent tool.",
			},
			timeout_minutes: {
				type: "integer",
				minimum: 0,
				description: `Optional max runtime in minutes for THIS run (default ${Math.round(
					DEFAULT_MAX_RUN_TIME_MS / 60_000,
				)}). Use 0 for NO timeout — you stay in control via subagent_steer/subagent_stop.`,
			},
			model: {
				type: "string",
				description:
					'Optional model override for THIS run as "provider/model-id" (e.g. "anthropic/claude-haiku-4-5"). Omitted = the agent\'s configured model.',
			},
			effort: {
				type: "string",
				description:
					'Optional reasoning effort for THIS run: "high", "medium" or "low". Applied as the model\'s variant, so it needs a model that publishes one.',
			},
			background: {
				type: "boolean",
				description:
					"Accepted for compatibility with the built-in tool and ignored: every run here is asynchronous already.",
			},
		},
		required: ["agent", "prompt"],
		additionalProperties: false,
	},
	subagent_status: {
		type: "object",
		properties: {
			id: {
				type: "string",
				description:
					"Optional delegation ID (e.g. 'elegant-blue-tiger'). Omit for the session-wide report; supply it for that delegation's live transcript digest.",
			},
		},
		additionalProperties: false,
	},
	subagent_read: {
		type: "object",
		properties: {
			id: { type: "string", description: "The delegation ID (e.g., 'elegant-blue-tiger')" },
		},
		required: ["id"],
		additionalProperties: false,
	},
	subagent_steer: {
		type: "object",
		properties: {
			id: { type: "string", description: "The delegation ID to steer." },
			message: {
				type: "string",
				description: "The extra instruction to deliver to the running delegation.",
			},
		},
		required: ["id", "message"],
		additionalProperties: false,
	},
	subagent_stop: {
		type: "object",
		properties: {
			id: { type: "string", description: "The delegation ID to stop." },
		},
		required: ["id"],
		additionalProperties: false,
	},
}

/**
 * Registers the five-tool v2 surface. The v1 defs are reused for description
 * and execute so the two surfaces cannot drift; only the schema shape is
 * v2-native. Registering `subagent` overrides v2's built-in tool of the same
 * name — a later valid registration replaces the same effective tool name.
 */
async function registerV2Tools(ctx: V2PluginContext, manager: DelegationManager): Promise<void> {
	const defs: Record<string, { description: string; execute: (args: any, toolCtx: any) => Promise<any> }> = {
		subagent: createSubagent(manager) as any,
		subagent_status: createSubagentStatus(manager) as any,
		subagent_read: createDelegationRead(manager) as any,
		subagent_steer: createDelegationSteer(manager) as any,
		subagent_stop: createDelegationStop(manager) as any,
	}

	await ctx.tool?.transform?.((editor: any) => {
		for (const [name, def] of Object.entries(defs)) {
			editor.add({
				name,
				description: def.description,
				input: V2_INPUT_SCHEMAS[name],
				execute: async (input: any, toolCtx: any) => toV2Result(await def.execute(input, toolCtx)),
			})
		}
	})
}

/** v2 setup: the whole delegation surface on the v2 context. */
export async function setupV2(ctx: V2PluginContext): Promise<(() => void) | undefined> {
	const directory = ctx.location?.directory ?? process.cwd()
	const client = createV1ShapedClient(ctx)

	const log = createLogger(client)
	const projectId = await getProjectId(directory, client)
	const baseDir = path.join(os.homedir(), ".local", "share", "opencode", "delegations", projectId)
	await fs.mkdir(baseDir, { recursive: true })

	// No nativeSteer on v2: steering rides the facade's prompt(delivery:"steer").
	const manager = new DelegationManager(client, baseDir, log, {})

	await manager.debugLog("background-agents v2 setup initialized")

	// Re-adopt delegations orphaned by a previous process exit (fire-and-forget).
	void manager.restoreActiveDelegations()

	await registerV2Tools(ctx, manager)

	// Guard hooks: loop guard + JSON recovery on the v2 tool stream. Each
	// hook degrades to a no-op when the host lacks tool.hook; log a line via
	// the manager so the gap is visible in debug.
	if (typeof ctx.tool?.hook === "function") {
		const loopGuard = createToolLoopGuardHook()
		const jsonRecovery = createJsonErrorRecoveryHook()
		const guardAfter = async (event: { tool: string; sessionID?: string; callID?: string; result?: any }) => {
			const output = { output: typeof event.result?.content === "string" ? event.result.content : undefined }
			await jsonRecovery["tool.execute.after"](event as never, output as never)
			await loopGuard["tool.execute.after"](event as never, output as never)
			if (typeof output.output === "string" && event.result) {
				event.result.content = output.output
			}
		}
		await ctx.tool.hook("execute.after", guardAfter)
		await ctx.tool.hook("execute.before", async (event: { tool: string; sessionID?: string; callID?: string; input?: unknown }) => {
			const output = { args: event.input }
			await loopGuard["tool.execute.before"](event as never, output as never)
			event.input = output.args
		})
	}

	// System injection + delegation context. v2 runs the context hook for
	// compaction calls too, so this also carries delegation state across
	// compaction — the role the v1 compacting hook played.
	await ctx.session.hook?.("context", async (event: any) => {
		try {
			event.system.push({ type: "text", text: DELEGATION_RULES })
			const rootSessionID = await manager.getRootSessionID(event.sessionID)
			const running = manager.getRunningDelegations(rootSessionID).map((d) => ({
				id: d.id,
				agent: d.agent,
				title: d.title,
				description: d.description,
				status: d.status,
				startedAt: d.startedAt,
				lastHeartbeatAt: d.progress.lastHeartbeatAt,
				prompt: d.prompt,
			}))
			const unreadCompleted = manager.getUnreadCompletedDelegations(rootSessionID, 10).map((d) => ({
				id: d.id,
				agent: d.agent,
				title: d.title,
				description: d.description,
				status: d.status,
				completedAt: d.completedAt,
			}))
			if (running.length > 0 || unreadCompleted.length > 0) {
				event.system.push({ type: "text", text: formatDelegationContext(running, unreadCompleted) })
			}
		} catch {
			// Context injection must never break the model call.
		}
	})

	// Deliver queued parent notifications on the next user turn (the v2
	// spelling of the v1 chat.message hook).
	await ctx.session.hook?.("prompt", (event: any) => {
		try {
			const pending = manager.drainPendingNotificationText(event.sessionID)
			if (pending) {
				event.prompt.text = event.prompt.text ? `${event.prompt.text}\n\n${pending}` : pending
			}
		} catch {
			// Admission must proceed even if notification draining fails.
		}
	})

	// Event loop: same handlers as the v1 event hook. mapV2EventToV1 turns
	// v2 event shapes into the v1 shapes the manager reads; the raw v2 event
	// always passes through first, so v2-native shapes still reach the
	// handlers that understand them.
	const controller = new AbortController()
	void (async () => {
		try {
			for await (const raw of ctx.event.subscribe({ signal: controller.signal })) {
				for (const evt of mapV2EventToV1(raw ?? {})) {
					const type: string = evt.type ?? ""
					const props: any = evt.properties ?? evt.data ?? {}
					if (type === "session.idle" && props.sessionID) {
						await manager.handleSessionIdle(props.sessionID)
					}
					if (type === "message.updated") {
						const sessionID = props.info?.sessionID ?? props.sessionID
						if (sessionID) manager.handleMessageEvent(sessionID)
					}
					if (type === "message.part.updated") {
						const part = props.part
						if (part) manager.handlePartEvent(part)
					}
				}
			}
		} catch {
			// Stream ended or errored; the manager's polling fallbacks cover the gap.
		}
	})()

	return () => controller.abort()
}
