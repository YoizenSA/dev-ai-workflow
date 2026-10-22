/**
 * Supervision over OpenCode v2's NATIVE background subagents.
 *
 * Launch, model selection and the parent completion notification
 * (`<subagent sessionID=… state=…>`) are all native. This adds what the native
 * tool lacks: a watchdog timeout, stop with partial output, steer, a live
 * transcript digest, and lookup of past children after compaction or restart.
 *
 * The id of a child is its native sessionID. Results are read from the child
 * session itself (session.context), never from a copy of ours.
 */

import * as fs from "node:fs"
import * as path from "node:path"
import { toArray } from "../../../shared/v2"
import type { DelegationTerminalEventData } from "./terminal-events"

export type ChildStatus = "running" | "complete" | "error" | "stopped" | "timeout"

export interface ChildRecord {
	sessionID: string
	parentSessionID: string
	agent: string
	description: string
	startedAt: number
	/** 0 = no watchdog. */
	timeoutMs: number
	status: ChildStatus
	endedAt?: number
	error?: string
}

/** The slice of the v2 session domain this module drives. */
export interface SupervisorSession {
	wait?(input: { sessionID: string }): Promise<unknown>
	context?(input: { sessionID: string }): Promise<unknown>
	interrupt?(input: { sessionID: string }): Promise<unknown>
	prompt?(input: { sessionID: string; text: string; delivery: "steer" }): Promise<unknown>
}

export interface SupervisorOptions {
	session: SupervisorSession
	/** Persisted index: v2 plugins have no API to list a session's children. */
	indexFile: string
	defaultTimeoutMs: number
	onTerminal?: (event: DelegationTerminalEventData) => void
	/** Pause between a wait() that returned early and the next settle check. */
	settleDelayMs?: number
}

// ponytail: keep the newest entries only; the index is a lookup aid, not an archive.
const MAX_INDEX = 200
const MAX_TEXT_BLOCK = 700
const MAX_DIGEST = 4_000
const MAX_READ = 20_000

function errorText(error: unknown): string {
	if (!error) return ""
	if (typeof error === "string") return error
	const e = error as { message?: unknown; data?: { message?: unknown } }
	return String(e.message ?? e.data?.message ?? JSON.stringify(error))
}

/** Final assistant text of a transcript (what the native completion delivers). */
export function finalText(messages: any[]): string {
	const last = [...messages].reverse().find((m) => m?.type === "assistant")
	return (last?.content ?? [])
		.filter((p: any) => p?.type === "text" && p.text)
		.map((p: any) => p.text)
		.join("\n\n")
}

/** Tail-biased digest of a live transcript: assistant text, tool activity, steers. */
export function digest(messages: any[]): string {
	const lines: string[] = []
	for (const m of messages) {
		if (m?.type === "user" && typeof m.text === "string" && m.text.startsWith("[SUPERVISOR STEER]")) {
			lines.push(`>> ${m.text.slice(0, 200)}`)
		}
		if (m?.type !== "assistant") continue
		for (const part of m.content ?? []) {
			if (part?.type === "text" && part.text?.trim()) {
				const text = part.text.trim()
				lines.push(text.length > MAX_TEXT_BLOCK ? `${text.slice(0, MAX_TEXT_BLOCK)} […]` : text)
			} else if (part?.type === "tool") {
				const s = part.state ?? {}
				const title = s.title ? `: ${s.title}` : ""
				const err = s.status === "error" ? ` — ${errorText(s.error).slice(0, 160)}` : ""
				lines.push(`[tool] ${part.name} (${s.status})${title}${err}`)
			}
		}
	}
	const out = lines.join("\n")
	return out.length > MAX_DIGEST ? `[… earlier activity trimmed …]\n${out.slice(-MAX_DIGEST)}` : out
}

/** The child's loop has produced a finished (or failed) assistant turn last. */
function settled(messages: any[]): boolean {
	const last = [...messages].reverse().find((m) => m?.type === "assistant" || m?.type === "user")
	return last?.type === "assistant" && (last.time?.completed !== undefined || last.error !== undefined)
}

