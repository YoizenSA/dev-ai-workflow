/**
 * Terminal-event RPC contract for background-agents human notifications.
 *
 * On v2 the facade's `tui.showToast` is a no-op and the built-in
 * `opencode.notifications` plugin suppresses the OS notification for
 * subagents, so finished background subagents are silent for the human. When the host
 * supports plugin RPC, setup registers the `delegation_terminal` event; the
 * TUI sidecar (ywai/plugins/tui/background-agents-notify.tsx) subscribes and
 * raises an OS notification (only when unfocused) plus a native toast.
 *
 * Plain JSON-Schema literal on purpose: Rpc.define is a structural identity
 * check, so a portable `{id, methods, events}` object registers without
 * importing @opencode/* into the server bundle.
 */

export const TERMINAL_RPC_ID = "ywai-background-agents"
export const TERMINAL_EVENT_NAME = "delegation_terminal"

/** One background subagent reached a terminal status (complete/error/stopped/timeout). delegationID = its sessionID. */
export interface DelegationTerminalEventData {
	kind: "terminal"
	delegationID: string
	agent: string
	status: string
	sessionID: string
	parentSessionID: string
	/** Siblings still running under the same parent; 0 = this was the last one. */
	remaining: number
	title?: string
	error?: string
}

export type BackgroundAgentsTerminalEvent = DelegationTerminalEventData


export const BackgroundAgentsTerminalRpc = {
	id: TERMINAL_RPC_ID,
	methods: {},
	events: {
		[TERMINAL_EVENT_NAME]: {
			schema: {
				type: "object",
				properties: {
					kind: { type: "string" },
					delegationID: { type: "string" },
					agent: { type: "string" },
					status: { type: "string" },
					sessionID: { type: "string" },
					parentSessionID: { type: "string" },
					remaining: { type: "integer" },
					title: { type: "string" },
					error: { type: "string" },
				},
				required: ["kind", "parentSessionID"],
				additionalProperties: false,
			},
		},
	},
} as const

export interface TerminalRpcRegistration {
	events: {
		emit: (name: string, data: unknown) => Promise<unknown>
	}
	dispose?: () => unknown
}

/** Register the terminal-event RPC; undefined (never throws) without a host rpc surface. */
export async function tryRegisterTerminalRpc(ctx: unknown): Promise<TerminalRpcRegistration | undefined> {
	try {
		const rpc = (ctx as { rpc?: { register?: unknown } } | null | undefined)?.rpc
		if (!rpc || typeof rpc.register !== "function") return undefined
		const registration = (await (rpc.register as (def: unknown, handlers: Record<string, never>) => unknown)(
			BackgroundAgentsTerminalRpc,
			{},
		)) as TerminalRpcRegistration | null | undefined
		if (!registration || typeof registration.events?.emit !== "function") return undefined
		return registration
	} catch {
		return undefined
	}
}
