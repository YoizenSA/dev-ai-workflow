import { describe, expect, test } from "bun:test"
import {
	WRITE_ACTIONS,
	evaluateGate,
	registerPermissionGate,
	type GateState,
	type PermissionEvent,
} from "../src/gates/permission"
import { buildCatalog, defaultCatalog, KNOWN_AGENTS } from "../src/route/catalog"
import { routeTask, topMargin } from "../src/route/decide"
import type { JevClient, SystemOneRequest, SystemOneResponse } from "../src/jev/client"

const blockingReview: GateState = {
	review: {
		runId: "run_1",
		action: "request_changes",
		blockingFindings: 2,
		source: "jev",
		at: "now",
	},
}

const allowEdit = (): PermissionEvent => ({
	sessionID: "ses_1",
	// v2.0.6 reports a write through the `write` tool as action "edit".
	action: "edit",
	effect: "allow",
})

describe("permission gate", () => {
	test("hardens allow to ask after a blocking review", () => {
		const outcome = evaluateGate(allowEdit(), blockingReview)
		expect(outcome.effect).toBe("ask")
		expect(outcome.reason).toContain("2 unresolved blocker")
	})

	test("never relaxes: ask and deny are left alone", () => {
		expect(evaluateGate({ ...allowEdit(), effect: "ask" }, blockingReview).effect).toBeUndefined()
		expect(evaluateGate({ ...allowEdit(), effect: "deny" }, blockingReview).effect).toBeUndefined()
	})

	test("ignores a decision that did not come from Jev", () => {
		const fromLlm: GateState = {
			review: { ...blockingReview.review!, source: "llm" },
		}
		expect(evaluateGate(allowEdit(), fromLlm).effect).toBeUndefined()
	})

	test("a clean review does not gate anything", () => {
		const clean: GateState = { review: { ...blockingReview.review!, action: "clean" } }
		expect(evaluateGate(allowEdit(), clean).effect).toBeUndefined()
	})

	test("read-only actions are not the gate's business", () => {
		expect(evaluateGate({ ...allowEdit(), action: "read" }, blockingReview).effect).toBeUndefined()
		expect(evaluateGate({ ...allowEdit(), action: "glob" }, blockingReview).effect).toBeUndefined()
	})

	test("bash is gated too - a shell is the way around an edit gate", () => {
		expect(WRITE_ACTIONS.has("bash")).toBe(true)
		expect(evaluateGate({ ...allowEdit(), action: "bash" }, blockingReview).effect).toBe("ask")
	})

	test("a route to a non-writing agent asks before a write", () => {
		const routed: GateState = {
			route: { choice: "planning", closeCall: false, source: "jev" },
		}
		expect(evaluateGate(allowEdit(), routed).reason).toContain('routed this task to "planning"')
	})

	test("a route to a writing agent does not ask", () => {
		const routed: GateState = { route: { choice: "dev", closeCall: false, source: "jev" } }
		expect(evaluateGate(allowEdit(), routed).effect).toBeUndefined()
	})

	test("a close call asks even when the agent writes", () => {
		const routed: GateState = { route: { choice: "dev", closeCall: true, source: "jev" } }
		expect(evaluateGate(allowEdit(), routed).reason).toContain("close call")
	})

	test("no state means no opinion", () => {
		expect(evaluateGate(allowEdit(), undefined).effect).toBeUndefined()
	})

	test("the hook mutates the event in place, as v2.0.6 requires", async () => {
		let handler: ((event: PermissionEvent) => Promise<void>) | undefined
		await registerPermissionGate(
			{ hook: async (_name, cb) => void (handler = cb) },
			async () => blockingReview,
		)
		const event = allowEdit()
		await handler!(event)
		expect(event.effect).toBe("ask")
		expect(event.message).toContain("Jev")
	})

	test("a gate that throws leaves the host policy untouched", async () => {
		let handler: ((event: PermissionEvent) => Promise<void>) | undefined
		await registerPermissionGate(
			{ hook: async (_name, cb) => void (handler = cb) },
			async () => {
				throw new Error("storage down")
			},
		)
		const event = allowEdit()
		await handler!(event)
		expect(event.effect).toBe("allow")
	})

	test("a host without a permission hook is not an error", async () => {
		await registerPermissionGate(undefined, async () => blockingReview)
		await registerPermissionGate({}, async () => blockingReview)
	})
})

