/**
 * jev-gate - Jev as a policy layer over OpenCode 2 (PLAN Fase 2).
 *
 * Ships review, find, route, and jev_check_page (Gherkin Then vs snapshot).
 *
 * Both return a short markdown summary; the full report goes to `ctx.storage`
 * under its runId, so the model never receives 40 KB of JSON.
 *
 * Fail closed on purpose: with no TYPESAFE_API_KEY the tools refuse rather
 * than degrade, because "a Jev review" that Jev never ran is the one output
 * this plugin must never produce (PLAN 0 / 3.1).
 *
 * The Fase 0 probes stay available behind JEV_GATE_PROBE=1 - Code Mode means
 * the schema-size question needs re-measuring on every host version.
 *
 * Authoring notes from Spike B (v2.0.6):
 * - `process.cwd()` is the user's HOME at plugin load, not the project, so
 *   every path comes from `ctx.location.directory`.
 * - Plugin tools reach the model only through Code Mode's `execute`, so the
 *   descriptions below are written for someone reading a catalog entry.
 */
import { appendFileSync, readFileSync, readdirSync, statSync } from "node:fs"
import { homedir } from "node:os"
import { join, relative, resolve } from "node:path"
import type { V2PluginContext, V2ToolEditor } from "../../shared/v2"
import { collectChangedFiles, type VcsLike } from "./adapters/vcs"
import {
	loadDecision,
	loadRoute,
	loadRun,
	saveDecision,
	saveRoute,
	saveRun,
	type StorageLike,
} from "./adapters/store"
import { registerPermissionGate, type PermissionHost } from "./gates/permission"
import { routeTask } from "./route/decide"
import {
	CONFIRM_FILES_CODEBASE,
	MAX_FILES_CODEBASE,
	isDenied,
	isReviewable,
	isTestFile,
} from "./domain/config"
import type { ChangedFile } from "./domain/types"
import { findApiKey, keyHelp } from "./adapters/key"
import { HttpJevClient, MissingKeyError } from "./jev/client"
import { runChangeReview } from "./review/workflow"
import { checkPage, summarizePageCheck } from "./review/page"
import { detail, summarize } from "./format"
import { findInSegments, FIND_THRESHOLD } from "./find/score"
import { segmentFile, MAX_FIND_SEGMENTS, type Segment } from "./find/segment"

interface JevContext extends V2PluginContext {
	vcs?: VcsLike
	storage?: StorageLike
	permission?: PermissionHost
}

/** Deny globs and every reported path are POSIX-shaped, Windows included. */
function toPosix(path: string): string {
	return path.split("\\").join("/")
}

function projectDir(ctx: JevContext): string {
	return ctx.location?.directory ?? process.cwd()
}

/**
 * One place to turn any failure into a message the agent can repeat safely.
 * Every branch says nothing was reviewed, because the failure mode that costs
 * the most is an agent reporting silence as a clean review.
 */
function failure(err: unknown): string {
	const reason = err instanceof MissingKeyError ? err.message : String(err)
	return (
		`**Jev review did not run.** ${reason}\n\n` +
		"Nothing was reviewed. Do not describe this as a passing or clean review."
	)
}

/** Walk a path into reviewable files, respecting the deny list and the caps. */
export function collectPathFiles(root: string, target: string): ChangedFile[] {
	const absolute = resolve(root, target)
	const files: ChangedFile[] = []

	const visit = (current: string) => {
		if (files.length >= MAX_FILES_CODEBASE) return
		const info = statSync(current)
		if (info.isDirectory()) {
			for (const entry of readdirSync(current)) {
				if (entry === "node_modules" || entry === ".git" || entry === "dist") continue
				visit(join(current, entry))
			}
			return
		}
		const path = relative(root, current).replace(/\\/g, "/")
		if (!isReviewable(path) || isDenied(path) || isTestFile(path)) return
		// The pipeline screens patches, so present the file as an all-added
		// patch: same shape, and here every line genuinely is under review.
		const lines = readFileSync(current, "utf8").split(/\r?\n/)
		files.push({
			path,
			patch: [`@@ -0,0 +1,${lines.length} @@`, ...lines.map((line) => `+${line}`)].join("\n"),
		})
	}

	visit(absolute)
	return files
}

/**
 * Files Find is allowed to read: source only, deny list applied, docs out.
 * Find answering with a README is the failure the plan called out by name.
 */
export function collectSearchFiles(root: string, target = "."): Array<{ path: string; content: string }> {
	const absolute = resolve(root, target)
	const files: Array<{ path: string; content: string }> = []

	const visit = (current: string) => {
		const info = statSync(current)
		if (info.isDirectory()) {
			// Sorted: the segment cap makes walk order decide what gets searched
			// at all, and readdirSync order is not guaranteed stable.
			for (const entry of readdirSync(current).sort()) {
				if (entry === "node_modules" || entry === ".git" || entry === "dist") continue
				visit(join(current, entry))
			}
			return
		}
		const path = toPosix(relative(root, current))
		if (!isReviewable(path) || isDenied(path)) return
		files.push({ path, content: readFileSync(current, "utf8") })
	}

	visit(absolute)
	return files
}

