/**
 * Live transcript accumulator, fed by v2 session events.
 *
 * The v2 plugin Context has no message-listing API, so peek/status read from
 * this in-memory view built from `session.next.text.delta` and
 * `session.next.tool.*` events. Text is kept per assistant message (usually
 * the final one is the delegation's answer); tool activity is a capped ring.
 */

interface ToolEvent {
	at: number
	tool: string
	phase: "called" | "success" | "failed"
}

const MAX_TEXT_CHARS = 200_000
const MAX_TOOL_EVENTS = 200

export class Transcript {
	private readonly texts = new Map<string, string>()
	private readonly textOrder: string[] = []
	private readonly toolEvents: ToolEvent[] = []
	private lastActivityAt = 0

	/** A text delta for an assistant message. */
	appendText(assistantMessageID: string, delta: string): void {
		const existing = this.texts.get(assistantMessageID)
		const next = (existing ?? "") + delta
		if (next.length > MAX_TEXT_CHARS) {
			this.texts.set(assistantMessageID, next.slice(-MAX_TEXT_CHARS))
		} else {
			this.texts.set(assistantMessageID, next)
		}
		if (!existing) this.textOrder.push(assistantMessageID)
		this.touch()
	}

	recordTool(tool: string, phase: ToolEvent["phase"]): void {
		this.toolEvents.push({ at: Date.now(), tool, phase })
		if (this.toolEvents.length > MAX_TOOL_EVENTS) {
			this.toolEvents.splice(0, this.toolEvents.length - MAX_TOOL_EVENTS)
		}
		this.touch()
	}

	touch(): void {
		this.lastActivityAt = Date.now()
	}

	get lastActivity(): number {
		return this.lastActivityAt
	}

	/** The delegation's answer: assistant text, latest message first. */
	resultText(): string {
		for (let i = this.textOrder.length - 1; i >= 0; i--) {
			const text = this.texts.get(this.textOrder[i])
			if (text && text.trim()) return text.trim()
		}
		return ""
	}

	/** All assistant text in order (multi-message turns). */
	fullText(): string {
		return this.textOrder
			.map((id) => this.texts.get(id) ?? "")
			.filter((t) => t.trim())
			.join("\n\n")
			.trim()
	}

	/** Digest for delegation_peek: recent text tail + tool activity. */
	peekDigest(maxChars = 3_500): string {
		const lines: string[] = []
		const text = this.fullText()
		if (text) {
			lines.push("### Latest output")
			lines.push(text.length > maxChars ? `…${text.slice(-maxChars)}` : text)
		} else {
			lines.push("_(no assistant text yet)_")
		}
		if (this.toolEvents.length > 0) {
			const recent = this.toolEvents.slice(-12)
			lines.push("", "### Recent tool activity")
			for (const ev of recent.reverse()) {
				const icon = ev.phase === "success" ? "✓" : ev.phase === "failed" ? "✗" : "→"
				lines.push(`- ${icon} \`${ev.tool}\` (${new Date(ev.at).toISOString().slice(11, 19)})`)
			}
			const total = this.toolEvents.length
			if (total > 12) lines.push(`- … ${total - 12} earlier tool events`)
		}
		return lines.join("\n")
	}

	toolCallCount(): number {
		return this.toolEvents.filter((e) => e.phase === "called").length
	}
}
