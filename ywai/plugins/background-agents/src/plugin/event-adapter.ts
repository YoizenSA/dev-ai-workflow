/**
 * v2 → v1 event adapter for the event loop in v2.ts.
 *
 * v2 renamed or re-shaped several server events the DelegationManager reads:
 *
 * - `session.idle` became `session.status` with `status.type: "idle"`;
 * - `message.updated` no longer exists; token telemetry moved to
 *   `session.usage.updated` / `session.step.ended`, which carry no parts.
 *
 * mapV2EventToV1 is additive synthesis only: the first element of the
 * returned array is always the raw input event, unmodified, so handlers
 * tolerant of the v2 shape keep seeing it. Synthesized v1-shape events are
 * appended after it.
 *
 * A synthesized `session.idle` from an idle `session.status` means a v2
 * consumer watching both (this plugin's loop does) handles one idle twice.
 * handleSessionIdle is safe for that: terminal states return early and
 * scheduleComplete cancels and re-arms the debounce timer. New idle
 * consumers must tolerate duplicate delivery the same way.
 */

/** One server event, loosely typed: v1 envelopes use `properties`, v2
 * envelopes may use `data`; both carry a string `type`. */
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

/**
 * v2 usage telemetry (`session.usage.updated` / `session.step.ended`) → a
 * v1 completed-assistant `message.updated` heartbeat.
 *
 * The documented v2 event carries no message identity, so `info.id` is a
 * deterministic fingerprint of the telemetry content: the usage.updated /
 * step.ended pair for one request dedups to a single observation
 * (handleMessageEvent is keyed by session, so replays are harmless, and
 * identical token snapshots collapse), replays stay stable, and genuinely
 * distinct token snapshots stay distinct. No wall clock or randomness — only
 * fields already on the event.
 *
 * Incomplete token blocks are dropped, never mapped into a half-readable
 * shape: the heartbeat path only reads sessionID, and a missing one must not
 * fabricate a heartbeat.
 */
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

/**
 * Map one v2 server event into one or more manager events. Returns
 * `[rawEvent, ...synthesizedV1Shapes]` — the raw event is always first and
 * is never mutated. Synthesis:
 *
 * - idle `session.status` → v1 `session.idle` `{sessionID}`;
 * - usage telemetry → v1 completed-assistant `message.updated` heartbeat.
 *
 * `session.created` needs no synthesis here: the manager reads delegation
 * sessions through the client facade, not through early-registration
 * events, and the v2 loop in v2.ts only consumes idle and message events.
 */
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