async function reviewAndFormat(
	ctx: JevContext,
	files: ChangedFile[],
	sessionID: string | undefined,
): Promise<string> {
	const apiKey = findApiKey(projectDir(ctx))
	if (!apiKey) throw new MissingKeyError(keyHelp(projectDir(ctx)))
	const report = await runChangeReview(new HttpJevClient({ apiKey }), files)
	await saveRun(ctx.storage, report)
	if (sessionID) await saveDecision(ctx.storage, sessionID, report)
	return summarize(report)
}

/**
 * Record why setup failed, where a user can find it.
 *
 * A plugin whose setup() rejects still logs "loading plugin" and then
 * registers nothing, so the tools are simply absent with no explanation - the
 * exact failure this plugin hit in the field. The Fase 0 probes wrapped every
 * registration for this reason; production setup has to do the same.
 */
function recordSetupFailure(step: string, err: unknown) {
	const line = `[jev-gate] ${new Date().toISOString()} ${step} failed: ${String(err)}`
	console.error(line)
	try {
		appendFileSync(join(homedir(), ".jev-gate-setup.log"), line + "\n")
	} catch {
		// The console line is the fallback; never fail load over a log write.
	}
}

function toolSurface(ctx: JevContext): string {
	const tool = ctx.tool as { transform?: unknown; add?: unknown; hook?: unknown } | undefined
	return JSON.stringify({
		typeofTool: typeof tool,
		keys: tool && typeof tool === "object" ? Object.keys(tool) : [],
		typeofTransform: typeof tool?.transform,
		typeofAdd: typeof tool?.add,
		typeofHook: typeof tool?.hook,
	})
}

function withCodeMode(def: Parameters<V2ToolEditor["add"]>[0]) {
	return { ...def, options: { ...(def as { options?: object }).options, codemode: true } }
}

