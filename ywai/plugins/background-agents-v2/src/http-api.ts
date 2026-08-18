/**
 * HTTP client for the OpenCode v2 server API — the stable contract every
 * front (CLI, MCP server, watcher) drives. Wire shapes validated against
 * the v2 beta/dev runtimes:
 *
 *   POST /api/session                        {agent?, model?, title?}
 *   POST /api/session/{id}/prompt            {prompt: {text, delivery?}}
 *   POST /api/session/{id}/synthetic         {prompt: {text, description?}}
 *   POST /api/session/{id}/interrupt
 *   GET  /api/event (SSE)                    data: {type, data: {...}}
 *
 * OPENCODE_URL selects the server (default http://127.0.0.1:4096, matching
 * ywai's convention).
 */

import type { CreateSessionInput, SessionIO } from "./adapter"
import type { V2Event } from "./adapter"

export function serverBaseURL(): string {
	return (process.env.OPENCODE_URL ?? "http://127.0.0.1:4096").replace(/\/+$/, "")
}

async function postJSON(base: string, path: string, body: unknown): Promise<unknown> {
	const res = await fetch(`${base}${path}`, {
		method: "POST",
		headers: { "content-type": "application/json" },
		body: JSON.stringify(body),
	})
	if (!res.ok) {
		const text = await res.text().catch(() => "")
		throw new Error(`${path} → ${res.status}: ${text.slice(0, 300)}`)
	}
	const text = await res.text()
	return text ? JSON.parse(text) : undefined
}

/** SessionIO implemented over the v2 HTTP API. */
export function httpSessionIO(base = serverBaseURL()): SessionIO {
	return {
		async create(input: CreateSessionInput) {
			const body: Record<string, unknown> = {}
			if (input.agent) body.agent = input.agent
			if (input.model) {
				body.model = {
					id: input.model.id,
					providerID: input.model.providerID,
					...(input.model.variant ? { variant: input.model.variant } : {}),
				}
			}
			if (input.title) body.title = input.title
			const created = (await postJSON(base, "/api/session", body)) as {
				data?: { id?: string }
				id?: string
			}
			const id = created?.data?.id ?? created?.id
			if (!id) throw new Error(`session.create returned no id: ${JSON.stringify(created).slice(0, 200)}`)
			return id
		},
		async prompt(sessionID, text, delivery) {
			const body: Record<string, unknown> = { prompt: { text } }
			if (delivery) (body.prompt as Record<string, unknown>).delivery = delivery
			await postJSON(base, `/api/session/${encodeURIComponent(sessionID)}/prompt`, body)
		},
		async synthetic(sessionID, text, description) {
			const body: Record<string, unknown> = { prompt: { text } }
			if (description) (body.prompt as Record<string, unknown>).description = description
			await postJSON(base, `/api/session/${encodeURIComponent(sessionID)}/synthetic`, body)
		},
		async interrupt(sessionID) {
			await postJSON(base, `/api/session/${encodeURIComponent(sessionID)}/interrupt`, {})
		},
	}
}

/**
 * Follows the server's SSE event stream until stop() is called, delivering
 * parsed events to onEvent. Reconnects with a short backoff so a server
 * restart cannot silently blind a sync wait or a watcher.
 */
export function followEvents(
	onEvent: (event: V2Event) => void,
	opts: { base?: string; signal?: AbortSignal } = {},
): { stop: () => void; stopped: Promise<void> } {
	const base = opts.base ?? serverBaseURL()
	const controller = new AbortController()
	const onAbort = () => controller.abort()
	opts.signal?.addEventListener("abort", onAbort)
	let stopped = false

	const run = async () => {
		let backoffMs = 250
		while (!stopped && !controller.signal.aborted) {
			try {
				const res = await fetch(`${base}/api/event`, {
					headers: { accept: "text/event-stream" },
					signal: controller.signal,
				})
				if (!res.ok || !res.body) throw new Error(`SSE /api/event → ${res.status}`)
				backoffMs = 250
				const reader = res.body.getReader()
				const decoder = new TextDecoder()
				let buffer = ""
				while (!stopped) {
					const { done, value } = await reader.read()
					if (done) break
					buffer += decoder.decode(value, { stream: true })
					let index: number
					while ((index = buffer.indexOf("\n")) >= 0) {
						const line = buffer.slice(0, index).trim()
						buffer = buffer.slice(index + 1)
						if (!line.startsWith("data:")) continue
						try {
							onEvent(JSON.parse(line.slice(5).trim()) as V2Event)
						} catch {
							// keep-alive comments and partial frames: skip
						}
					}
				}
			} catch (error) {
				if (stopped || controller.signal.aborted) break
				void error
				await new Promise((r) => setTimeout(r, backoffMs))
				backoffMs = Math.min(backoffMs * 2, 5_000)
			}
		}
	}

	const stoppedPromise = run()
	return {
		stop() {
			stopped = true
			controller.abort()
			opts.signal?.removeEventListener("abort", onAbort)
		},
		stopped: stoppedPromise,
	}
}
