/**
 * CLI front for the v2 delegation engine. Subcommands map 1:1 to the tool
 * surface so the CLI, the MCP server and (when the runtime grows tool
 * domains) the plugin all behave identically.
 *
 *   background-agents-v2 delegate --agent dev --mode sync "prompt"
 *   background-agents-v2 steer <id> "message"
 *   background-agents-v2 peek|stop|read <id>
 *   background-agents-v2 status|list
 *   background-agents-v2 watch <parentSessionID> <childSessionID> <id> <timeoutMs>
 *
 * Async delegations run inside the opencode server; this process only needs
 * to exist for sync waits. A detached `watch` process per async delegation
 * delivers the completion notification, so no daemon is required.
 */

import { effortToVariant, formatModelRef, parseModelString } from "./adapter"
import { followEvents, httpSessionIO, serverBaseURL } from "./http-api"
import { DelegationManager, defaultRecordsDir } from "./manager"
import type { DelegationRecord } from "./state"
import { writeDelegationFile } from "./state"
import { Transcript } from "./transcript"

function usage(): string {
	return `background-agents-v2 — OpenCode v2 delegation engine

Usage:
  delegate --agent <agent> [--mode sync|async] [--model provider/model] [--effort high|medium|low]
           [--timeout-minutes N] [--parent <sessionID>] <prompt>
  steer <id> <message>        Inject an instruction into a running delegation
  peek <id>                   Live transcript digest of a running delegation
  stop <id>                   Interrupt a running delegation
  read <id>                   Full persisted output
  status                      Running delegations for the default parent
  list                        All delegations
  watch <parent> <child> <id> <timeoutMs>   (internal) notify on completion

Environment:
  OPENCODE_URL   opencode v2 server (default http://127.0.0.1:4096)
  BACKGROUND_AGENTS_TIMEOUT_MINUTES   default timeout (default 15, 0 = none)
`
}

function fail(message: string): never {
	console.error(message)
	process.exit(1)
}

async function fetchMessages(base: string, sessionID: string): Promise<any[]> {
	try {
		const res = await fetch(`${base}/api/session/${encodeURIComponent(sessionID)}/message`, {
			headers: { accept: "application/json" },
		})
		if (!res.ok) return []
		const body = (await res.json()) as { data?: unknown }
		return Array.isArray(body?.data) ? (body.data as any[]) : []
	} catch {
		return []
	}
}

/** Best-effort transcript from the server's message list (cross-process). */
async function transcriptFromServer(base: string, sessionID: string): Promise<Transcript> {
	const transcript = new Transcript()
	for (const message of await fetchMessages(base, sessionID)) {
		const parts = message?.parts ?? message?.info?.parts ?? []
		for (const part of parts) {
			if (part?.type === "text" && typeof part.text === "string") {
				transcript.appendText(message.info?.id ?? message.id ?? "text", part.text)
			}
			if (typeof part?.tool === "string") {
				transcript.recordTool(part.tool, part.state?.status === "error" ? "failed" : "success")
			}
		}
	}
	return transcript
}

async function loadRecordByFuzzyId(recordsDir: string, id: string): Promise<DelegationRecord | undefined> {
	const { listRecords } = await import("./state")
	const records = await listRecords(recordsDir)
	return records.find((r) => r.id === id || r.childSessionID === id || r.id.startsWith(id))
}