describe("route catalog", () => {
	test("is built from the agents the host reports", () => {
		const catalog = buildCatalog([{ name: "dev" }, { name: "planning" }])
		expect(Object.keys(catalog).sort()).toEqual(["dev", "human", "inline", "planning", "review"])
		expect(catalog.dev.agent).toBe("dev")
		expect(catalog.dev.writes).toBe(true)
		expect(catalog.planning.writes).toBe(false)
	})

	test("an unknown agent is described by itself and assumed to write", () => {
		const catalog = buildCatalog([{ name: "infra-docs", description: "Maintains the infra wiki" }])
		expect(catalog["infra-docs"].what).toBe("Maintains the infra wiki")
		// Assuming it cannot write is the mistake that lets a write past the gate.
		expect(catalog["infra-docs"].writes).toBe(true)
	})

	test("subagents are not routable destinations", () => {
		const catalog = buildCatalog([
			{ name: "dev" },
			{ name: "advisor", mode: "subagent" },
			{ name: "scenario-runner", mode: "subagent" },
		])
		expect(catalog.advisor).toBeUndefined()
		expect(catalog["scenario-runner"]).toBeUndefined()
		expect(catalog.dev).toBeDefined()
		// The fallback roster excludes them too.
		expect(defaultCatalog()["scenario-runner"]).toBeUndefined()
	})

	test("the known roster covers the core agents a ywai install ships", () => {
		for (const name of ["orchestrator", "dev", "planning", "architect", "reviewer", "finder", "ask", "qa", "devops", "designer", "memory"]) {
			expect(KNOWN_AGENTS[name], `${name} missing from KNOWN_AGENTS`).toBeDefined()
		}
	})

	test("human survives an empty roster", () => {
		expect(buildCatalog([]).human).toBeDefined()
		expect(buildCatalog([]).human.agent).toBeUndefined()
	})

	test("the fallback roster is the ywai one, not OpenCode's plan/build", () => {
		const catalog = defaultCatalog()
		expect(catalog.orchestrator).toBeDefined()
		expect(catalog.build).toBeUndefined()
		expect(Object.keys(KNOWN_AGENTS)).toContain("finder")
	})

	test("every option says what it is not for", () => {
		for (const [name, option] of Object.entries(defaultCatalog())) {
			expect(option.not_for.length, `${name} has an empty not_for`).toBeGreaterThan(0)
		}
	})
})

describe("routeTask", () => {
	const client = (response: SystemOneResponse): JevClient => ({
		async systemOne(_request: SystemOneRequest) {
			return response
		},
	})

	test("maps a choice to its agent", async () => {
		const decision = await routeTask(
			client({ model: "jev", answers: { route: { type: "choice", choice: "dev" } } }),
			{ task: "add rate limiting", agents: [{ name: "dev" }] },
		)
		expect(decision.choice).toBe("dev")
		expect(decision.agent).toBe("dev")
		expect(decision.writes).toBe(true)
		expect(decision.closeCall).toBe(false)
	})

	test("an unknown label escalates to human instead of guessing", async () => {
		const decision = await routeTask(
			client({ model: "jev", answers: { route: { type: "choice", choice: "wizard" } } }),
			{ task: "do something" },
		)
		expect(decision.choice).toBe("human")
		expect(decision.closeCall).toBe(true)
		expect(decision.reasonCodes).toContain("close_call")
	})

	test("a thin margin is a close call", async () => {
		const decision = await routeTask(
			client({
				model: "jev",
				answers: {
					route: { type: "choice", choice: "dev", probabilities: { dev: 0.44, planning: 0.4 } },
				},
			}),
			{ task: "maybe refactor", agents: [{ name: "dev" }, { name: "planning" }] },
		)
		expect(decision.closeCall).toBe(true)
		expect(decision.reasonCodes).toContain("close_call")
		expect(decision.reasonCodes).toContain("low_confidence")
	})

	test("reason codes are derived by us, not by Jev", async () => {
		const decision = await routeTask(
			client({ model: "jev", answers: { route: { type: "choice", choice: "dev" } } }),
			{ task: "fix it", wantsWrite: true, failingTests: true, agents: [{ name: "dev" }] },
		)
		expect(decision.reasonCodes).toEqual(["write_requested", "failing_tests"])
	})

	test("topMargin needs two numbers to mean anything", () => {
		expect(topMargin(undefined)).toBeUndefined()
		expect(topMargin({ a: 1 })).toBeUndefined()
		expect(topMargin({ a: 0.7, b: 0.2 })).toBeCloseTo(0.5)
	})
})
