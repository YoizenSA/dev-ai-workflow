/**
 * DelegationManager v2 — supervision for delegated child sessions.
 *
 * What survives from the v1 plugin: readable ids, per-delegation timeout with
 * steer-reset, persisted state, debounced parent notification, watchdog tick.
 * What v2 natives replace: steer is a prompt with delivery:"steer", stop is
 * session.interrupt, parent notification is a queued synthetic message, and
 * sync completion is event-driven (session.idle / step ended) because the
 * beta runtime's session.wait answers 503 "not available yet".
 */

import * as fs from "node:fs/promises"
import * as os from "node:os"
import * as path from "node:path"
import {
	createSessionIO,
	type SessionIO,
	type V2Event,
} from "./adapter"
import { generateUniqueId } from "./id"
import {
	type DelegationRecord,
	type DelegationStatus,
	type ModelRef,
	listRecords,
	writeDelegationFile,
} from "./state"
import { Transcript } from "./transcript"

// Default max runtime. Global override: BACKGROUND_AGENTS_TIMEOUT_MINUTES.
// Per-delegation override: delegate's timeout_minutes. 0 = no timeout.
const DEFAULT_MAX_RUN_TIME_MS = (() => {
	const minutes = Number(process.env.BACKGROUND_AGENTS_TIMEOUT_MINUTES)
	if (Number.isFinite(minutes) && minutes >= 0) return minutes * 60_000
	return 15 * 60_000
})()

const WATCHDOG_INTERVAL_MS = 5_000
/** A delivered steer re-opens a fresh window of the same size. */
const COMPLETE_DEBOUNCE_MS = 750
/** Quiet period after a finished step before a sync caller is released. */
const STEP_QUIET_MS = 1_500

export interface DelegateInput {
	parentSessionID: string
	parentMessageID?: string
	parentAgent?: string
	prompt: string
	agent: string
	/** sync = return the result from the delegate call; async = return an id. */
	mode: "sync" | "async"
	/** "provider/model-id", optionally "#variant". */
	model?: ModelRef
	/** Max runtime in ms; 0 = unlimited. Undefined = configured default. */
	maxRunTimeMs?: number
	title?: string
}

export interface DelegationListItem {
	id: string
	status: DelegationStatus
	agent?: string
	title?: string
	description?: string
	unread?: boolean
}

interface CompletionWaiter {
	sessionID: string
	resolve: (outcome: { status: DelegationStatus; result: string; error?: string }) => void
	timer?: ReturnType<typeof setTimeout>
	quietTimer?: ReturnType<typeof setTimeout>
}

type Logger = (msg: string) => void

export interface ManagerOptions {
	/** Quiet period after a finished step before completion is declared. */
	stepQuietMs?: number
}

export class DelegationManager {
	private readonly io: SessionIO
	private readonly recordsDir: string
	private readonly log: Logger
	private readonly stepQuietMs: number
	private readonly records = new Map<string, DelegationRecord>()
	private readonly transcripts = new Map<string, Transcript>()
	private readonly waiters = new Map<string, CompletionWaiter>()
	private watchdog?: ReturnType<typeof setInterval>
	/** debounced completion flushes: delegation id → timer */
	private readonly pendingNotifications = new Map<string, ReturnType<typeof setTimeout>>()

	constructor(io: SessionIO, recordsDir: string, log: Logger = () => {}, options?: ManagerOptions) {
		this.io = io
		this.recordsDir = recordsDir
		this.log = log
		this.stepQuietMs = options?.stepQuietMs ?? STEP_QUIET_MS
	}

	async start(): Promise<void> {
		await fs.mkdir(this.recordsDir, { recursive: true })
		await this.restore()
		this.watchdog = setInterval(() => void this.tick(), WATCHDOG_INTERVAL_MS)
		// Keep the process releasable even if a watchdog handle lingers.
		this.watchdog.unref?.()
	}

	async stop(): Promise<void> {
		if (this.watchdog) clearInterval(this.watchdog)
		this.watchdog = undefined
	}

	// ── delegation lifecycle ────────────────────────────────────────────────

