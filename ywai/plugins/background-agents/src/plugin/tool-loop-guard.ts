/**
 * Tool loop guard.
 *
 * Detects a session re-issuing the exact same tool call (same tool, same
 * arguments) consecutively with no change, which is how model-side infinite
 * loops present (opencode issue #1071: a sub-agent repeating
 * identical read/grep calls forever).
 *
 * Behavior:
 * - 3 consecutive calls with identical arguments AND identical results:
 *   append corrective text to the tool output telling the model to stop and
 *   change approach.
 * - For read-only file tools (read, grep, glob), 5 consecutive identical
 *   calls refuse the next one in tool.execute.before by throwing, so the
 *   loop terminates instead of running forever.
 * - The run counter only advances in tool.execute.after, when an identical-
 *   args call produced an output byte-identical to the prior call. A call
 *   that returns NEW information resets the run, so it can never accumulate
 *   toward a block (a legitimate re-read after the file changed).
 * - tool.execute.before never increments the counter, so overlapping
 *   parallel calls cannot inflate the count before results are known.
 */

const LOOP_GUARD_WARN_AT = 3
const LOOP_GUARD_BLOCK_AT = 5

/**
 * Tools exempt from the entire guard: task management tools whose repeated
 * invocation is legitimate. ywai's delegation tools are exempt too — polling
 * delegation_status is how the supervisor watches background work.
 */
const LOOP_GUARD_EXEMPT: Record<string, true> = {
	task: true,
	task_cancel: true,
	task_message: true,
	task_revive: true,
	delegate: true,
	delegation_list: true,
	delegation_status: true,
	delegation_read: true,
	subagent: true,
	subagent_status: true,
	subagent_read: true,
	subagent_steer: true,
	subagent_stop: true,
}

/**
 * Tools that may be hard-blocked when repeated. Only read-only file analysis
 * hard-blocks after confirmed identical results. Tools with side effects stay
 * warn-only.
 */
const LOOP_GUARD_BLOCK_TOOLS: Record<string, true> = {
	read: true,
	grep: true,
	glob: true,
	list: true,
}

const LOOP_GUARD_MARKER = "[REPEATED TOOL CALLS - STOP]"

export const LOOP_GUARD_WARNING = `
${LOOP_GUARD_MARKER}

You have issued the exact same tool call with identical arguments ${LOOP_GUARD_WARN_AT} times in a row and received identical results. This is an infinite loop and you are making no progress.

STOP repeating this call. Instead:
1. Reconsider what you are looking for; the result above already contains what this call can tell you.
2. If you need different information, make a DIFFERENT call (different path, pattern, or tool).
3. If the task is actually done, produce your final answer now instead of calling more tools.
`

/** Max sessions tracked before evicting the least-recently-observed session. */
const MAX_TRACKED_SESSIONS = 512

interface BeforeInput {
	tool: string
	sessionID?: string
	callID?: string
}

interface BeforeOutput {
	args?: unknown
}

interface AfterInput {
	tool: string
	sessionID?: string
	callID?: string
}

interface AfterOutput {
	output?: unknown
	title?: string
	metadata?: unknown
}

export interface ToolLoopGuardHooks {
	"tool.execute.before": (input: BeforeInput, output: BeforeOutput) => Promise<void>
	"tool.execute.after": (input: AfterInput, output: AfterOutput) => Promise<void>
	/** Clear per-turn state when a new user message lands. */
	observeNewUserMessage(sessionID: string, messageID: string): void
	/** Clear all state for a finished or deleted session. */
	resetSession(sessionID: string): void
	/** Test seam: wipe all state between cases. */
	resetForTests(): void
}

/** Deterministic fingerprint of tool + args, insensitive to key order. */
export function fingerprint(tool: string, args: unknown): string {
	return `${tool.toLowerCase()}:${stableStringify(args)}`
}

function stableStringify(value: unknown): string {
	if (value === null || typeof value !== "object") {
		return JSON.stringify(value)
	}
	if (Array.isArray(value)) {
		return `[${value.map(stableStringify).join(",")}]`
	}
	const record = value as Record<string, unknown>
	const keys = Object.keys(record).sort()
	return `{${keys.map((k) => `${JSON.stringify(k)}:${stableStringify(record[k])}`).join(",")}}`
}