async function setup(ctx: JevContext) {
	recordSetupFailure("tool-surface", toolSurface(ctx))
	if (typeof ctx.tool?.transform !== "function") {
		recordSetupFailure(
			"tool-registration",
			"ctx.tool.transform is not a function on this OpenCode version; no jev_* tools were registered",
		)
		return
	}
	await ctx.tool.transform((editor: V2ToolEditor) => {
		const add = (def: Parameters<V2ToolEditor["add"]>[0]) => editor.add(withCodeMode(def))
		add({
			name: "jev_review_diff",
			description:
				"Review the current working-tree diff with Jev, a classifier that answers with " +
				"probabilities rather than prose. Returns Jev's own findings with file:line, " +
				"mechanism and severity. Report them as written: never add findings of your own " +
				"and attribute them to Jev, and never state one more confidently than its score.",
			input: {
				type: "object",
				properties: {
					base: { type: "string", description: "Git ref to diff against. Defaults to HEAD." },
				},
			},
			execute: async (input: { base?: string }, toolCtx?: { sessionID?: string }) => {
				try {
					const root = projectDir(ctx)
					const { files, source } = await collectChangedFiles(ctx.vcs, root, input?.base ?? "HEAD")
					if (files.length === 0) {
						return { content: "No changed files in the working tree. Nothing to review." }
					}
					const summary = await reviewAndFormat(ctx, files, toolCtx?.sessionID)
					return { content: `${summary}\n\n_diff source: ${source}_` }
				} catch (err) {
					return { content: failure(err) }
				}
			},
		})

		add({
			name: "jev_review_path",
			description:
				`Review one file or directory with Jev, the same pipeline as jev_review_diff. ` +
				`Asks for confirmation over ${CONFIRM_FILES_CODEBASE} files and never reviews more ` +
				`than ${MAX_FILES_CODEBASE}.`,
			input: {
				type: "object",
				properties: {
					path: { type: "string", description: "File or directory, relative to the project." },
					confirm: {
						type: "boolean",
						description: "Required to review more than the confirmation threshold.",
					},
				},
				required: ["path"],
			},
			execute: async (
				input: { path: string; confirm?: boolean },
				toolCtx?: { sessionID?: string },
			) => {
				try {
					const files = collectPathFiles(projectDir(ctx), input.path)
					if (files.length === 0) {
						return { content: `No reviewable JS/TS files under \`${input.path}\`.` }
					}
					if (files.length > CONFIRM_FILES_CODEBASE && !input.confirm) {
						return {
							content:
								`\`${input.path}\` has ${files.length} reviewable files, over the ` +
								`${CONFIRM_FILES_CODEBASE} confirmation threshold. Nothing was sent to Jev. ` +
								"Re-run with confirm: true to proceed.",
						}
					}
					return { content: await reviewAndFormat(ctx, files, toolCtx?.sessionID) }
				} catch (err) {
					return { content: failure(err) }
				}
			},
		})

		add({
			name: "jev_find",
			description:
				"Semantic grep: find where something is actually done in the codebase, not where " +
				"it is merely named or mentioned. Returns file:line ranges with a snippet and a " +
				"probability. Slower and costlier than grep - use it when the words in the code " +
				`are not the words in the question. Searches at most ${MAX_FIND_SEGMENTS} segments ` +
				"in directory order, so on a large repo it covers only part of it unless you " +
				"narrow the search with `root`; the result always says how much it covered.",
			input: {
				type: "object",
				properties: {
					query: { type: "string", description: "What to look for, in plain words." },
					root: { type: "string", description: "Directory to search. Defaults to the project." },
					threshold: { type: "number", description: `Minimum probability. Defaults to ${FIND_THRESHOLD}.` },
					limit: { type: "number", description: "Maximum hits to return. Defaults to 20." },
				},
				required: ["query"],
			},
			execute: async (input: { query: string; root?: string; threshold?: number; limit?: number }) => {
				try {
					const apiKey = findApiKey(projectDir(ctx))
					if (!apiKey) throw new MissingKeyError(keyHelp(projectDir(ctx)))
					const files = collectSearchFiles(projectDir(ctx), input.root ?? ".")
					const segments: Segment[] = []
					for (const file of files) segments.push(...segmentFile(file.path, file.content))
					if (segments.length === 0) {
						return { content: `Nothing searchable under \`${input.root ?? "."}\`.` }
					}

					const result = await findInSegments(new HttpJevClient({ apiKey }), input.query, segments, {
						threshold: input.threshold,
						limit: input.limit,
					})

					const threshold = input.threshold ?? FIND_THRESHOLD
					if (result.hits.length === 0) {
						if (result.truncated) {
							const pct = Math.round((result.scannedSegments / result.totalSegments) * 100)
							return {
								content:
									`**Search incomplete - this is not a negative result.** Nothing scored at or ` +
									`over ${threshold} for "${input.query}", but only the first ` +
									`${result.scannedSegments} of ${result.totalSegments} segments were sent ` +
									`(${pct}% of the corpus, in directory order - the rest was never searched).\n\n` +
									`Narrow the search with \`root\` and run it again before concluding anything.`,
							}
						}
						return {
							content:
								`No segment scored at or over ${threshold} for "${input.query}" across all ` +
								`${result.scannedSegments} segment(s) of \`${input.root ?? "."}\`. ` +
								`The whole corpus was searched, but a miss is still not proof it does not exist.`,
						}
					}

					const lines = result.hits.map((hit) =>
						[
							`- \`${hit.file}:${hit.startLine}-${hit.endLine}\` (${Math.round(hit.probability * 100)}%)`,
							"```",
							hit.snippet,
							"```",
						].join("\n"),
					)
					const truncated = result.truncated
						? `\n\n_Partial: searched the first ${result.scannedSegments} of ` +
							`${result.totalSegments} segments, in directory order. Anything past that ` +
							`was never sent - narrow with \`root\` to cover it._`
						: ""
					return {
						content:
							`**Jev find: "${input.query}"** (${result.hits.length} hit(s), ` +
							`${result.scannedSegments} segments, ${(result.latencyMs / 1000).toFixed(1)}s, ` +
							`${result.usage.requests} requests)\n\n${lines.join("\n\n")}${truncated}`,
					}
				} catch (err) {
					return { content: failure(err) }
				}
			},
		})

		add({
			name: "jev_report",
			description:
				"Show the scores behind a Jev review: every screened file against every dimension, " +
				"with its probability, plus which signals opened a finding and which could not be " +
				"placed. Use it when the summary's verdict is not enough - especially after an " +
				"INCONCLUSIVE run, where the scores are the only record of what Jev saw. " +
				"Defaults to this session's last review.",
			input: {
				type: "object",
				properties: {
					runId: {
						type: "string",
						description: "A runId from an earlier summary. Defaults to this session's last review.",
					},
				},
			},
			execute: async (input: { runId?: string }, toolCtx?: { sessionID?: string }) => {
				try {
					let runId = input?.runId
					if (!runId) {
						const decision = toolCtx?.sessionID
							? await loadDecision(ctx.storage, toolCtx.sessionID)
							: undefined
						runId = decision?.runId
					}
					if (!runId) {
						return {
							content:
								"No runId given and no review recorded for this session. " +
								"Run `jev_review_diff` or `jev_review_path` first.",
						}
					}
					const report = await loadRun(ctx.storage, runId)
					if (!report) {
						// Storage is per-session in practice, so an old runId from
						// another session reads as missing, not as an empty run.
						return {
							content:
								`No stored run for \`${runId}\`. Reports do not outlive the session ` +
								"that produced them, so re-run the review instead of reporting nothing.",
						}
					}
					return { content: detail(report) }
				} catch (err) {
					return { content: failure(err) }
				}
			},
		})

		add({
			name: "jev_route",
			description:
				"Ask Jev who should execute a task: an installed agent, an inline answer, a Jev " +
				"review, or a person. Records the decision for this session, which makes later " +
				"writes ask for confirmation when the task was not routed to a writing agent.",
			input: {
				type: "object",
				properties: {
					task: { type: "string", description: "The task, in the user's own words." },
					wantsWrite: { type: "boolean", description: "The task explicitly asks to change files." },
					failingTests: { type: "boolean", description: "There are failing tests right now." },
				},
				required: ["task"],
			},
			execute: async (
				input: { task: string; wantsWrite?: boolean; failingTests?: boolean },
				toolCtx?: { sessionID?: string },
			) => {
				try {
					const apiKey = findApiKey(projectDir(ctx))
					if (!apiKey) throw new MissingKeyError(keyHelp(projectDir(ctx)))
					// Route over the agents this install actually has, not the
					// stock OpenCode roster: a ywai session runs as `orchestrator`.
					let agents: Array<{ name?: string; description?: string }> = []
					try {
						const listed = await ctx.agent?.list?.()
						if (Array.isArray(listed)) agents = listed
					} catch {
						// Fall back to the known roster inside routeTask.
					}
					const decision = await routeTask(new HttpJevClient({ apiKey }), { ...input, agents })
					if (toolCtx?.sessionID) {
						await saveRoute(ctx.storage, toolCtx.sessionID, {
							choice: decision.choice,
							closeCall: decision.closeCall,
							source: decision.source,
						})
					}
					const reasons = decision.reasonCodes.length
						? ` - ${decision.reasonCodes.join(", ")}`
						: ""
					const agentLine = decision.agent ? ` (agent: \`${decision.agent}\`)` : ""
					return {
						content:
							`**Jev route: \`${decision.choice}\`**${agentLine}${reasons}

` +
							(decision.closeCall
								? "This is a close call. Confirm with the user before acting on it."
								: "Switching agent is the caller's decision; Jev only recommends."),
					}
				} catch (err) {
					return { content: failure(err) }
				}
			},
		})

		add({
			name: "jev_check_page",
			description:
				"Score a Gherkin Then against an accessibility snapshot with Jev. " +
				"Playwright or chrome-devtools describes the page; Jev answers with a " +
				"probability. Jev never clicks. Returns PASS or FAIL with the probability. " +
				"Do not restate the result as more certain than that number, and do not " +
				"treat a missing key or a failed call as PASS.",
			input: {
				type: "object",
				properties: {
					snapshot: {
						type: "string",
						description: "Accessibility tree of the page (aria snapshot or take_snapshot).",
					},
					then: {
						type: "string",
						description: "The Gherkin Then, verbatim.",
					},
					url: { type: "string", description: "Page URL, if known." },
					title: { type: "string", description: "Page title, if known." },
				},
				required: ["snapshot", "then"],
			},
			execute: async (input: { snapshot: string; then: string; url?: string; title?: string }) => {
				try {
					if (!input?.snapshot?.trim() || !input?.then?.trim()) {
						return { content: "snapshot and then are required. Nothing was sent to Jev." }
					}
					const apiKey = findApiKey(projectDir(ctx))
					if (!apiKey) throw new MissingKeyError(keyHelp(projectDir(ctx)))
					const result = await checkPage(new HttpJevClient({ apiKey }), input)
					return { content: summarizePageCheck(result) }
				} catch (err) {
					return { content: failure(err) }
				}
			},
		})
	})

	// The gate reads what the tools above stored. It can only harden.
	await registerPermissionGate(ctx.permission, async (sessionID) => ({
		review: await loadDecision(ctx.storage, sessionID),
		route: await loadRoute(ctx.storage, sessionID),
	}))
}

/**
 * v2 reads `id` and `setup()` from the default export and rejects
 * function-shaped exports at load.
 */
export default {
	id: "ywai-jev-gate",
	setup: async (ctx: V2PluginContext) => {
		// Never let a registration failure reject setup(): a rejected setup
		// silently yields a loaded plugin with no tools, which reads to the
		// agent as "Unknown tool" and to the user as nothing at all.
		try {
			await setup(ctx as JevContext)
		} catch (err) {
			recordSetupFailure("setup", err)
		}
		if (process.env.JEV_GATE_PROBE === "1") {
			try {
				const { setupProbes } = await import("./probes")
				await setupProbes(ctx as never)
			} catch (err) {
				recordSetupFailure("probes", err)
			}
		}
	},
}
