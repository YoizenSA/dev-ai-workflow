/**
 * v2 -> v1 event adapter for ywai plugins.
 *
 * v2 renamed or re-shaped several server events:
 * - `session.idle` became `session.status` with `status.type: "idle"`;
 * - `message.updated` no longer exists; token telemetry moved to
 *   `session.usage.updated` / `session.step.ended`.
 *
 * mapV2EventToV1 is additive synthesis only: the raw event always comes first,
 * unmodified. Synthesized v1-shape events follow.
 */

export interface PluginEvent {
	type?: string
	properties?: Record<string, any>
	data?: Record<string, any>
	[key: string]: any
}

function propsOf(event: PluginEvent): Record<string, any> {
	const props = event.properties ?? event.data ?? {}
	return props && typeof props === "object" ? props : {}
}

function finiteNumber(value: unknown): number | undefined {
	return typeof value === "number" && Number.isFinite(value) ? value : undefined
}

export function usageToMessageUpdated(props: Record<string, any>): PluginEvent | undefined {
	const sessionID = props.sessionID
	if (typeof sessionID !== "string" || sessionID === "") return undefined
	const tokens = props.tokens
	if (!tokens || typeof tokens !== "object") return undefined
	const cache = tokens.cache && typeof tokens.cache === "object" ? tokens.cache : undefined
	const input = finiteNumber(tokens.input)
	const cacheRead = finiteNumber(cache?.read)
	const cacheWrite = finiteNumber(cache?.write)
	if (input === undefined || cacheRead === undefined || cacheWrite === undefined) return undefined
	const output = finiteNumber(tokens.output) ?? 0
	const reasoning = finiteNumber(tokens.reasoning) ?? 0
	const id = `v2-usage:${sessionID}:${input}:${output}:${cacheRead}:${cacheWrite}`
	const completedAt = finiteNumber(props.timestamp) ?? finiteNumber(props.activityAt) ?? 0
	return {
		type: "message.updated",
		properties: {
			info: {
				id,
				role: "assistant",
				sessionID,
				time: { completed: completedAt },
				tokens: { input, output, reasoning, cache: { read: cacheRead, write: cacheWrite } },
			},
		},
	}
}

export function mapV2EventToV1(event: PluginEvent): PluginEvent[] {
	const out: PluginEvent[] = [event]
	const type = typeof event.type === "string" ? event.type : ""
	const props = propsOf(event)

	if (type === "session.status") {
		const status = props.status
		const statusType = status && typeof status === "object" ? status.type : undefined
		if (statusType === "idle" && typeof props.sessionID === "string") {
			out.push({ type: "session.idle", properties: { sessionID: props.sessionID } })
		}
		return out
	}

	if (type === "session.usage.updated" || type === "session.step.ended") {
		const mapped = usageToMessageUpdated(props)
		if (mapped) out.push(mapped)
	}

	return out
}
