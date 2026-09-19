/**
 * graft-build — build the graft graph once, for a repo that has never had one.
 *
 * See ./decide.ts for why this exists and why it does nothing else.
 */

import { spawn } from "node:child_process"
import * as fs from "node:fs"
import * as path from "node:path"

import { failureMarkerPath, needsInitialBuild } from "./decide.js"

/**
 * Process-lifetime guard. A host may load the plugin once per session while
 * several sessions share one process; the build takes minutes on a large repo
 * and `graft build` takes its own lock, so a second spawn would only sit
 * there. Keyed by repo because one process does serve several directories.
 */
const started = new Set<string>()

function markFailed(repo: string): void {
	try {
		const marker = failureMarkerPath(repo)
		fs.mkdirSync(path.dirname(marker), { recursive: true })
		fs.writeFileSync(marker, new Date().toISOString())
	} catch {
		// A marker we cannot write costs a retry next open, nothing worse.
	}
}

/**
 * Spawn the build detached. Never awaited: the graph is not needed to start a
 * session, and a first build on a large repo is minutes long — blocking setup
 * on it would hold the session open behind indexing.
 *
 * Tier-1 only. No `--deep`, which is the LLM pass: the same money guard graft
 * puts on its own automatic path. Nothing automatic gets to spend.
 */
export function startInitialBuild(repo: string): boolean {
	if (!needsInitialBuild(repo) || started.has(repo)) return false
	started.add(repo)
	try {
		const child = spawn("graft", ["build", repo], {
			cwd: repo,
			stdio: "ignore",
			detached: true,
		})
		// ENOENT (graft not installed) arrives as an event, not a throw.
		child.on("error", () => markFailed(repo))
		child.on("exit", (code) => {
			if (code !== 0) markFailed(repo)
		})
		child.unref()
		return true
	} catch {
		markFailed(repo)
		return false
	}
}

async function setup(ctx: { location?: { directory?: string } }): Promise<void> {
	startInitialBuild(ctx?.location?.directory ?? process.cwd())
}

/**
 * v2 validates the default export against an object schema, so a callable
 * export is rejected outright and the plugin never loads. The object form
 * serves both flavors: v2 reads id/setup, v1 calls server().
 */
export default {
	id: "ywai-graft-build",
	setup,
	async server(ctx: { directory?: string }) {
		startInitialBuild(ctx?.directory ?? process.cwd())
		return {}
	},
}
