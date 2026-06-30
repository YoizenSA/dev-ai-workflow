import { create } from 'zustand'

// commentaryStore holds the live "narration" feed for a workflow run. This feed
// is parsed from the orchestrator's own stdout/stderr stream (no second LLM):
// each output line is classified into an entry type so the panel reads like a
// running commentary.
//
// The feed is in-memory only (not persisted) — it represents the current run.

export type CommentaryEventType = 'assistant' | 'tool_use' | 'error' | 'summary'

export interface CommentaryEntry {
	id: string
	text: string
	timestamp: string
	eventType: CommentaryEventType
}

interface CommentaryState {
	entries: CommentaryEntry[]
	processing: boolean
	runId: string | null

	// startRun resets the feed for a new run.
	startRun: (runId: string) => void
	// pushLine classifies one output line and appends it as an entry.
	pushLine: (stream: 'stdout' | 'stderr', text: string) => void
	// markDone stops the "processing" indicator.
	markDone: () => void
	// clear empties the feed.
	clear: () => void
}

let seq = 0
function nextId(): string {
	seq += 1
	return `c${seq}`
}

// classifyLine guesses the entry type from a raw output line. The orchestrator's
// output isn't structured, so we look for tell-tale prefixes. Unknown lines
// default to "assistant" (the model's own narration/text).
function classifyLine(stream: 'stdout' | 'stderr', text: string): CommentaryEventType {
	const t = text.trim()
	if (stream === 'stderr' || /^(error|fail|exception|panic)/i.test(t)) {
		return 'error'
	}
	// Tool-use markers common in opencode/agent output.
	if (/^(tool|task|delegate|call|executing|running)/i.test(t)) {
		return 'tool_use'
	}
	if (/^(done|complete|finished|summary|result)/i.test(t)) {
		return 'summary'
	}
	return 'assistant'
}

export const useCommentaryStore = create<CommentaryState>((set) => ({
	entries: [],
	processing: false,
	runId: null,

	startRun: (runId) =>
		set({ entries: [], processing: true, runId }),

	pushLine: (stream, text) => {
		const trimmed = text.trim()
		if (!trimmed) return
		const entry: CommentaryEntry = {
			id: nextId(),
			text: trimmed,
			timestamp: new Date().toISOString(),
			eventType: classifyLine(stream, text),
		}
		set((s) => ({ entries: [...s.entries, entry] }))
	},

	markDone: () => set({ processing: false }),

	clear: () => set({ entries: [], processing: false, runId: null }),
}))