	async delegate(input: DelegateInput): Promise<
		| { mode: "sync"; status: DelegationStatus; result: string; error?: string; id: string }
		| { mode: "async"; id: string; childSessionID: string; timeoutMs: number }
	> {
		const maxRunTimeMs =
			input.maxRunTimeMs !== undefined ? input.maxRunTimeMs : DEFAULT_MAX_RUN_TIME_MS

		const id = generateUniqueId((candidate) => this.records.has(candidate))
		const childSessionID = await this.io.create({
			agent: input.agent,
			model: input.model,
			title: `${input.agent} · ${id}`,
		})

		const now = Date.now()
		const record: DelegationRecord = {
			id,
			parentSessionID: input.parentSessionID,
			parentMessageID: input.parentMessageID,
			childSessionID,
			agent: input.agent,
			model: input.model,
			mode: input.mode,
			prompt: input.prompt,
			title: input.title,
			status: "running",
			createdAt: now,
			startedAt: now,
			lastActivityAt: now,
			deadlineAt: maxRunTimeMs > 0 ? now + maxRunTimeMs : 0,
			steerCount: 0,
			toolCalls: 0,
			notified: false,
		}
		this.records.set(id, record)
		this.transcripts.set(childSessionID, new Transcript())
		await this.persist(record)

		await this.io.prompt(childSessionID, input.prompt)

		if (input.mode === "sync") {
			const outcome = await this.waitForCompletion(id, maxRunTimeMs)
			return { mode: "sync", id, ...outcome }
		}
		return { mode: "async", id, childSessionID, timeoutMs: maxRunTimeMs }
	}

	/** Resolves when the child turn completes (or the timeout/stop lands). */
	private waitForCompletion(
		id: string,
		maxRunTimeMs: number,
	): Promise<{ status: DelegationStatus; result: string; error?: string }> {
		const record = this.records.get(id)
		if (!record) {
			return Promise.resolve({ status: "failed", result: "", error: "delegation vanished" })
		}
		return new Promise((resolve) => {
			const waiter: CompletionWaiter = {
				sessionID: record.childSessionID,
				resolve: (outcome) => {
					if (waiter.timer) clearTimeout(waiter.timer)
					if (waiter.quietTimer) clearTimeout(waiter.quietTimer)
					this.waiters.delete(record.childSessionID)
					resolve(outcome)
				},
			}
			this.waiters.set(record.childSessionID, waiter)
			if (maxRunTimeMs > 0) {
				waiter.timer = setTimeout(() => {
					void this.expire(id).then((outcome) => waiter.resolve(outcome))
				}, maxRunTimeMs)
			}
		})
	}

	/**
	 * Completion signal (step finished cleanly / session went idle): finalize
	 * after a short quiet period, which covers both sync waiters (released by
	 * finalize) and async delegations (notified). Re-arming resets the timer,
	 * so back-to-back steps in one turn do not fire early.
	 */
	private armCompletion(sessionID: string): void {
		const record = this.findByChild(sessionID)
		if (!record || record.status !== "running") return
		const previous = this.completionTimers.get(sessionID)
		if (previous) clearTimeout(previous)
		const timer = setTimeout(() => {
			this.completionTimers.delete(sessionID)
			void this.completeBySession(sessionID, "completed")
		}, this.stepQuietMs)
		this.completionTimers.set(sessionID, timer)
	}

	private async expire(
		id: string,
	): Promise<{ status: DelegationStatus; result: string; error?: string }> {
		const record = this.records.get(id)
		if (!record || record.status !== "running") {
			return { status: "timeout", result: this.resultFor(id), error: "already finished" }
		}
		try {
			await this.io.interrupt(record.childSessionID)
		} catch (error) {
			this.log(`interrupt on timeout failed: ${String(error)}`)
		}
		const result = this.resultFor(id)
		await this.finalize(record, "timeout", result, "deadline exceeded")
		return { status: "timeout", result, error: "deadline exceeded" }
	}

	private async completeBySession(
		sessionID: string,
		status: DelegationStatus,
		error?: string,
	): Promise<void> {
		const record = this.findByChild(sessionID)
		if (!record || record.status !== "running") return
		const result = this.resultFor(record.id)
		await this.finalize(record, status, result, error)
	}

	private async finalize(
		record: DelegationRecord,
		status: DelegationStatus,
		result: string,
		error?: string,
	): Promise<void> {
		record.status = status
		record.completedAt = Date.now()
		record.result = result
		if (error) record.error = error
		await this.persist(record)

		const waiter = this.waiters.get(record.childSessionID)
		if (waiter) waiter.resolve({ status, result, error })

		if (record.mode === "async" && !record.notified) {
			this.scheduleNotification(record)
		}
	}

	// ── v2 event feed ───────────────────────────────────────────────────────

