/**
 * Unified-diff parsing (PLAN 10, Fase 1).
 *
 * Only what locate needs: the added-line regions of a patch, numbered against
 * the NEW file, because a finding has to point at a line that still exists.
 */
import type { Hunk } from "./types"

const HEADER = /^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@/

/**
 * Split a patch into hunks, numbered in the new file.
 *
 * A hunk with no added lines (a pure deletion) still counts: removing an auth
 * check is the canonical finding, and it has no added line to point at. Its
 * range collapses to the line where the deletion happened.
 */
export function parseHunks(patch: string): Hunk[] {
	const hunks: Hunk[] = []
	let current: { newLine: number; lines: string[]; first?: number; last?: number } | null = null

	const flush = () => {
		if (!current) return
		const start = current.first ?? current.newLine
		hunks.push({
			startLine: start,
			endLine: current.last ?? start,
			text: current.lines.join("\n"),
		})
		current = null
	}

	for (const line of patch.split(/\r?\n/)) {
		const header = HEADER.exec(line)
		if (header) {
			flush()
			current = { newLine: Number(header[1]), lines: [line] }
			continue
		}
		if (!current) continue
		if (line.startsWith("diff ") || line.startsWith("--- ") || line.startsWith("+++ ")) {
			flush()
			continue
		}

		current.lines.push(line)
		if (line.startsWith("+")) {
			current.first ??= current.newLine
			current.last = current.newLine
			current.newLine++
		} else if (line.startsWith("-")) {
			// Deleted lines do not advance the new-file counter, but they are
			// where a removed check lived: anchor to the current position.
			current.first ??= current.newLine
			current.last ??= current.newLine
		} else if (!line.startsWith("\\")) {
			current.newLine++
		}
	}
	flush()
	return hunks
}

/** Added lines only, for questions that should not re-read untouched code. */
export function addedLines(patch: string): string[] {
	return patch
		.split(/\r?\n/)
		.filter((line) => line.startsWith("+") && !line.startsWith("+++"))
		.map((line) => line.slice(1))
}
