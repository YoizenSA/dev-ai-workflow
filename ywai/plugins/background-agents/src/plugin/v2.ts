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
import { DelegationManager } from "./delegation-manager"
import { createLogger } from "./logger"
import { getProjectId } from "./primitives/get-project-id"
import type { OpencodeClient } from "./primitives/types"
import { DELEGATION_RULES } from "./rules"
import {
	createSubagent,
	createSubagentStatus,
	createDelegationRead,
	createDelegationSteer,
	createDelegationStop,
} from "./tools"
import { DEFAULT_MAX_RUN_TIME_MS } from "./types"

/**
 * The slice of the v2 plugin context this module uses. Typed loosely on
 * purpose: the v2 plugin API is beta and its effect-based SDK types churn,
 * so every boundary here is structural.
 */
interface V2PluginContext {
	location: { directory: string }
	session: Record<string, (...args: any[]) => Promise<any>>
	agent: Record<string, (...args: any[]) => Promise<any>>
	tool: Record<string, any>
	event: { subscribe(options?: { signal?: AbortSignal }): AsyncIterable<any> }
}

/** v1 SDK responses are {data}-wrapped; some v2 ctx returns may be too. */
function unwrap(value: any): any {
	return value && typeof value === "object" && !Array.isArray(value) && "data" in value ? value.data : value
}

/** First array hidden inside a client-style envelope. */
function toArray(value: any): any[] {
	const inner = unwrap(value)
	if (Array.isArray(inner)) return inner
	return inner?.agents ?? inner?.sessions ?? inner?.items ?? []
}

/**
 * A v1-SDK-shaped client over the v2 plugin context, covering exactly the
 * calls DelegationManager makes. Each mapping degrades consciously:
 *
 * - session.status: v2 has no server-wide status poll; returning undefined
 *   data means "unknown", which every caller already handles by falling back
 *   to event-driven heartbeats.
 * - session.delete: v2 exposes no session deletion; finished-delegation
 *   cleanup becomes a no-op, so old delegation sessions stay listed.
 * - session.promptAsync: delivered as a "steer" prompt, which is the v2
 *   spelling of "inject this into a running session".
 */