async function main(argv: string[]): Promise<number> {
	const [command, ...rest] = argv
	if (!command || command === "help" || command === "--help") {
		console.log(usage())
		return 0
	}
	const base = serverBaseURL()
	const recordsDir = defaultRecordsDir(process.env.BACKGROUND_AGENTS_PROJECT_KEY ?? "default")

	const managerFor = () =>
		new DelegationManager(httpSessionIO(base), recordsDir, (m) => console.error(m))

	switch (command) {
		case "delegate": {
			let agent = ""
			let mode = "async"
			let model: string | undefined
			let effort: string | undefined
			let timeoutMinutes: number | undefined
			let parent = process.env.BACKGROUND_AGENTS_PARENT ?? "ses_unknown"
			const positional: string[] = []
			for (let i = 0; i < rest.length; i++) {
				const arg = rest[i]
				switch (arg) {
					case "--agent": agent = rest[++i] ?? ""; break
					case "--mode": mode = rest[++i] ?? "async"; break
					case "--model": model = rest[++i]; break
					case "--effort": effort = rest[++i]; break
					case "--timeout-minutes": timeoutMinutes = Number(rest[++i]); break
					case "--parent": parent = rest[++i] ?? parent; break
					default: positional.push(arg)
				}
			}
			const prompt = positional.join(" ").trim()
			if (!agent || !prompt) fail("delegate requires --agent <agent> and a prompt\n\n" + usage())

			let modelRef
			if (model) {
				modelRef = parseModelString(model)
				if (!modelRef) fail(`Invalid model "${model}". Expected "provider/model-id".`)
			}
			if (effort) {
				if (!modelRef) fail("effort requires --model together with it.")
				modelRef = { ...modelRef, variant: effortToVariant(effort) }
			}

			const manager = managerFor()
			if (mode === "sync") {
				const events = followEvents((e) => manager.handleEvent(e), { base })
				try {
					const outcome = await manager.delegate({
						parentSessionID: parent,
						prompt,
						agent,
						mode: "sync",
						model: modelRef,
						maxRunTimeMs: timeoutMinutes !== undefined ? timeoutMinutes * 60_000 : undefined,
					})
					if (outcome.mode !== "sync") throw new Error("expected sync")
					console.log(outcome.result || `(${outcome.status}, no output)`)
					return outcome.status === "completed" ? 0 : 2
				} finally {
					events.stop()
				}
			}

			const outcome = await manager.delegate({
				parentSessionID: parent,
				prompt,
				agent,
				mode: "async",
				model: modelRef,
				maxRunTimeMs: timeoutMinutes !== undefined ? timeoutMinutes * 60_000 : undefined,
			})
			if (outcome.mode !== "async") throw new Error("expected async")
			spawnWatcher(parent, outcome.childSessionID, outcome.id, outcome.timeoutMs)
			console.log(
				`Delegation started: ${outcome.id}\nAgent: ${agent}` +
					(modelRef ? `\nModel: ${formatModelRef(modelRef)}` : "") +
					`\nTimeout: ${outcome.timeoutMs > 0 ? `${Math.round(outcome.timeoutMs / 60_000)}min` : "none"}` +
					`\nNotification will arrive in the parent session when complete.`,
			)
			return 0
		}

		case "steer": {
			const [id, ...words] = rest
			const message = words.join(" ").trim()
			if (!id || !message) fail("steer requires <id> <message>")
			const record = await loadRecordByFuzzyId(recordsDir, id)
			if (!record) fail(`No delegation matching "${id}"`)
			console.log(await managerFor().steer(record.parentSessionID, record.id, message))
			return 0
		}

		case "stop": {
			const [id] = rest
			if (!id) fail("stop requires <id>")
			const record = await loadRecordByFuzzyId(recordsDir, id)
			if (!record) fail(`No delegation matching "${id}"`)
			console.log(await managerFor().stopDelegation(record.parentSessionID, record.id))
			return 0
		}

		case "peek": {
			const [id] = rest
			if (!id) fail("peek requires <id>")
			const record = await loadRecordByFuzzyId(recordsDir, id)
			if (!record) fail(`No delegation matching "${id}"`)
			if (record.status === "running") {
				const transcript = await transcriptFromServer(base, record.childSessionID)
				console.log(`## ${record.id} [running]\n\n${transcript.peekDigest()}`)
			} else {
				console.log(`## ${record.id} [${record.status}]\n\n${record.result ?? "(no output)"}`)
			}
			return 0
		}

		case "read": {
			const [id] = rest
			if (!id) fail("read requires <id>")
			const record = await loadRecordByFuzzyId(recordsDir, id)
			if (!record) fail(`No delegation matching "${id}"`)
			if (record.result) {
				console.log(record.result)
			} else if (record.status === "running") {
				const transcript = await transcriptFromServer(base, record.childSessionID)
				console.log(`(still running — latest below)\n\n${transcript.fullText() || "(no output yet)"}`)
			} else {
				console.log(`(no persisted output for ${record.id})`)
			}
			return 0
		}

		case "list": {
			const { listRecords } = await import("./state")
			const records = await listRecords(recordsDir)
			if (records.length === 0) {
				console.log("No delegations recorded.")
				return 0
			}
			for (const r of records) {
				console.log(`- ${r.id} [${r.status}] ${r.agent} · ${new Date(r.startedAt).toISOString()}`)
			}
			return 0
		}

		case "status": {
			const { listRecords } = await import("./state")
			const records = (await listRecords(recordsDir)).filter((r) => r.status === "running")
			if (records.length === 0) {
				console.log("No running delegations.")
				return 0
			}
			for (const r of records) {
				const elapsed = Math.round((Date.now() - r.startedAt) / 1000)
				const left = r.deadlineAt > 0 ? `${Math.max(0, Math.round((r.deadlineAt - Date.now()) / 1000))}s left` : "no deadline"
				console.log(`- ${r.id} (${r.agent}) ${elapsed}s elapsed, ${left}`)
			}
			return 0
		}

		case "watch": {
			const [parent, child, id, timeoutMsArg] = rest
			if (!parent || !child || !id) fail("watch requires <parent> <child> <id> [timeoutMs]")
			return await runWatch(parent, child, id, Number(timeoutMsArg ?? 0), base, recordsDir)
		}

		default:
			fail(`Unknown command "${command}"\n\n${usage()}`)
	}
}