export class Supervisor {
	private children = new Map<string, ChildRecord>()
	private timers = new Map<string, ReturnType<typeof setTimeout>>()
	private disposed = false

	constructor(private opts: SupervisorOptions) {}

	/** Re-adopt the persisted index; still-running children get their watcher back. */
	restore(): void {
		let rows: ChildRecord[] = []
		try {
			rows = JSON.parse(fs.readFileSync(this.opts.indexFile, "utf8"))
		} catch {
			return
		}
		for (const row of Array.isArray(rows) ? rows : []) {
			if (!row?.sessionID) continue
			this.children.set(row.sessionID, row)
			if (row.status === "running") this.watch(row)
		}
	}

	track(input: {
		sessionID: string
		parentSessionID: string
		agent: string
		description: string
		timeoutMs?: number
	}): void {
		if (this.children.get(input.sessionID)?.status === "running") return
		const record: ChildRecord = {
			sessionID: input.sessionID,
			parentSessionID: input.parentSessionID,
			agent: input.agent,
			description: input.description,
			startedAt: Date.now(),
			timeoutMs: input.timeoutMs ?? this.opts.defaultTimeoutMs,
			status: "running",
		}
		this.children.set(record.sessionID, record)
		this.persist()
		this.watch(record)
	}

	private watch(record: ChildRecord): void {
		this.arm(record)
		void this.awaitSettle(record.sessionID)
	}

	private arm(record: ChildRecord): void {
		clearTimeout(this.timers.get(record.sessionID))
		if (record.timeoutMs <= 0) return
		const left = record.startedAt + record.timeoutMs - Date.now()
		this.timers.set(
			record.sessionID,
			setTimeout(() => void this.halt(record.sessionID, "timeout"), Math.max(left, 0)),
		)
	}

	/**
	 * wait() can return before a freshly admitted child has started its loop,
	 * so a return only counts once the transcript shows a settled turn.
	 */
	private async awaitSettle(sessionID: string): Promise<void> {
		const wait = this.opts.session.wait
		if (!wait) return // ponytail: no wait on this host → status stays "running" until stop/timeout
		while (!this.disposed && this.children.get(sessionID)?.status === "running") {
			try {
				await wait.call(this.opts.session, { sessionID })
			} catch {
				return
			}
			const messages = await this.messages(sessionID)
			if (this.children.get(sessionID)?.status !== "running") return
			if (settled(messages)) {
				const last = [...messages].reverse().find((m) => m?.type === "assistant")
				this.finish(sessionID, last?.error ? "error" : "complete", errorText(last?.error) || undefined)
				return
			}
			await new Promise((r) => setTimeout(r, this.opts.settleDelayMs ?? 1_000))
		}
	}

	private finish(sessionID: string, status: ChildStatus, error?: string): void {
		const record = this.children.get(sessionID)
		if (!record || record.status !== "running") return
		clearTimeout(this.timers.get(sessionID))
		this.timers.delete(sessionID)
		record.status = status
		record.endedAt = Date.now()
		if (error) record.error = error
		this.persist()
		const remaining = [...this.children.values()].filter(
			(c) => c.parentSessionID === record.parentSessionID && c.status === "running",
		).length
		try {
			this.opts.onTerminal?.({
				kind: "terminal",
				delegationID: sessionID,
				agent: record.agent,
				status,
				sessionID,
				parentSessionID: record.parentSessionID,
				remaining,
				title: record.description || undefined,
				...(error ? { error } : {}),
			})
		} catch {
			// The human channel must never break supervision.
		}
	}

	/** Interrupt natively; the native job then notifies the parent on its own. */
	private async halt(sessionID: string, status: "stopped" | "timeout"): Promise<void> {
		const reason = status === "timeout" ? "Timed out by watchdog" : "Stopped by supervisor"
		this.finish(sessionID, status, reason)
		try {
			await this.opts.session.interrupt?.({ sessionID })
		} catch {
			// Best-effort: the record already says it was halted.
		}
	}

