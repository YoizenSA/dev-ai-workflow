/**
 * v1-shaped OpencodeClient facade over the v2 plugin context.
 *
 * Translates v1 SDK methods (Hono-style {path, body}) into v2 flat session
 * calls (prompt, switchModel, switchAgent, interrupt, wait, etc.).
 *
 * Supports `noReply: true` by routing through `session.synthetic` when present,
 * or falling back to a steer prompt.
 */

import type { V2PluginContext } from "./types"

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
	const session = {
		async status(): Promise<{ data: undefined }> {
			return { data: undefined }
		},

		async get(input: { path: { id: string } }): Promise<{ data: any }> {
			if (typeof ctx.session.get === "function") {
				return { data: unwrap(await ctx.session.get({ sessionID: input.path.id })) }
			}
			return { data: undefined }
		},

		async create(input: { body?: { title?: string; parentID?: string } }): Promise<{ data: any }> {
			if (typeof ctx.session.create === "function") {
				return {
					data: unwrap(
						await ctx.session.create({ title: input.body?.title, parentID: input.body?.parentID }),
					),
				}
			}
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
			if (input.body?.agent && typeof ctx.session.switchAgent === "function") {
				await ctx.session.switchAgent({ sessionID: input.path.id, agent: input.body.agent })
			}
			if (typeof ctx.session.prompt === "function") {
				await ctx.session.prompt({
					sessionID: input.path.id,
					text,
					delivery: "steer",
				})
			}
			return { data: undefined }
		},

		async abort(input: { path: { id: string } }): Promise<{ data: undefined }> {
			if (typeof ctx.session.interrupt === "function") {
				await ctx.session.interrupt({ sessionID: input.path.id })
			}
			return { data: undefined }
		},

		async delete(input: { path: { id: string } }): Promise<{ data: undefined }> {
			if (typeof ctx.session.delete === "function") {
				try {
					await ctx.session.delete({ sessionID: input.path.id })
				} catch {
					// best-effort
				}
			}
			return { data: undefined }
		},

		async messages(input: { path: { id: string } }): Promise<{ data: any }> {
			if (typeof ctx.session.context === "function") {
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
				if (typeof ctx.session.synthetic === "function") {
					await ctx.session.synthetic({
						sessionID: input.path.id,
						text,
					})
					return { data: { parts: [] } }
				}
				// Fallback: steer delivery without expecting response parts
				if (typeof ctx.session.prompt === "function") {
					await ctx.session.prompt({
						sessionID: input.path.id,
						text,
						delivery: "steer",
					})
				}
				return { data: { parts: [] } }
			}

			if (body.agent && typeof ctx.session.switchAgent === "function") {
				await ctx.session.switchAgent({ sessionID: input.path.id, agent: body.agent })
			}

			if (body.model && typeof ctx.session.switchModel === "function") {
				await ctx.session.switchModel({
					sessionID: input.path.id,
					model: {
						providerID: body.model.providerID,
						id: body.model.modelID,
						...(body.model.variant ? { variant: body.model.variant } : {}),
					},
				})
			}

			if (typeof ctx.session.prompt === "function") {
				const promptArgs: Record<string, any> = {
					sessionID: input.path.id,
					text,
					delivery: "steer",
				}
				if (fileParts.length > 0) {
					promptArgs.files = fileParts
				}
				await ctx.session.prompt(promptArgs)
			}

			if (typeof ctx.session.wait === "function") {
				try {
					await ctx.session.wait({ sessionID: input.path.id })
				} catch {
					// best-effort
				}
			}

			// Try to recover assistant response text from context
			if (typeof ctx.session.context === "function") {
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
			if (typeof ctx.provider?.list === "function") {
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
			if (typeof ctx.agent?.list === "function") {
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