/**
 * Detached watcher for one async delegation: follows the SSE stream for the
 * child session, and when the turn completes (or the deadline hits) posts the
 * <task-notification> synthetic into the parent and updates the record.
 */
async function runWatch(
	parent: string,
	child: string,
	id: string,
	timeoutMs: number,
	base: string,
	recordsDir: string,
): Promise<number> {
	const io = httpSessionIO(base)
	const transcript = new Transcript()
	let finished = false

	const settle = async (status: string, error?: string) => {
		if (finished) return
		finished = true
		events.stop()
		const { listRecords } = await import("./state")
		const records = await listRecords(recordsDir)
		const record = records.find((r) => r.id === id)
		if (!record) process.exit(0)
		record.status = status as DelegationRecord["status"]
		record.completedAt = Date.now()
		record.result = transcript.resultText() || record.result
		if (error) record.error = error
		record.notified = true
		await writeDelegationFile(recordsDir, record)
		const summary =
			record.result && record.result.length > 1_500
				? `${record.result.slice(0, 1_500)}\n…(truncated)`
				: (record.result ?? "(no output)")
		const label = status === "completed" ? "COMPLETE" : status.toUpperCase()
		try {
			await io.synthetic(
				parent,
				`<task-notification> delegation ${id} (${record.agent}) ${label}\n\n${summary}`,
				`delegation ${id} ${label}`,
			)
		} catch {
			// parent gone: record is updated, read still works
		}
		process.exit(0)
	}

	const events = followEvents(
		(event) => {
			const data = (event.data ?? {}) as Record<string, any>
			if (data.sessionID !== child) return
			switch (event.type) {
				case "session.next.text.delta":
				case "session.text.delta":
					if (typeof data.delta === "string") {
						transcript.appendText(String(data.assistantMessageID ?? "text"), data.delta)
					}
					break
				case "session.next.tool.called":
				case "session.tool.called":
					transcript.recordTool(String(data.tool ?? "tool"), "called")
					break
				case "session.next.tool.success":
				case "session.tool.success":
					transcript.recordTool(String(data.tool ?? "tool"), "success")
					break
				case "session.next.tool.failed":
				case "session.tool.failed":
					transcript.recordTool(String(data.tool ?? "tool"), "failed")
					break
				case "session.next.step.ended":
				case "session.step.ended":
					if (data.finish === "stop" || data.finish === "length") {
						setTimeout(() => void settle("completed"), 1_500)
					}
					break
				case "session.execution.succeeded":
					void settle("completed")
					break
				case "session.execution.failed":
					void settle("failed", String(data.error ?? "execution failed"))
					break
				case "session.execution.interrupted":
					void settle("interrupted", "interrupted")
					break
				case "session.idle":
					setTimeout(() => void settle("completed"), 750)
					break
				default:
					break
			}
		},
		{ base },
	)

	if (timeoutMs > 0) {
		setTimeout(async () => {
			try { await io.interrupt(child) } catch { /* already gone */ }
			await settle("timeout", "deadline exceeded")
		}, timeoutMs)
		// A watcher should never outlive its delegation by much.
		setTimeout(() => void settle("timeout", "watcher lifetime exceeded"), timeoutMs + 60_000)
	}
	await events.stopped
	return 0
}

/** Spawns the detached watcher for an async delegation (best-effort). */
function spawnWatcher(parent: string, child: string, id: string, timeoutMs: number): void {
	const self = process.argv[1]
	const args = [self, "watch", parent, child, id, String(timeoutMs)]
	try {
		if (typeof Bun !== "undefined" && Bun.spawn) {
			Bun.spawn([process.execPath, ...args], {
				stdin: "ignore",
				stdout: "ignore",
				stderr: "ignore",
			}).unref?.()
		} else {
			const { spawn } = require("node:child_process") as typeof import("node:child_process")
			spawn(process.execPath, args, { detached: true, stdio: "ignore" }).unref()
		}
	} catch {
		// Notifications are best-effort; delegation_read always works.
	}
}

if (import.meta.main) {
	process.exitCode = await main(process.argv.slice(2))
}