	handleEvent(event: V2Event): void {
		const data = (event.data ?? {}) as Record<string, any>
		const sessionID: string | undefined = data.sessionID
		if (!sessionID) return
		const transcript = this.transcripts.get(sessionID)
		if (transcript) transcript.touch()

		switch (event.type) {
			case "session.next.text.delta":
			case "session.text.delta":
				if (transcript && typeof data.delta === "string") {
					transcript.appendText(String(data.assistantMessageID ?? "text"), data.delta)
				}
				this.markActivity(sessionID)
				break
			case "session.next.tool.called":
			case "session.tool.called":
				transcript?.recordTool(String(data.tool ?? data.id ?? "tool"), "called")
				this.bumpToolCalls(sessionID)
				this.markActivity(sessionID)
				break
			case "session.next.tool.success":
			case "session.next.tool.failed":
			case "session.tool.success":
			case "session.tool.failed":
				transcript?.recordTool(
					String(data.tool ?? data.id ?? "tool"),
					event.type.endsWith("failed") ? "failed" : "success",
				)
				this.markActivity(sessionID)
				break
			case "session.next.step.ended":
			case "session.step.ended":
				if (data.finish === "stop" || data.finish === "length") {
					this.armCompletion(sessionID)
				}
				this.markActivity(sessionID)
				break
			case "session.execution.succeeded":
				void this.completeBySession(sessionID, "completed")
				break
			case "session.execution.failed":
				void this.completeBySession(sessionID, "failed", String(data.error ?? "execution failed"))
				break
			case "session.execution.interrupted":
				void this.completeBySession(sessionID, "interrupted", "interrupted")
				break
			case "session.idle":
				// A child going idle finished its turn — release waiters and
				// finalize async completions that no execution event covered.
				this.armIdleCompletion(sessionID)
				break
			default:
				break
		}
	}

	private readonly completionTimers = new Map<string, ReturnType<typeof setTimeout>>()

	private armIdleCompletion(sessionID: string): void {
		// idle fires between turns too (waiting for the next prompt), so it
		// arms the same debounced completion as a finished step.
		this.armCompletion(sessionID)
	}

	private markActivity(sessionID: string): void {
		const record = this.findByChild(sessionID)
		if (record) record.lastActivityAt = Date.now()
	}

	private bumpToolCalls(sessionID: string): void {
		const record = this.findByChild(sessionID)
		if (record) record.toolCalls++
	}

	// ── supervisor controls ─────────────────────────────────────────────────

	async steer(sessionID: string, id: string, message: string): Promise<string> {
		const record = this.resolveVisible(sessionID, id)
		if (!record) return `❌ No delegation '${id}' found for this session.`
		if (record.status !== "running") {
			return `❌ Delegation '${id}' is ${record.status} — only running delegations can be steered.`
		}
		await this.io.prompt(record.childSessionID, message, "steer")
		record.steerCount++
		// A delivered steer re-opens a fresh timeout window of the same size.
		if (record.deadlineAt > 0) {
			const window = this.deadlineWindow(record)
			record.deadlineAt = Date.now() + window
		}
		record.lastActivityAt = Date.now()
		await this.persist(record)
		return `Steered ${id}: instruction delivered into the running agent. Timeout window restarted.`
	}

	async stopDelegation(sessionID: string, id: string): Promise<string> {
		const record = this.resolveVisible(sessionID, id)
		if (!record) return `❌ No delegation '${id}' found for this session.`
		if (record.status !== "running") {
			return `Delegation '${id}' is already ${record.status}.`
		}
		try {
			await this.io.interrupt(record.childSessionID)
		} catch (error) {
			this.log(`interrupt failed: ${String(error)}`)
		}
		const result = this.resultFor(id)
		await this.finalize(record, "stopped", result, "stopped by supervisor")
		return `Stopped ${id}. Partial output saved — read it with delegation_read(id="${id}").`
	}

	async peek(sessionID: string, id: string): Promise<string> {
		const record = this.resolveVisible(sessionID, id)
		if (!record) return `❌ No delegation '${id}' found for this session.`
		const transcript = this.transcripts.get(record.childSessionID)
		if (!transcript) {
			return record.result
				? `## ${id} [${record.status}]\n\n${record.result}`
				: `## ${id} [${record.status}]\n\n_(no live transcript — process may have restarted)_`
		}
		return `## ${id} [${record.status}]\n\n${transcript.peekDigest()}`
	}

	async statusReport(sessionID: string): Promise<string> {
		const all = this.listByParent(sessionID)
		const running = all.filter((r) => r.status === "running")
		if (running.length === 0) {
			return "No running delegations for this session."
		}
		const lines = running.map((r) => {
			const elapsed = Math.round((Date.now() - r.startedAt) / 1000)
			const timeout =
				r.deadlineAt > 0
					? `${Math.max(0, Math.round((r.deadlineAt - Date.now()) / 1000))}s left`
					: "no deadline"
			const steers = r.steerCount ? `, ${r.steerCount} steer(s)` : ""
			const tools = r.toolCalls ? `, ${r.toolCalls} tool calls` : ""
			return `- **${r.id}** (${r.agent}) — ${elapsed}s elapsed, ${timeout}${tools}${steers}`
		})
		return `## Running delegations\n\n${lines.join("\n")}`
	}

