/**
 * advisor — v2 promise-plugin setup.
 *
 * Runs the advisor review loop against the OpenCode v2 promise-plugin context:
 * - Watches session.idle (synthesized by mapV2EventToV1) and runs reviews
 * - Delivers advice via session.prompt({ noReply: true }), routed through
 *   ctx.session.synthetic so it is visible without spending a turn
 * - Replaces event.system for advisor-owned review sessions via ctx.session.hook("context")
 * - Registers advisor_status, advisor_set_model, advisor_toggle tools via ctx.tool.transform
 */

import { CONFIG_PATH, loadConfig, loadWatchdog, toastVariant } from "./config"
import { setModel, status, toggle } from "./controls"
import { fetchMessages } from "./messages"
import { SessionCursor, renderDelta, worthReviewing } from "./delta"
import { EmissionGuard, type Severity } from "./emission-guard"
import { ADVISOR_SYSTEM_PROMPT, buildAdvisorPrompt, parseVerdict, renderAdvisory } from "./verdict"
import {
	createV1ShapedClient,
	mapV2EventToV1,
	type V2PluginContext,
	type V2SessionContextEvent,
} from "../../shared/v2"

const ADVISOR_AGENT = "advisor"

type SessionState = {
	cursor: SessionCursor
	guard: EmissionGuard
	running: boolean
}

async function runAdvisorV2(
	client: any,
	parentID: string,
	model: any,
	delta: string,
	watchdog: string | undefined,
	ownSessions: Set<string>,
): Promise<string> {
	const created = await client.session.create({ body: { parentID, title: "advisor" } })
	const sessionID = created?.data?.id ?? created?.id
	if (!sessionID) return ""
	ownSessions.add(sessionID)

	try {
		const res = await client.session.prompt({
			path: { id: sessionID },
			body: {
				model,
				agent: ADVISOR_AGENT,
				system: ADVISOR_SYSTEM_PROMPT,
				tools: { "*": false },
				parts: [{ type: "text", text: buildAdvisorPrompt(delta, watchdog) }],
			},
		})
		const parts = res?.data?.parts ?? res?.parts ?? []
		const text = parts
			.filter((p: any) => p && (p.type === "text" || typeof p.text === "string"))
			.map((p: any) => p.text ?? "")
			.join("\n")
		return text
	} finally {
		ownSessions.delete(sessionID)
		try {
			await client.session.delete({ path: { id: sessionID } })
		} catch {
			// ignore
		}
	}
}

async function deliverV2(
	client: any,
	sessionID: string,
	severity: Severity,
	note: string,
): Promise<void> {
	await client.session.prompt({
		path: { id: sessionID },
		body: {
			noReply: true,
			parts: [{ type: "text", text: renderAdvisory(severity, note) }],
		},
	})
}

export async function setupV2(ctx: V2PluginContext): Promise<(() => void) | undefined> {
	const directory = ctx.location?.directory ?? process.cwd()
	const config = await loadConfig(CONFIG_PATH)

	// Register tools via ctx.tool.transform
	if (typeof ctx.tool?.transform === "function") {
		await ctx.tool.transform((editor) => {
			editor.add({
				name: "advisor_status",
				description: "Report whether the advisor is enabled and which model it reviews with.",
				input: { type: "object", properties: {}, additionalProperties: false },
				execute: async () => {
					const res = await status(CONFIG_PATH)
					return typeof res === "string" ? res : JSON.stringify(res)
				},
			})
			editor.add({
				name: "advisor_set_model",
				description: "Set the model the advisor reviews with. Requires provider/model.",
				input: {
					type: "object",
					properties: { model: { type: "string", description: "provider/model" } },
					required: ["model"],
					additionalProperties: false,
				},
				execute: async (args: { model: string }) => {
					return (await setModel(args.model, CONFIG_PATH)).message
				},
			})
			editor.add({
				name: "advisor_toggle",
				description: "Turn the advisor on or off for future sessions.",
				input: {
					type: "object",
					properties: { enabled: { type: "boolean", description: "true to enable, false to disable" } },
					required: ["enabled"],
					additionalProperties: false,
				},
				execute: async (args: { enabled: boolean }) => {
					return (await toggle(args.enabled, CONFIG_PATH)).message
				},
			})
		})
	}

	if (!config.enabled || !config.model) {
		return undefined
	}

	const client = createV1ShapedClient(ctx)
	const model = config.model
	const watchdog = await loadWatchdog(directory)
	const sessions = new Map<string, SessionState>()
	const ownSessions = new Set<string>()

	// Replace system prompt for advisor's own sessions
	if (typeof ctx.session?.hook === "function") {
		await ctx.session.hook("context", async (event: V2SessionContextEvent) => {
			if (!event.sessionID || !ownSessions.has(event.sessionID)) return
			event.system = [{ type: "text", text: ADVISOR_SYSTEM_PROMPT }]
		})
	}

	async function review(sessionID: string): Promise<void> {
		const state = sessions.get(sessionID)
		if (!state || state.running) return
		state.running = true
		try {
			const history = await fetchMessages(client, sessionID)
			const delta = state.cursor.take(history)
			if (delta.reset) state.guard.reset()
			if (!worthReviewing(delta.messages)) return

			state.guard.beginUpdate()
			const reply = await runAdvisorV2(client, sessionID, model, renderDelta(delta.messages), watchdog, ownSessions)
			const verdict = parseVerdict(reply)
			if (verdict.severity === "silent") return

			const admitted = state.guard.admit(verdict.note, verdict.severity)
			if (!admitted.accepted) return

			await deliverV2(client, sessionID, admitted.severity, admitted.note)
		} catch {
			// Advisor errors must never break the reviewed session
		} finally {
			state.running = false
		}
	}

	const controller = new AbortController()
	void (async () => {
		try {
			for await (const raw of ctx.event.subscribe({ signal: controller.signal })) {
				for (const evt of mapV2EventToV1(raw ?? {})) {
					const type = evt.type ?? ""
					const props = evt.properties ?? evt.data ?? {}
					const sessionID = props.sessionID ?? props.info?.sessionID
					if (!sessionID || ownSessions.has(sessionID)) continue

					if (type === "session.idle") {
						if (!sessions.has(sessionID)) {
							const history = await fetchMessages(client, sessionID)
							sessions.set(sessionID, {
								cursor: new SessionCursor(history.length),
								guard: new EmissionGuard(),
								running: false,
							})
							continue
						}
						await review(sessionID)
						continue
					}

					if (type === "session.compacted") {
						sessions.get(sessionID)?.cursor.reset()
						sessions.get(sessionID)?.guard.reset()
						continue
					}

					if (type === "session.deleted") {
						sessions.delete(sessionID)
					}
				}
			}
		} catch {
			// stream ended
		}
	})()

	return () => controller.abort()
}
