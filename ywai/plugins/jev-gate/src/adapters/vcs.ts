/**
 * Where the diff comes from (PLAN 3).
 *
 * `ctx.vcs.diff({ mode })` is the host's own view of the working tree and is
 * preferred; `adapters/git.ts` is the fallback for scripts, tests, and any
 * host version whose shape does not match. Both produce the same
 * `ChangedFile[]`, so the pipeline never learns which one answered.
 */
import { splitDiff, workingDiff } from "./git"
import type { ChangedFile } from "../domain/types"

export interface VcsLike {
	diff?(input: { mode: string }): Promise<unknown>
	status?(): Promise<unknown>
}

/**
 * Modes tried in order. `mode` is required and validated against `Vcs.Mode`:
 * on v2.0.6 only "working" is accepted - "all", "staged", "unstaged" and
 * "head" all fail with `SchemaError(Expected Vcs.Mode)`. The list stays
 * ordered so a host that adds modes later picks the widest one it accepts.
 */
const MODES = ["all", "working"]

/**
 * Pull changed files out of whatever envelope the host wrapped them in.
 *
 * v2.0.6 answers `{ location, data: [{ file, patch }] }`, which already is the
 * per-file split; a plain diff string is also accepted so a different host
 * shape still works.
 */
export function extractChangedFiles(result: unknown): ChangedFile[] {
	if (typeof result === "string") return splitDiff(result)
	if (!result || typeof result !== "object") return []
	const data = (result as { data?: unknown }).data
	if (typeof data === "string") return splitDiff(data)
	if (!Array.isArray(data)) return []

	const files: ChangedFile[] = []
	for (const entry of data) {
		if (typeof entry === "string") {
			files.push(...splitDiff(entry))
			continue
		}
		const record = entry as { file?: string; patch?: string; diff?: string }
		const patch = record.patch ?? record.diff
		if (!patch) continue
		if (record.file) files.push({ path: record.file, patch })
		else files.push(...splitDiff(patch))
	}
	return files
}

/**
 * The working-tree diff, host first and git second.
 *
 * Returns the source alongside the files so a report can say where its input
 * came from - a review of the wrong tree is worse than no review.
 */
export async function collectChangedFiles(
	vcs: VcsLike | undefined,
	cwd: string,
	base = "HEAD",
): Promise<{ files: ChangedFile[]; source: "vcs" | "git" }> {
	for (const mode of MODES) {
		try {
			const files = extractChangedFiles(await vcs?.diff?.({ mode }))
			if (files.length > 0) return { files, source: "vcs" }
		} catch {
			// Wrong mode for this host version; try the next, then git.
		}
	}
	return { files: await workingDiff(cwd, base), source: "git" }
}