interface SessionState {
	/** Fingerprint of the most recent completed eligible call (args). */
	last: string
	/** Consecutive completed calls with identical args AND identical output. */
	runs: number
	/** Fingerprint of the most recent completed call's output. */
	lastOutput: string
}

interface CallState {
	sessionID: string
	key: string
}

export function createToolLoopGuardHook(): ToolLoopGuardHooks {
	const sessions = new Map<string, SessionState>()
	/** Per-call state so `after` can re-check without re-deriving args. */
	const callKeys = new Map<string, CallState>()

	/** Prune the session maps to MAX_TRACKED_SESSIONS (FIFO by insertion). */
	function keepSessionsBounded(): void {
		while (sessions.size > MAX_TRACKED_SESSIONS) {
			const oldest = sessions.keys().next().value as string | undefined
			if (oldest === undefined) break
			sessions.delete(oldest)
		}
	}

	return {
		"tool.execute.before": async (input: BeforeInput, output: BeforeOutput): Promise<void> => {
			const sessionID = input.sessionID
			if (!sessionID) return
			const toolName = input.tool.toLowerCase()

			if (LOOP_GUARD_EXEMPT[toolName]) return

			const key = fingerprint(toolName, output.args)

			// Refuse only on a CONFIRMED identical run: the previous BLOCK_AT
			// calls all had identical args AND identical results. The current
			// call's result is not yet known, but the run is already degenerate.
			const existing = sessions.get(sessionID)
			if (existing && existing.last === key && existing.runs >= LOOP_GUARD_BLOCK_AT && LOOP_GUARD_BLOCK_TOOLS[toolName]) {
				throw new Error(
					`Refusing to execute "${input.tool}": this exact call (same tool, same arguments) has returned identical results ${existing.runs} times in a row and constitutes an infinite loop. Stop repeating it. Reassess your goal, make a different call, or produce your final answer.`,
				)
			}

			if (input.callID) callKeys.set(input.callID, { sessionID, key })
		},

		"tool.execute.after": async (input: AfterInput, output: AfterOutput): Promise<void> => {
			const sessionID = input.sessionID
			if (!sessionID) return
			const toolName = input.tool.toLowerCase()
			if (LOOP_GUARD_EXEMPT[toolName]) return

			const call = input.callID ? callKeys.get(input.callID) : undefined
			if (input.callID) callKeys.delete(input.callID)
			// An after hook without a matching before hook is stale. In
			// particular, do not let a late after hook recreate state after
			// session deletion.
			if (!input.callID || !call) return

			const outputHash = fingerprint(toolName, output.output)

			const existing = sessions.get(sessionID)
			let state: SessionState
			if (existing && call.key === existing.last) {
				// Identical args. Advance the run only when the result is also
				// identical; a changed result is progress and restarts the run.
				state = {
					last: call.key,
					runs: outputHash === existing.lastOutput ? existing.runs + 1 : 1,
					lastOutput: outputHash,
				}
			} else {
				// Different args or untracked call: start a fresh run.
				state = {
					last: call.key,
					runs: 1,
					lastOutput: outputHash,
				}
			}
			sessions.set(sessionID, state)
			keepSessionsBounded()

			if (state.runs < LOOP_GUARD_WARN_AT) return
			if (typeof output.output !== "string") return
			if (output.output.includes(LOOP_GUARD_MARKER)) return
			output.output += `\n${LOOP_GUARD_WARNING}`
		},

		observeNewUserMessage(sessionID: string, _messageID: string): void {
			// A fresh user message is a new goal: identical calls to the previous
			// turn are legitimate again.
			sessions.delete(sessionID)
		},

		resetSession(sessionID: string): void {
			sessions.delete(sessionID)
			for (const [callID, call] of callKeys) {
				if (call.sessionID === sessionID) callKeys.delete(callID)
			}
		},

		resetForTests(): void {
			sessions.clear()
			callKeys.clear()
		},
	}
}
