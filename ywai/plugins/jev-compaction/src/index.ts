/**
 * jev-compaction - drops stale tool calls and results before each model
 * request, using Jev decisions from fast-jev-compaction (vendored).
 *
 * Runs on the v2 `context` session hook, which lets a plugin rewrite
 * `event.messages` per request. Nothing is persisted: the plan is re-applied
 * on every request from an in-memory cache, and Jev is only asked again when
 * the history is over the threshold and holds tool calls it has not decided.
 *
 * Fail open on purpose: this plugin only saves tokens. No key, a Jev error,
 * or an unfittable history leaves the messages untouched.
 */
import type { V2PluginContext, V2ToolEditor } from "../../shared/v2"
import { readFileSync } from "node:fs"
import { findApiKey, keyFileCandidates } from "../../jev-gate/src/adapters/key"
import { compactMessages } from "../vendor/fast-jev-compaction/messages"
import { collectToolCalls } from "../vendor/fast-jev-compaction/state"
import { applyPlan, planFrom, toLib, type Plan, type V2Message } from "./adapter"

const PRESERVE_RECENT = 6 // the library default, needed to know which calls it pins

interface SessionCache {
	plan: Plan
	/** Non-pinned tool ids Jev already decided on; first decision wins. */
	decided: Set<string>
	busy: boolean
	warned: boolean
}

const sessions = new Map<string, SessionCache>()
/** Sessions that asked for compaction on their next request, threshold aside. */
const forced = new Set<string>()

export function requestCompaction(sessionID: string): void {
	forced.add(sessionID)
}

let projectDir: string | undefined

/**
 * JEV_COMPACTION_THRESHOLD_TOKENS, else `compactionThresholdTokens` in the
 * first jev-gate.json that has it (same files as the key), else 200k. The env
 * alone is not enough: v2 plugins do not inherit the shell's environment.
 */
export function threshold(): number {
	const fromEnv = Number(process.env.JEV_COMPACTION_THRESHOLD_TOKENS)
	if (Number.isFinite(fromEnv) && fromEnv > 0) return fromEnv
	for (const path of keyFileCandidates(projectDir)) {
		try {
			const value = Number(JSON.parse(readFileSync(path, "utf8")).compactionThresholdTokens)
			if (Number.isFinite(value) && value > 0) return value
		} catch {
			// Missing or malformed: try the next file.
		}
	}
	return 200_000
}

// ponytail: chars/4 over the JSON; swap for the model's reported usage if the trigger misfires.
function estimateTokens(messages: readonly V2Message[]): number {
	return Math.ceil(JSON.stringify(messages).length / 4)
}

export async function compactEvent(
	event: { sessionID: string; messages: V2Message[] },
	apiKey: string | undefined,
	compact: typeof compactMessages = compactMessages,
): Promise<void> {
	let cache = sessions.get(event.sessionID)
	if (!cache) {
		cache = { plan: new Map(), decided: new Set(), busy: false, warned: false }
		sessions.set(event.sessionID, cache)
	}

	apply(event.messages, cache.plan)

	// Measured after the cached plan, so Jev runs only when the compacted view
	// grows past the threshold again, not on every turn with a new tool call.
	const force = forced.delete(event.sessionID)
	if (apiKey && !cache.busy && (force || estimateTokens(event.messages) > threshold())) {
		const lib = toLib(event.messages)
		const open = collectToolCalls(lib, PRESERVE_RECENT).filter(
			(call) => !call.pinned && !cache.decided.has(call.tool_use_id),
		)
		if (open.length > 0) {
			cache.busy = true
			try {
				const result = await compact(lib, { apiKey, preserveRecentMessages: PRESERVE_RECENT })
				for (const [id, entry] of planFrom(lib, result.messages)) {
					if (!cache.plan.has(id)) cache.plan.set(id, entry)
				}
				for (const call of collectToolCalls(lib, PRESERVE_RECENT)) {
					if (!call.pinned) cache.decided.add(call.tool_use_id)
				}
				apply(event.messages, cache.plan)
			} catch (err) {
				if (!cache.warned) {
					cache.warned = true
					console.warn(`[jev-compaction] left messages untouched: ${(err as Error).message}`)
				}
			} finally {
				cache.busy = false
			}
		}
	}
}

// In place: the host reads the array it handed us.
function apply(messages: V2Message[], plan: Plan): void {
	if (plan.size > 0) messages.splice(0, messages.length, ...applyPlan(messages, plan))
}

/** v2 reads `id` and `setup()` from the default export. */
export default {
	id: "ywai-jev-compaction",
	setup: async (ctx: V2PluginContext) => {
		if (!ctx.session.hook) return
		projectDir = ctx.location?.directory
		await ctx.tool?.transform?.((editor: V2ToolEditor) =>
			editor.add({
				name: "jev_compact",
				description:
					"Compact this session's history with Jev on the next request, even under the " +
					"token threshold: stale tool calls and results are dropped or truncated, " +
					"everything else stays verbatim. Needs a TypeSafe API key.",
				input: { type: "object", properties: {} },
				options: { codemode: true },
				execute: async (_input: unknown, toolCtx?: { sessionID?: string }) => {
					if (!toolCtx?.sessionID) return { content: "No session id; nothing scheduled." }
					requestCompaction(toolCtx.sessionID)
					return { content: "Jev compaction scheduled for the next request." }
				},
			} as Parameters<V2ToolEditor["add"]>[0]),
		)
		await ctx.session.hook("context", (event) =>
			compactEvent(event, findApiKey(projectDir)),
		)
	},
}
