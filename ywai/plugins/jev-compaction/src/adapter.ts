/**
 * Maps OpenCode v2 messages to fast-jev-compaction messages and back.
 *
 * The way back does not convert the library's output. It diffs input and output
 * into a per-tool-id plan and applies that plan to the original v2 messages, so
 * nothing outside tool-call / tool-result parts can change.
 */
import type { Message as LibMessage } from "../vendor/fast-jev-compaction/types"

export interface V2Message {
	id?: string
	role: string
	content: Array<Record<string, any>>
	[key: string]: any
}

export type PlanEntry = { action: "drop" } | { action: "truncate"; text: string }
export type Plan = Map<string, PlanEntry>

function resultText(result: { type: string; value: unknown } | undefined): string {
	if (!result) return ""
	if (typeof result.value === "string") return result.value
	if (result.type === "content" && Array.isArray(result.value)) {
		return result.value
			.map((part: any) => (typeof part?.text === "string" ? part.text : ""))
			.join("\n")
	}
	return JSON.stringify(result.value) ?? ""
}

/** System messages are skipped; tool-role messages become user messages, as in Anthropic. */
export function toLib(messages: readonly V2Message[]): LibMessage[] {
	const out: LibMessage[] = []
	for (const message of messages) {
		if (message.role === "system") continue
		const lib: LibMessage = {
			role: message.role === "assistant" ? "assistant" : "user",
			text: message.content
				.filter((part) => part.type === "text")
				.map((part) => part.text)
				.join("\n"),
			toolUses: message.content
				.filter((part) => part.type === "tool-call")
				.map((part) => ({ tool_use_id: part.id, tool: part.name, input: part.input ?? {} })),
		}
		const results = message.content.filter((part) => part.type === "tool-result")
		if (results.length > 0) {
			lib.toolResults = results.map((part) => ({
				tool_use_id: part.id,
				text: resultText(part.result),
				isError: part.result?.type === "error",
			}))
		}
		out.push(lib)
	}
	return out
}

/** Ids whose call disappeared are dropped; ids whose result text changed are truncated. */
export function planFrom(before: readonly LibMessage[], after: readonly LibMessage[]): Plan {
	const keptCalls = new Set(after.flatMap((m) => m.toolUses.map((t) => t.tool_use_id)))
	const resultsAfter = new Map(
		after.flatMap((m) => (m.toolResults ?? []).map((r) => [r.tool_use_id, r.text] as const)),
	)
	const plan: Plan = new Map()
	for (const message of before) {
		for (const call of message.toolUses) {
			if (!keptCalls.has(call.tool_use_id)) plan.set(call.tool_use_id, { action: "drop" })
		}
	}
	for (const message of before) {
		for (const result of message.toolResults ?? []) {
			if (plan.has(result.tool_use_id)) continue
			const text = resultsAfter.get(result.tool_use_id)
			if (text !== undefined && text !== result.text) {
				plan.set(result.tool_use_id, { action: "truncate", text })
			}
		}
	}
	return plan
}

/** Keeps the class of each rebuilt message; v2 messages are Schema.Class instances. */
function rebuild(message: V2Message, content: V2Message["content"]): V2Message {
	return Object.assign(Object.create(Object.getPrototypeOf(message)), message, { content })
}

export function applyPlan(messages: readonly V2Message[], plan: Plan): V2Message[] {
	const out: V2Message[] = []
	for (const message of messages) {
		let changed = false
		const content: V2Message["content"] = []
		for (const part of message.content) {
			const isTool = part.type === "tool-call" || part.type === "tool-result"
			const entry = isTool ? plan.get(part.id) : undefined
			if (!entry) {
				content.push(part)
				continue
			}
			changed = true
			if (entry.action === "drop") continue
			content.push(
				part.type === "tool-result" ? { ...part, result: { type: "text", value: entry.text } } : part,
			)
		}
		if (!changed) out.push(message)
		else if (content.length > 0) out.push(rebuild(message, content))
	}
	return out
}
