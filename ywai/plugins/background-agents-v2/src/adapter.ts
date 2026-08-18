/**
 * Adapter over the v2 plugin Context: the small session/event surface the
 * manager needs, with the wire shapes validated against the v2 beta runtime
 * (0.0.0-beta-202608110357):
 *
 *   create    POST /api/session            {agent?, model?, title?}
 *   prompt    POST /api/session/{id}/prompt   {prompt: {text, delivery?}}
 *   synthetic POST /api/session/{id}/synthetic {prompt: {text, description?}}
 *   interrupt POST /api/session/{id}/interrupt
 *   events    SSE  /api/event — in-process via ctx.event.subscribe
 *
 * ctx.session.wait exists in the API surface but the beta runtime answers
 * 503 "not available yet", so sync completion is event-driven (see manager).
 */

import type { ModelRef } from "./state"

export interface CreateSessionInput {
	agent?: string
	model?: ModelRef
	title?: string
}

export interface SessionIO {
	create(input: CreateSessionInput): Promise<string>
	prompt(sessionID: string, text: string, delivery?: "steer" | "queue"): Promise<void>
	synthetic(sessionID: string, text: string, description?: string): Promise<void>
	interrupt(sessionID: string): Promise<void>
}

/** A v2 server event, as delivered to plugins: {id, type, data, location?}. */
export interface V2Event {
	type: string
	data?: Record<string, unknown>
}

interface SessionDomainShape {
	create(input: Record<string, unknown>): Promise<{ id?: string }>
	prompt(input: Record<string, unknown>): Promise<unknown>
	synthetic(input: Record<string, unknown>): Promise<unknown>
	interrupt(input: Record<string, unknown>): Promise<unknown>
}

/** Builds the SessionIO from the plugin Context's session domain. */
function createSessionIO(session: SessionDomainShape): SessionIO {
	return {
		async create(input) {
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
			const created = await session.create(body)
			if (!created?.id) throw new Error("session.create returned no id")
			return created.id
		},
		async prompt(sessionID, text, delivery) {
			// The plugin API's `text` field maps to the wire's `prompt` body.
			const body: Record<string, unknown> = { text }
			if (delivery) body.delivery = delivery
			await session.prompt({ sessionID, ...body })
		},
		async synthetic(sessionID, text, description) {
			const body: Record<string, unknown> = { text }
			if (description) body.description = description
			await session.synthetic({ sessionID, ...body })
		},
		async interrupt(sessionID) {
			await session.interrupt({ sessionID })
		},
	}
}

/** Parses "provider/model-id" (and "provider/model-id#variant") into a ref. */
export function parseModelString(value: string): ModelRef | undefined {
	const [ref, variant] = value.trim().split("#")
	const [providerID, ...modelSegments] = ref.split("/")
	const id = modelSegments.join("/")
	if (!providerID || !id) return undefined
	return variant ? { providerID, id, variant } : { providerID, id }
}

/** Renders a ref back to "provider/model#variant" for display. */
export function formatModelRef(ref: ModelRef): string {
	return `${ref.providerID}/${ref.id}${ref.variant ? `#${ref.variant}` : ""}`
}

const EFFORT_ALIASES: Record<string, string> = {
	max: "high",
	minimal: "low",
	min: "low",
}

/** Normalizes an effort selector to a model variant, with light aliases. */
export function effortToVariant(effort: string): string {
	const e = effort.trim().toLowerCase()
	return EFFORT_ALIASES[e] ?? e
}

export { createSessionIO }
export type { SessionDomainShape }