	async list(sessionID: string): Promise<DelegationListItem[]> {
		return this.listByParent(sessionID).map((r) => ({
			id: r.id,
			status: r.status,
			agent: r.agent,
			title: r.title,
			description: r.prompt.slice(0, 120),
			unread: r.status !== "running" && !r.notified,
		}))
	}

	async read(sessionID: string, id: string): Promise<string> {
		const record = this.resolveVisible(sessionID, id)
		if (!record) return `❌ No delegation '${id}' found for this session.`
		record.notified = true
		await this.persist(record)
		if (record.result) return record.result
		const transcript = this.transcripts.get(record.childSessionID)
		if (transcript && record.status === "running") {
			return `_(still running — latest output below)_\n\n${transcript.peekDigest()}`
		}
		return `_(no persisted output for '${id}')_`
	}

	// ── notifications ───────────────────────────────────────────────────────

	private scheduleNotification(record: DelegationRecord): void {
		const previous = this.pendingNotifications.get(record.id)
		if (previous) clearTimeout(previous)
		const timer = setTimeout(() => {
			this.pendingNotifications.delete(record.id)
			void this.deliverNotification(record)
		}, COMPLETE_DEBOUNCE_MS)
		this.pendingNotifications.set(record.id, timer)
	}

	private async deliverNotification(record: DelegationRecord): Promise<void> {
		const summary =
			record.result && record.result.length > 1_500
				? `${record.result.slice(0, 1_500)}\n…(truncated — full output via delegation_read(id="${record.id}"))`
				: (record.result ?? "(no output)")
		const label = record.status === "completed" ? "COMPLETE" : record.status.toUpperCase()
		const text =
			`<task-notification> delegation ${record.id} (${record.agent}) ${label}\n` +
			`Duration: ${Math.round(((record.completedAt ?? Date.now()) - record.startedAt) / 1000)}s` +
			`${record.steerCount ? ` · steers: ${record.steerCount}` : ""}` +
			`${record.toolCalls ? ` · tool calls: ${record.toolCalls}` : ""}\n\n${summary}`
		try {
			await this.io.synthetic(record.parentSessionID, text, `delegation ${record.id} ${label}`)
			record.notified = true
			await this.persist(record)
		} catch (error) {
			this.log(`parent notification failed: ${String(error)}`)
		}
	}

	// ── watchdog ────────────────────────────────────────────────────────────

	private async tick(): Promise<void> {
		const now = Date.now()
		for (const record of this.records.values()) {
			if (record.status !== "running") continue
			if (record.deadlineAt > 0 && now > record.deadlineAt) {
				await this.expire(record.id)
			}
		}
	}

	// ── persistence & lookup ────────────────────────────────────────────────

	private deadlineWindow(record: DelegationRecord): number {
		// Window size = configured max runtime (steer restarts it whole).
		return DEFAULT_MAX_RUN_TIME_MS
	}

	private resultFor(id: string): string {
		const record = this.records.get(id)
		if (!record) return ""
		const transcript = this.transcripts.get(record.childSessionID)
		return transcript?.resultText() ?? record.result ?? ""
	}

	private findByChild(sessionID: string): DelegationRecord | undefined {
		for (const record of this.records.values()) {
			if (record.childSessionID === sessionID) return record
		}
		return undefined
	}

	private listByParent(sessionID: string): DelegationRecord[] {
		return [...this.records.values()]
			.filter((r) => r.parentSessionID === sessionID)
			.sort((a, b) => b.startedAt - a.startedAt)
	}

	private resolveVisible(parentSessionID: string, id: string): DelegationRecord | undefined {
		return this.records.get(id)?.parentSessionID === parentSessionID
			? this.records.get(id)
			: undefined
	}

	private async persist(record: DelegationRecord): Promise<void> {
		try {
			await writeDelegationFile(this.recordsDir, record)
		} catch (error) {
			this.log(`persist failed for ${record.id}: ${String(error)}`)
		}
	}

	/**
	 * Loads persisted records. Delegations still marked running after a
	 * restart are finalized as interrupted: the in-memory transcript that
	 * fed their result is gone, so claiming "running" would be a lie.
	 */
	private async restore(): Promise<void> {
		const records = await listRecords(this.recordsDir)
		let restored = 0
		for (const record of records) {
			if (record.status === "running") {
				record.status = "interrupted"
				record.completedAt = Date.now()
				record.error = "process restarted"
			}
			this.records.set(record.id, record)
			restored++
		}
		if (restored > 0) this.log(`restored ${restored} delegation records`)
	}
}

/** Default records dir for a project key, mirroring the v1 layout. */
export function defaultRecordsDir(projectKey: string): string {
	return path.join(os.homedir(), ".local", "share", "opencode", "delegations", projectKey)
}

export { createSessionIO }
