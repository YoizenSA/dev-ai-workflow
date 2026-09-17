/**
 * Permission gate (PLAN 6.4).
 *
 * One rule, and it is the whole point: Jev may only HARDEN. allow -> ask is
 * the single transition this file is allowed to make. It never turns an `ask`
 * into an `allow`, never touches a `deny`, and never acts on a decision that
 * did not come from Jev.
 *
 * Measured on v2.0.6 (Fase 5 probe):
 * - the event carries `{ sessionID, agent, action, resources, metadata, source,
 *   effect }`, and `action` is the tool name;
 * - a write through the `write` tool arrives as action **`edit`**, not
 *   `write` - the plan's guessed name would have missed every write;
 * - assigning `event.effect = "ask"` inside the hook takes effect: the host
 *   stopped the write and asked (auto-rejecting in a non-interactive run).
 */
import type { SessionDecision } from "../adapters/store"

/**
 * Actions that change the workspace.
 *
 * `edit` covers both edit and write on v2.0.6. `bash`, `shell` and `patch`
 * are included because a shell command is the obvious way around an edit
 * gate; if a host does not emit them, the extra names cost nothing.
 */
export const WRITE_ACTIONS = new Set(["edit", "write", "bash", "shell", "patch"])

export interface PermissionEvent {
	sessionID?: string
	action?: string
	effect?: string
	message?: string
	[key: string]: unknown
}

export interface RouteState {
	choice: string
	closeCall: boolean
	source: "jev" | "llm"
}

export interface GateState {
	review?: SessionDecision
	route?: RouteState
}

export interface GateOutcome {
	/** Undefined means: leave the event exactly as it was. */
	effect?: "ask"
	reason?: string
}

/**
 * Decide whether this event should be hardened.
 *
 * Pure, so the rules can be tested without a host: the hook below just applies
 * what this returns.
 */
export function evaluateGate(event: PermissionEvent, state: GateState | undefined): GateOutcome {
	if (!event.action || !WRITE_ACTIONS.has(event.action)) return {}
	// Only `allow` is ours to change. An existing `ask` is already as strict,
	// and a `deny` is the user's config talking.
	if (event.effect !== "allow") return {}
	if (!state) return {}

	const review = state.review
	if (review?.source === "jev" && review.action === "request_changes") {
		return {
			effect: "ask",
			reason: `Jev: ${review.blockingFindings} unresolved blocker(s) from the last review (${review.runId})`,
		}
	}

	const route = state.route
	if (route?.source === "jev" && (route.closeCall || !routeAllowsWrites(route))) {
		const suffix = route.closeCall ? " (close call)" : ""
		return { effect: "ask", reason: `Jev routed this task to "${route.choice}"${suffix}` }
	}

	return {}
}

/**
 * Whether the routed destination is one that writes.
 *
 * Kept separate so a route decision can be stored without its whole catalog
 * entry: any route that is not an explicit writing agent is treated as
 * non-writing, which errs toward asking.
 */
const WRITING_ROUTES = new Set(["orchestrator", "dev", "qa", "devops", "build"])

export function routeAllowsWrites(route: RouteState): boolean {
	return WRITING_ROUTES.has(route.choice)
}

export interface PermissionHost {
	hook?(name: string, cb: (event: PermissionEvent) => Promise<void>): Promise<unknown>
}

/** Register the gate. A missing hook is not an error - the plugin still works. */
export async function registerPermissionGate(
	permission: PermissionHost | undefined,
	loadState: (sessionID: string) => Promise<GateState | undefined>,
): Promise<void> {
	await permission?.hook?.("evaluate", async (event) => {
		try {
			if (!event.sessionID) return
			const outcome = evaluateGate(event, await loadState(event.sessionID))
			if (!outcome.effect) return
			event.effect = outcome.effect
			// `message` was not among the event's own keys on v2.0.6, so the
			// host may ignore it. Set it anyway: it costs nothing and gives the
			// reason a place to live if the host starts reading it.
			event.message = outcome.reason
		} catch {
			// A gate that throws must not break every permission check; the
			// worst case is the host's own policy, unchanged.
		}
	})
}
