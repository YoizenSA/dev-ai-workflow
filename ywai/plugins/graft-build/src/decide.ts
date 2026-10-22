/**
 * The one case graft's own freshness gate refuses to handle.
 *
 * Every graft retrieval call probes the working tree and rebuilds the
 * structural graph before answering (see graft's graph/refresh.ts), so drift
 * is not our problem — a repo that has been built once stays honest on its
 * own, including inside a git worktree, which it seeds from the parent
 * checkout. The gate stops at exactly one case, and says why in its own
 * source: "Auto-building a whole repo under a query is a surprise, and it's
 * the one case where the user hasn't opted into graft at all yet."
 *
 * Opting in is what installing this plugin means. So this covers that case
 * and nothing else: no graph on disk, build it once, in the background.
 */

import * as fs from "node:fs"
import * as path from "node:path"

/**
 * ponytail: assumes graft's default `<repo>/graft` context dir. `--dir` and
 * GRAFT_DIR override it, and a repo using one just never triggers the build —
 * it falls back to graft's own "run graft build" message, which is where we
 * started. Read the override if someone actually uses one.
 */
export function graftDir(repo: string): string {
	return path.join(repo, "graft")
}

/** The file graft's gate itself probes to decide "graph or no graph". */
export function wiringPath(repo: string): string {
	return path.join(graftDir(repo), ".graph", "wiring.json")
}

/**
 * Marker for a build that failed here. Without it a repo graft cannot index
 * (no supported extensions, a parse crash) pays for a doomed build on every
 * single session open. It lives under graft/.cache, which is already
 * gitignored by graft, so it never reaches a commit.
 */
export function failureMarkerPath(repo: string): string {
	return path.join(graftDir(repo), ".cache", ".ywai-build-failed")
}

function exists(p: string): boolean {
	try {
		return fs.existsSync(p)
	} catch {
		return false
	}
}

/** Graft's own env kill switch, honored here so one variable turns both off. */
function killSwitched(): boolean {
	const v = process.env.GRAFT_NO_REFRESH
	return v !== undefined && v !== "" && v !== "0" && v !== "false"
}

/**
 * True only for a git repository that has never been built. Everything else —
 * a stale graph, a non-repo cwd, a repo we already failed on — is someone
 * else's job or nobody's.
 */
export function needsInitialBuild(repo: string): boolean {
	if (killSwitched()) return false
	if (!exists(path.join(repo, ".git"))) return false
	if (exists(wiringPath(repo))) return false
	if (exists(failureMarkerPath(repo))) return false
	return true
}