	private async messages(sessionID: string): Promise<any[]> {
		try {
			return toArray(await this.opts.session.context?.({ sessionID }))
		} catch {
			return []
		}
	}

	private persist(): void {
		const rows = [...this.children.values()].slice(-MAX_INDEX)
		try {
			fs.mkdirSync(path.dirname(this.opts.indexFile), { recursive: true })
			fs.writeFileSync(this.opts.indexFile, JSON.stringify(rows))
		} catch {
			// A lost index only costs post-restart lookup, never a running child.
		}
	}

	private visible(caller: string, id: string): ChildRecord | undefined {
		const record = this.children.get(id.trim())
		return record && record.parentSessionID === caller ? record : undefined
	}

	private notFound(id: string): string {
		return `❌ Subagent "${id}" not found in this session. Use subagent_status() to list them.`
	}

	async stop(caller: string, id: string): Promise<string> {
		const record = this.visible(caller, id)
		if (!record) return this.notFound(id)
		if (record.status !== "running") {
			return `ℹ️ Subagent "${id}" already ${record.status}. Use subagent_read("${id}").`
		}
		await this.halt(record.sessionID, "stopped")
		const partial = finalText(await this.messages(record.sessionID))
		return `🛑 Subagent "${id}" stopped.\n\n${partial || "(no output yet)"}`
	}

	async steer(caller: string, id: string, message: string): Promise<string> {
		const text = message.trim()
		if (!text) return "❌ Steering message is required."
		const record = this.visible(caller, id)
		if (!record) return this.notFound(id)
		if (record.status !== "running") {
			return `❌ Subagent "${id}" is ${record.status}; cannot steer a finished task.`
		}
		try {
			await this.opts.session.prompt?.({
				sessionID: record.sessionID,
				text: `[SUPERVISOR STEER] ${text}`,
				delivery: "steer",
			})
		} catch (error) {
			return `❌ Steer could not be delivered to "${id}": ${errorText(error)}. Retry, or subagent_stop("${id}").`
		}
		// A steer extends the run, so it also gets a fresh timeout window.
		record.startedAt = Date.now()
		this.persist()
		this.arm(record)
		return `✅ Steer sent to "${id}". It acts on it in its current run.`
	}

	async read(caller: string, id: string): Promise<string> {
		const record = this.visible(caller, id)
		if (!record) return this.notFound(id)
		const text = finalText(await this.messages(record.sessionID))
		const body = text.length > MAX_READ ? `${text.slice(0, MAX_READ)}\n[… truncated …]` : text
		return `## ${record.sessionID} [${record.status}] ${record.agent} — ${record.description}\n\n${body || "(no text output)"}`
	}

	/** Without id: every child of this session. With id: that child's live digest. */
	async status(caller: string, id?: string): Promise<string> {
		const now = Date.now()
		if (id) {
			const record = this.visible(caller, id)
			if (!record) return this.notFound(id)
			const elapsed = Math.round(((record.endedAt ?? now) - record.startedAt) / 1000)
			const body = digest(await this.messages(record.sessionID)) || "(no visible activity yet)"
			return `## ${record.sessionID} [${record.status}] agent=${record.agent} elapsed=${elapsed}s\n\n${body}`
		}
		const mine = [...this.children.values()].filter((c) => c.parentSessionID === caller)
		if (mine.length === 0) return "No background subagents in this session."
		const lines = mine.map((c) => {
			const elapsed = Math.round(((c.endedAt ?? now) - c.startedAt) / 1000)
			const deadline =
				c.status !== "running"
					? ""
					: c.timeoutMs > 0
						? ` · timeout in ${Math.max(Math.round((c.startedAt + c.timeoutMs - now) / 1000), 0)}s`
						: " · no timeout"
			return `- **${c.sessionID}** [${c.status}] ${c.agent} — ${c.description} · ${elapsed}s${deadline}${c.error ? ` · ${c.error}` : ""}`
		})
		return `## Background subagents\n\n${lines.join("\n")}`
	}

	dispose(): void {
		this.disposed = true
		for (const timer of this.timers.values()) clearTimeout(timer)
		this.timers.clear()
	}
}
