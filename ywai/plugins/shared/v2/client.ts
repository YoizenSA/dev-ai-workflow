/**
 * v1-shaped OpencodeClient facade over the v2 plugin context.
 *
 * Translates v1 SDK methods (Hono-style {path, body}) into v2 flat session
 * calls (prompt, switchModel, switchAgent, interrupt, wait, etc.).
 *
 * Supports `noReply: true` by routing through `session.synthetic` when present,
 * or falling back to a steer prompt. Probes host capabilities defensively
 * and logs honest degradation instead of throwing.
 */

import type { V2PluginContext } from "./types"

/** Log a v2 shim degradation message to stderr in debug environments. */
function shimLog(message: string, details?: Record<string, any>): void {
	if (process.env.DEBUG || process.env.YWAI_DEBUG) {
		const extra = details ? ` ${JSON.stringify(details)}` : ""
		process.stderr.write(`[ywai-v2-shim] ${message}${extra}\n`)
	}
}

/** Probed capabilities of an OpenCode v2 host context. */
export interface V2Capabilities {
	sessionGet: boolean
	sessionCreate: boolean
	sessionPrompt: boolean
	sessionSynthetic: boolean
	sessionSwitchModel: boolean
	sessionSwitchAgent: boolean
	sessionInterrupt: boolean
	sessionContext: boolean
	sessionWait: boolean
	sessionDelete: boolean
	sessionHook: boolean
	toolTransform: boolean
	providerList: boolean
	agentList: boolean
}

/** Probes the available capabilities on a v2 plugin context. */
export function probeV2Capabilities(ctx: V2PluginContext): V2Capabilities {
	const s = ctx?.session
	return {
		sessionGet: typeof s?.get === "function",
		sessionCreate: typeof s?.create === "function",
		sessionPrompt: typeof s?.prompt === "function",
		sessionSynthetic: typeof s?.synthetic === "function",
		sessionSwitchModel: typeof s?.switchModel === "function",
		sessionSwitchAgent: typeof s?.switchAgent === "function",
		sessionInterrupt: typeof s?.interrupt === "function",
		sessionContext: typeof s?.context === "function",
		sessionWait: typeof s?.wait === "function",
		sessionDelete: typeof s?.delete === "function",
		sessionHook: typeof s?.hook === "function",
		toolTransform: typeof ctx?.tool?.transform === "function",
		providerList: typeof ctx?.provider?.list === "function",
		agentList: typeof ctx?.agent?.list === "function",
	}
}

/** v1 SDK responses are {data}-wrapped; some v2 ctx returns may be too. */
export function unwrap(value: any): any {
	return value && typeof value === "object" && !Array.isArray(value) && "data" in value ? value.data : value
}

/** First array hidden inside a client-style envelope. */
export function toArray(value: any): any[] {
	const inner = unwrap(value)
	if (Array.isArray(inner)) return inner
	return inner?.agents ?? inner?.sessions ?? inner?.items ?? inner?.all ?? []
}

