/**
 * Turning files into segments to ask about (PLAN 3.2 / 4.3).
 *
 * Windows of 60 lines with 10 of overlap, trimmed to MAX_SEGMENT_CHARS. The
 * overlap exists so a function that straddles a boundary is still whole in one
 * of the two windows; without it the answer depends on where the window fell.
 */
export const SEGMENT_LINES = 60
export const SEGMENT_OVERLAP = 10
export const MAX_SEGMENT_CHARS = 4000
export const MAX_FIND_SEGMENTS = 200

export interface Segment {
	file: string
	startLine: number
	endLine: number
	text: string
}

/** Split one file into overlapping windows, numbered from 1. */
export function segmentFile(file: string, content: string): Segment[] {
	const lines = content.split(/\r?\n/)
	if (lines.length === 0) return []

	const segments: Segment[] = []
	const stride = Math.max(1, SEGMENT_LINES - SEGMENT_OVERLAP)
	for (let start = 0; start < lines.length; start += stride) {
		const slice = lines.slice(start, start + SEGMENT_LINES)
		const text = slice.join("\n")
		if (!text.trim()) continue
		segments.push({
			file,
			startLine: start + 1,
			endLine: start + slice.length,
			text: text.length > MAX_SEGMENT_CHARS ? text.slice(0, MAX_SEGMENT_CHARS) : text,
		})
		// The last window already reached the end; another stride would only
		// re-ask about the same tail.
		if (start + SEGMENT_LINES >= lines.length) break
	}
	return segments
}

/**
 * A snippet short enough to show, anchored on the best line we can name.
 * Find returns evidence, so the caller can check it without opening the file.
 */
export function snippetOf(segment: Segment, maxLines = 6): string {
	return segment.text.split("\n").slice(0, maxLines).join("\n")
}
