/**
 * jev_do's engine: the vendored browser-use/jev-ultrafast CLI (vendor/jev_ultrafast).
 * One JSON request on stdin; JSON lines back (states, then one result).
 * Values never leave this process except on the child's stdin.
 */
import { spawn } from "node:child_process"
import { existsSync } from "node:fs"
import { fileURLToPath } from "node:url"

export interface UltrafastRequest {
	url: string
	goal: string
	values: Record<string, string>
	profile_dir?: string
}

export interface UltrafastStep {
	step?: number
	action?: string
	kind?: string
	text?: string | null
	url?: string
	page_changed?: boolean | null
}

export interface UltrafastResult {
	type: "result"
	status: "done" | "blocked" | "error"
	history: UltrafastStep[]
	elapsed_ms?: number
	final_url?: string
	error?: string
	reason?: string
	final_page?: { url?: string; title?: string; snapshot: string }
}

export const DEFAULT_TIMEOUT_MS = 180_000

/** src/browser/ in the repo, dist/ in the bundle; JEV_ULTRAFAST_DIR wins. */
export function vendorDir(): string {
	const candidates = [
		process.env.JEV_ULTRAFAST_DIR,
		fileURLToPath(new URL("../../vendor/jev_ultrafast", import.meta.url)),
		fileURLToPath(new URL("../vendor/jev_ultrafast", import.meta.url)),
	]
	const found = candidates.find((dir) => dir && existsSync(dir))
	if (!found) throw new Error("vendor/jev_ultrafast not found; set JEV_ULTRAFAST_DIR.")
	return found
}

export function runUltrafast(
	request: UltrafastRequest,
	opts: {
		command?: string[]
		env?: Record<string, string>
		timeoutMs?: number
		onState?: (state: Record<string, unknown>) => void
	} = {},
): Promise<UltrafastResult> {
	const [cmd, ...args] = opts.command ?? [
		"uv",
		"run",
		"--quiet",
		"--project",
		vendorDir(),
		"python",
		"-m",
		"jev_ultrafast.cli",
	]
	return new Promise((resolve) => {
		const fail = (error: string): UltrafastResult => ({ type: "result", status: "error", history: [], error })
		const child = spawn(cmd, args, { env: { ...process.env, ...opts.env }, stdio: ["pipe", "pipe", "pipe"] })
		let result: UltrafastResult | undefined
		let buffer = ""
		let stderr = ""
		let timedOut = false
		const timer = setTimeout(() => {
			timedOut = true
			child.kill("SIGTERM")
		}, opts.timeoutMs ?? DEFAULT_TIMEOUT_MS)
		const line = (raw: string) => {
			let msg: Record<string, unknown>
			try {
				msg = JSON.parse(raw)
			} catch {
				return // uv or library chatter
			}
			if (msg.type === "result") result = msg as unknown as UltrafastResult
			else if (msg.type === "state") opts.onState?.(msg)
		}
		child.stdout.on("data", (chunk) => {
			buffer += chunk
			const lines = buffer.split("\n")
			buffer = lines.pop() ?? ""
			lines.filter((l) => l.trim()).forEach(line)
		})
		child.stderr.on("data", (chunk) => {
			stderr = (stderr + chunk).slice(-2000)
		})
		child.on("error", (err) => {
			clearTimeout(timer)
			resolve(fail(`could not start ${cmd}: ${err.message}`))
		})
		child.on("close", (code) => {
			clearTimeout(timer)
			if (buffer.trim()) line(buffer)
			if (timedOut) resolve(fail(`timed out after ${opts.timeoutMs ?? DEFAULT_TIMEOUT_MS} ms`))
			else resolve(result ?? fail(`exited ${code} without a result. ${stderr.trim()}`))
		})
		child.stdin.end(JSON.stringify(request))
	})
}

export function summarizeUltrafast(result: UltrafastResult, goal: string): string {
	const story = result.history
		.map((h) => (h.kind === "fill" ? `typed into **${h.action}**` : h.kind === "click" ? `clicked **${h.action}**` : h.kind))
		.join(", then ")
	return [
		`**Test:** ${goal}`,
		`**Result:** ${result.status}` + (result.elapsed_ms != null ? ` in ${result.elapsed_ms} ms` : ""),
		result.reason ? `**Reason:** ${result.reason}` : "",
		result.error ? `**Error:** ${result.error}` : "",
		story ? `**What happened:** ${story}` : "Nothing was clicked.",
		result.final_url ? `**Final URL:** ${result.final_url}` : "",
		result.final_page?.snapshot
			? `**Final page** (pass this as jev_check_page \`snapshot\`):\n\`\`\`\n${result.final_page.snapshot}\n\`\`\``
			: "",
		"done means Jev stopped acting, not that the scenario passed. Check the Then with jev_check_page.",
	]
		.filter(Boolean)
		.join("\n")
}