export function createV1ShapedClient(ctx: V2PluginContext): OpencodeClient {
	const session = {
		async status(): Promise<{ data: undefined }> {
			return { data: undefined }
		},
		async get(input: { path: { id: string } }): Promise<{ data: any }> {
			return { data: unwrap(await ctx.session.get({ sessionID: input.path.id })) }
		},
		async promptAsync(input: {
			path: { id: string }
			body?: { agent?: string; parts?: Array<{ type: string; text?: string }> }
		}): Promise<{ data: undefined }> {
			const text = (input.body?.parts ?? [])
				.filter((part) => part.type === "text")
				.map((part) => part.text ?? "")
				.join("\n")
			if (input.body?.agent) {
				await ctx.session.switchAgent({ sessionID: input.path.id, agent: input.body.agent })
			}
			await ctx.session.prompt({
				sessionID: input.path.id,
				text,
				delivery: "steer",
			})
			return { data: undefined }
		},
		async abort(input: { path: { id: string } }): Promise<{ data: undefined }> {
			await ctx.session.interrupt({ sessionID: input.path.id })
			return { data: undefined }
		},
		async delete(_input: { path: { id: string } }): Promise<{ data: undefined }> {
			// v2 exposes no session deletion to plugins; cleanup is skipped.
			return { data: undefined }
		},
		async messages(input: { path: { id: string } }): Promise<{ data: any }> {
			const items = toArray(await ctx.session.context({ sessionID: input.path.id }))
			// v1 messages are {info:{role,...}, parts:[...]}; v2 flattens role to
			// the top level. Normalize so the transcript digest works on both.
			return {
				data: items.map((item: any) => {
					if (!item || typeof item !== "object") return item
					const role = item.info?.role ?? item.message?.role ?? item.role ?? item.type
					const parts = item.parts ?? item.content ?? item.info?.parts ?? []
					return { ...item, info: { ...item, role }, parts }
				}),
			}
		},
		async prompt(input: {
			path: { id: string }
			body?: {
				agent?: string
				model?: { providerID: string; modelID: string; variant?: string }
				parts?: Array<{ type: string; text?: string }>
			}
		}): Promise<{ data: { parts: never[] } }> {
			const text = (input.body?.parts ?? [])
				.filter((part) => part.type === "text")
				.map((part) => part.text ?? "")
				.join("\n")
			// v2's SessionPromptInput carries no model field: the override lives
			// on session.switchModel. Passing it to prompt() was silently
			// dropped, so every delegation ran on the agent's configured model.
			// The variant rides along — it is how `effort` reaches the model.
			const model = input.body?.model
				? {
						providerID: input.body.model.providerID,
						id: input.body.model.modelID,
						...(input.body.model.variant ? { variant: input.body.model.variant } : {}),
					}
				: undefined
			// Same story as the model: SessionPromptInput carries no agent field
			// either, so passing it to prompt() left every delegation on v2's
			// default agent ("build") no matter which agent was requested.
			if (input.body?.agent) {
				await ctx.session.switchAgent({ sessionID: input.path.id, agent: input.body.agent })
			}
			if (model) {
				await ctx.session.switchModel({ sessionID: input.path.id, model })
			}
			await ctx.session.prompt({
				sessionID: input.path.id,
				text,
				delivery: "steer",
			})
			// v1's prompt() resolves when the assistant turn is done; v2's
			// resolves at admission. Riding out the turn keeps the manager's
			// turn-error detection and completion scheduling meaningful.
			try {
				await ctx.session.wait({ sessionID: input.path.id })
			} catch {
				// Waiting is best-effort; the idle-event path still finalizes.
			}
			// No assistant parts here: metadata generation falls back, and the
			// initial delegation prompt only reads the turn's error field.
			return { data: { parts: [] } }
		},
		async create(input: { body?: { title?: string; parentID?: string } }): Promise<{ data: any }> {
			// v2's host drops parentID (known beta degradation: the delegation
			// session is a root session rather than a child).
			return {
				data: unwrap(
					await ctx.session.create({ title: input.body?.title, parentID: input.body?.parentID }),
				),
			}
		},
	}
	return {
		session,
		config: {
			// v2 plugin contexts expose no config reader; callers degrade to
			// their fallbacks (metadata without small_model, capability parse
			// without an agent permission map).
			async get(): Promise<{ data: undefined }> {
				return { data: undefined }
			},
		},
		app: {
			// v2 plugin contexts have no app-log endpoint; debug logging
			// degrades to the manager's file logger.
			async log(): Promise<{ data: undefined }> {
				return { data: undefined }
			},
			async agents() {
				const agents = toArray(await ctx.agent.list())
				return {
					data: agents.map((agent) => ({
						name: agent.id ?? agent.name,
						description: agent.description,
						mode: agent.mode,
					})),
				}
			},
		},
		tui: {
			// v2 plugin contexts have no TUI surface; toasts are dropped.
			async showToast(): Promise<void> {},
		},
	} as unknown as OpencodeClient
}

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

	await ctx.tool.transform((editor: any) => {
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

	// System injection + delegation context. v2 runs the context hook for
	// compaction calls too, so this also carries delegation state across
	// compaction — the role the v1 compacting hook played.
	await ctx.session.hook("context", async (event: any) => {
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
	await ctx.session.hook("prompt", (event: any) => {
		try {
			const pending = manager.drainPendingNotificationText(event.sessionID)
			if (pending) {
				event.prompt.text = event.prompt.text ? `${event.prompt.text}\n\n${pending}` : pending
			}
		} catch {
			// Admission must proceed even if notification draining fails.
		}
	})

	// Event loop: same handlers as the v1 event hook, tolerant of either the
	// v1 envelope (properties) or a v2 envelope (data).
	const controller = new AbortController()
	void (async () => {
		try {
			for await (const raw of ctx.event.subscribe({ signal: controller.signal })) {
				const evt: any = raw ?? {}
				const type: string = evt.type ?? ""
				const props: any = evt.properties ?? evt.data ?? {}
				if (type === "session.status") {
					const statusType = props.status?.type
					if (statusType === "idle" && props.sessionID) {
						await manager.handleSessionIdle(props.sessionID)
					}
				}
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
		} catch {
			// Stream ended or errored; the manager's polling fallbacks cover the gap.
		}
	})()

	return () => controller.abort()
}
