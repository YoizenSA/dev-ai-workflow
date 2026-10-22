/**
 * git fallback for tests and scripts (PLAN 2 / 3).
 *
 * In the plugin, the diff comes from `ctx.vcs.diff()`. This exists so the core
 * library can run from a CLI or an eval script without OpenCode, and so Fase 6
 * can replay a real repo. It shells out to git and parses nothing it does not
 * have to.
 */
import { execFile } from "node:child_process"
import { promisify } from "node:util"
import type { ChangedFile } from "../domain/types"

const run = promisify(execFile)

async function git(args: string[], cwd: string): Promise<string> {
	const { stdout } = await run("git", args, { cwd, maxBuffer: 32 * 1024 * 1024 })
	return stdout
}

/**
 * Split `git diff` output into one patch per file.
 *
 * Paths come from the `+++ b/...` line, falling back to the `diff --git`
 * header for deletions, where `+++` is `/dev/null`.
 */
export function splitDiff(diff: string): ChangedFile[] {
	const files: ChangedFile[] = []
	const chunks = diff.split(/^diff --git /m).slice(1)
	for (const chunk of chunks) {
		const body = `diff --git ${chunk}`
		const plus = /^\+\+\+ b\/(.+)$/m.exec(body)?.[1]
		const header = /^diff --git a\/(.+?) b\/(.+)$/m.exec(body)
		const path = plus && plus !== "/dev/null" ? plus : (header?.[2] ?? header?.[1])
		if (!path) continue
		files.push({ path: path.trim(), patch: body })
	}
	return files
}

/**
 * The working-tree diff against `base` (default: HEAD), staged changes
 * included. Untracked files are out of scope: they have no diff to review.
 */
export async function workingDiff(cwd: string, base = "HEAD"): Promise<ChangedFile[]> {
	const diff = await git(["diff", "--no-color", "--unified=3", base], cwd)
	return splitDiff(diff)
}

export async function isGitRepo(cwd: string): Promise<boolean> {
	try {
		await git(["rev-parse", "--git-dir"], cwd)
		return true
	} catch {
		return false
	}
}
