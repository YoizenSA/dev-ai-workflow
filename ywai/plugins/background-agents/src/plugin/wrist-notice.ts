import * as fs from "node:fs"
import * as os from "node:os"
import * as path from "node:path"
import type { BackgroundAgentsTerminalEvent } from "./terminal-events"

/** JSON body of POST /api/push/notify. The control server turns it into the glance. */
export interface WristNotice {
	kind: "done" | "failed" | "waiting"
	agent: string
	label: string
	detail?: string
	remaining?: number
}

const DEFAULT_CONTROL_URL = "http://127.0.0.1:5768"

export interface WristConfig {
	enabled: boolean
	url: string
}

/** Where a glance is posted. Off, or a missing file, posts nowhere. */
export function wristTarget(config: WristConfig | null | undefined): string | null {
	if (!config?.enabled) return null
	const url = (config.url || DEFAULT_CONTROL_URL).trim().replace(/\/$/, "")
	return url || null
}

export function readWristConfig(file = path.join(os.homedir(), ".ywai", "wrist.json")): WristConfig {
	const off: WristConfig = { enabled: false, url: DEFAULT_CONTROL_URL }
	try {
		const doc = JSON.parse(fs.readFileSync(file, "utf8")) as { enabled?: unknown; url?: unknown }
		return {
			enabled: doc.enabled === true,
			url: typeof doc.url === "string" && doc.url.trim() ? doc.url.trim().replace(/\/$/, "") : DEFAULT_CONTROL_URL,
		}
	} catch {
		return off
	}
}

/** One wrist glance for a delegation that finished. The batch-complete event is a second buzz, so it is dropped. */
export function noticeFromTerminal(event: BackgroundAgentsTerminalEvent): WristNotice | null {
	if (event.kind !== "terminal") return null
	const agent = event.agent?.trim()
	if (!agent) return null
	const kind = event.status === "complete" ? "done" : terminalFailure(event.status)
	if (!kind) return null
	const title = event.title?.trim()
	const about = event.description?.trim()
	const reason = kind === "failed" ? event.error?.trim() : ""
	const detail = [about, reason].filter((part) => part).join(" — ")
	return {
		kind,
		agent,
		label: title || event.delegationID,
		detail: detail || undefined,
		remaining: event.remaining > 0 ? event.remaining : undefined,
	}
}

function terminalFailure(status: string): "failed" | null {
	if (status === "error" || status === "cancelled" || status === "timeout") return "failed"
	return null
}

export function deliverTerminalGlance(
	event: BackgroundAgentsTerminalEvent,
	post: (notice: WristNotice) => void = postWristNotice,
): void {
	const notice = noticeFromTerminal(event)
	if (notice) post(notice)
}

/** Fire-and-forget. Off, or a down control server, must not break the delegation path. */
export function postWristNotice(
	notice: WristNotice,
	fetchImpl: typeof fetch = fetch,
	config: WristConfig | null = readWristConfig(),
): void {
	const base = wristTarget(config)
	if (!base) return
	void Promise.resolve()
		.then(() =>
			fetchImpl(`${base}/api/push/notify`, {
				method: "POST",
				headers: { "content-type": "application/json" },
				body: JSON.stringify(notice),
			}),
		)
		.catch(() => {})
}