export function createV1ShapedClient(ctx: V2PluginContext): any {
	const caps = probeV2Capabilities(ctx)

	const session = {
		async status(): Promise<{ data: undefined }> {
			return { data: undefined }
		},

		async get(input: { path: { id: string } }): Promise<{ data: any }> {
			if (caps.sessionGet && ctx.session.get) {
				return { data: unwrap(await ctx.session.get({ sessionID: input.path.id })) }
			}
			shimLog("session.get not supported on this host", { id: input?.path?.id })
			return { data: undefined }
		},

		async create(input: { body?: { title?: string; parentID?: string } }): Promise<{ data: any }> {
			if (caps.sessionCreate && ctx.session.create) {
				return {
					data: unwrap(
						await ctx.session.create({ title: input.body?.title, parentID: input.body?.parentID }),
					),
				}
			}
			shimLog("session.create not supported; returning synthetic ID")
			return { data: { id: `v2-session-${Date.now()}` } }
		},

		async promptAsync(input: {
			path: { id: string }
			body?: { agent?: string; parts?: Array<{ type: string; text?: string }> }
		}): Promise<{ data: undefined }> {
			const text = (input.body?.parts ?? [])
				.filter((part) => part.type === "text")
				.map((part) => part.text ?? "")
				.join("\n")
			if (input.body?.agent) {
				if (caps.sessionSwitchAgent && ctx.session.switchAgent) {
					await ctx.session.switchAgent({ sessionID: input.path.id, agent: input.body.agent })
				} else {
					shimLog("session.switchAgent not supported", { agent: input.body.agent })
				}
			}
			if (caps.sessionPrompt && ctx.session.prompt) {
				await ctx.session.prompt({
					sessionID: input.path.id,
					text,
					delivery: "steer",
				})
			} else {
				shimLog("session.prompt not supported on promptAsync")
			}
			return { data: undefined }
		},

		async abort(input: { path: { id: string } }): Promise<{ data: undefined }> {
			if (caps.sessionInterrupt && ctx.session.interrupt) {
				await ctx.session.interrupt({ sessionID: input.path.id })
			} else {
				shimLog("session.interrupt not supported", { id: input?.path?.id })
			}
			return { data: undefined }
		},

		async delete(input: { path: { id: string } }): Promise<{ data: undefined }> {
			if (caps.sessionDelete && ctx.session.delete) {
				try {
					await ctx.session.delete({ sessionID: input.path.id })
				} catch {
					// best-effort cleanup
				}
			}
			return { data: undefined }
		},

		async messages(input: { path: { id: string } }): Promise<{ data: any }> {
			if (caps.sessionContext && ctx.session.context) {
				const items = toArray(await ctx.session.context({ sessionID: input.path.id }))
				return {
					data: items.map((item: any) => {
						if (!item || typeof item !== "object") return item
						const role = item.info?.role ?? item.message?.role ?? item.role ?? item.type
						const parts = item.parts ?? item.content ?? item.info?.parts ?? []
						return { ...item, info: { ...item, role }, parts }
					}),
				}
			}
			shimLog("session.context not supported; returning empty messages", { id: input?.path?.id })
			return { data: [] }
		},

		async prompt(input: {
			path: { id: string }
			body?: {
				agent?: string
				model?: { providerID: string; modelID: string; variant?: string }
				noReply?: boolean
				system?: string
				tools?: Record<string, any>
				parts?: Array<Record<string, any>>
			}
		}): Promise<{ data: { parts: any[] } }> {
			const body = input.body ?? {}
			const textParts = (body.parts ?? [])
				.filter((part) => part.type === "text" && typeof part.text === "string")
				.map((part) => part.text)
			const fileParts = (body.parts ?? [])
				.filter((part) => part.type === "file")
				.map((part) => ({
					uri: part.url ?? part.uri ?? "",
					name: part.filename ?? part.name,
					mime: part.mime,
				}))

			const text = textParts.join("\n")

			// Route noReply: true through session.synthetic when available,
			// so the message is visible in transcript without spending a turn.
			if (body.noReply) {
				if (caps.sessionSynthetic && ctx.session.synthetic) {
					await ctx.session.synthetic({
						sessionID: input.path.id,
						text,
					})
					return { data: { parts: [] } }
				}
				// Fallback: steer delivery without expecting response parts
				if (caps.sessionPrompt && ctx.session.prompt) {
					await ctx.session.prompt({
						sessionID: input.path.id,
						text,
						delivery: "steer",
					})
				} else {
					shimLog("neither session.synthetic nor session.prompt available for noReply")
				}
				return { data: { parts: [] } }
			}

			if (body.agent) {
				if (caps.sessionSwitchAgent && ctx.session.switchAgent) {
					await ctx.session.switchAgent({ sessionID: input.path.id, agent: body.agent })
				} else {
					shimLog("session.switchAgent not supported; keeping current agent", { agent: body.agent })
				}
			}

			if (body.model) {
				if (caps.sessionSwitchModel && ctx.session.switchModel) {
					await ctx.session.switchModel({
						sessionID: input.path.id,
						model: {
							providerID: body.model.providerID,
							id: body.model.modelID,
							...(body.model.variant ? { variant: body.model.variant } : {}),
						},
					})
				} else {
					shimLog("session.switchModel not supported; keeping default model", { model: body.model })
				}
			}

			if (caps.sessionPrompt && ctx.session.prompt) {
				const promptArgs: Record<string, any> = {
					sessionID: input.path.id,
					text,
					delivery: "steer",
				}
				if (fileParts.length > 0) {
					promptArgs.files = fileParts
				}
				await ctx.session.prompt(promptArgs)
			} else {
				shimLog("session.prompt not supported on this host")
			}

			if (caps.sessionWait && ctx.session.wait) {
				try {
					await ctx.session.wait({ sessionID: input.path.id })
				} catch {
					// best-effort
				}
			}

			// Try to recover assistant response text from context
			if (caps.sessionContext && ctx.session.context) {
				try {
					const items = toArray(await ctx.session.context({ sessionID: input.path.id }))
					const lastAssistant = [...items].reverse().find((m: any) => {
						const role = m.info?.role ?? m.message?.role ?? m.role ?? m.type
						return role === "assistant"
					})
					if (lastAssistant) {
						const parts = lastAssistant.parts ?? lastAssistant.content ?? lastAssistant.info?.parts ?? []
						return { data: { parts } }
					}
				} catch {
					// fallback to empty
				}
			}

			return { data: { parts: [] } }
		},
	}

	const provider = {
		async list(): Promise<{ data: { all: any[] } }> {
			if (caps.providerList && ctx.provider?.list) {
				try {
					const result = await ctx.provider.list()
					return { data: { all: toArray(result) } }
				} catch {
					return { data: { all: [] } }
				}
			}
			return { data: { all: [] } }
		},
	}

	const app = {
		async log(): Promise<{ data: undefined }> {
			return { data: undefined }
		},
		async agents() {
			if (caps.agentList && ctx.agent?.list) {
				try {
					const agents = toArray(await ctx.agent.list())
					return {
						data: agents.map((agent: any) => ({
							name: agent.id ?? agent.name,
							description: agent.description,
							mode: agent.mode,
						})),
					}
				} catch {
					return { data: [] }
				}
			}
			return { data: [] }
		},
	}

	return {
		session,
		provider,
		app,
		config: {
			async get(): Promise<{ data: undefined }> {
				return { data: undefined }
			},
		},
		tui: {
			async showToast(): Promise<void> {},
		},
	}
}
