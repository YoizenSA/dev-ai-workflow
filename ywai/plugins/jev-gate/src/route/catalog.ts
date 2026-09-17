/**
 * The route catalog (PLAN 3.5, corrected for a ywai-seeded config).
 *
 * The plan's catalog named OpenCode's stock `plan` / `build` agents. In a
 * ywai install those are not the agents that exist: a live session runs as
 * `orchestrator`, alongside `dev`, `planning`, `reviewer`, `finder`, `ask`,
 * `architect`, `qa`, `devops`. Routing to `build` in that config would name an
 * agent that is not there, so the catalog is built from the agents the host
 * actually reports, and the table below is the fallback when it reports none.
 *
 * Every option carries `what`, `not_for` and `examples`. `not_for` is the part
 * that matters: the delegate lesson was that an option with an empty `not_for`
 * wins everything, and the Fase 1 run showed the same failure in the owner
 * question, where `security` swallowed testGap and correctness.
 */
export interface RouteOption {
	what: string
	not_for: string[]
	examples: string[]
	/** The agent to switch to, when this option maps to one. */
	agent?: string
	/** Whether that agent may write files. Drives the gate, not the prose. */
	writes: boolean
}

/**
 * Routes that exist regardless of which agents are installed.
 *
 * `human` is not an agent on purpose: it is the escape hatch for a close call
 * or an irreversible action, and it must stay available even when the roster
 * is empty.
 */
export const UNIVERSAL_ROUTES: Record<string, RouteOption> = {
	inline: {
		what: "Answer here, in this turn, without changing any file",
		not_for: ["Any change to code", "Searching across the repo", "Reviewing a diff"],
		examples: ["What does this function return?", "Explain this error message"],
		writes: false,
	},
	review: {
		what: "Review a diff or a file with the Jev review tools",
		not_for: ["Writing the fix", "Designing something new", "Explaining unchanged code"],
		examples: ["Review my changes", "Is this diff safe to merge?"],
		writes: false,
	},
	human: {
		what: "A person should decide before an agent acts",
		not_for: ["Routine work with a clear owner", "Anything already decided"],
		examples: [
			"Rotate the production credentials",
			"Run an irreversible migration",
			"The task is ambiguous enough that guessing is expensive",
		],
		writes: false,
	},
}

/**
 * How a known ywai agent is described to Jev.
 *
 * Descriptions are shortened from each agent's own AGENT.md, so the routing
 * criteria and the agent's self-description cannot drift apart silently.
 */
export const KNOWN_AGENTS: Record<string, Omit<RouteOption, "agent">> = {
	orchestrator: {
		what: "Take a goal, pick an execution mode, and either act or coordinate specialists",
		not_for: ["A single question with a one-line answer", "Reviewing a diff"],
		examples: ["Build and ship X", "Coordinate this multi-step change"],
		writes: true,
	},
	dev: {
		what: "Write or change code, fix bugs, refactor, build features",
		not_for: ["Questions that need no change", "Explaining existing code", "Reviewing a diff"],
		examples: ["Add rate limiting to the API", "Fix the failing parser test"],
		writes: true,
	},
	planning: {
		what: "Investigate read-only and draft a plan, waiting for approval before acting",
		not_for: ["Implementing the plan", "Answering a one-line question"],
		examples: ["How should we approach the migration?", "Plan this refactor"],
		writes: false,
	},
	architect: {
		what: "Make design and architecture decisions, weigh trade-offs",
		not_for: ["Implementation", "Locating code"],
		examples: ["How should we structure the retry logic?", "Compare two approaches"],
		writes: false,
	},
	reviewer: {
		what: "Review code, audit quality, find bugs and security issues",
		not_for: ["Writing the fix", "Designing something new"],
		examples: ["Review this PR", "Audit this module"],
		writes: false,
	},
	finder: {
		what: "Locate code in the repo, read-only",
		not_for: ["Explaining what was found", "Changing what was found"],
		examples: ["Where do we validate the token?", "Which file opens the socket?"],
		writes: false,
	},
	ask: {
		what: "Answer questions and explain concepts from codebase evidence",
		not_for: ["Any change to code", "Locating every occurrence of something"],
		examples: ["What is this module for?", "Why does this fail?"],
		writes: false,
	},
	qa: {
		what: "Design test strategy, write tests, validate an implementation",
		not_for: ["Implementing the feature itself", "Reviewing unrelated code"],
		examples: ["Write tests for this parser", "What is missing coverage here?"],
		writes: true,
	},
	devops: {
		what: "CI/CD pipelines, deployment, containers, infrastructure",
		not_for: ["Application logic", "Reviewing a diff"],
		examples: ["Fix this pipeline", "Harden this Dockerfile"],
		writes: true,
	},
	designer: {
		what: "Audit interfaces, define visual specs, review screens against the design system and accessibility",
		not_for: ["Backend logic", "Implementing the whole feature", "Reviewing a diff"],
		examples: ["This screen looks bad", "Audit the spacing and contrast here"],
		writes: true,
	},
	memory: {
		what: "Analyze stored memories and produce a consolidation plan for human review",
		not_for: ["Changing code", "Answering a question about the codebase"],
		examples: ["Consolidate what we learned this week"],
		writes: false,
	},
	"scenario-runner": {
		what: "Run BDD scenarios against the running app and file the evidence",
		not_for: ["Writing the scenarios", "Fixing what the run exposes"],
		examples: ["Run the scenarios", "Verify it actually works end to end"],
		writes: true,
	},
	"qa-orchestrator": {
		what: "Coordinate QA automation work and teach manual testers how automation works",
		not_for: ["Application code outside the tests", "Reviewing an unrelated diff"],
		examples: ["Guide me through automating this", "Plan the test strategy"],
		writes: true,
	},
	"qa-dev": {
		what: "Write automated tests",
		not_for: ["Implementing the feature under test", "Deciding the test strategy"],
		examples: ["Create a Playwright test for this flow"],
		writes: true,
	},
}

export interface AgentLike {
	name?: string
	description?: string
	/**
	 * `subagent` agents are not delegation targets - advisor says so in its own
	 * description, and scenario-runner is driven by a handoff, not by a route.
	 */
	mode?: string
}

/**
 * Build the catalog from the agents the host reports.
 *
 * An agent we do not recognise still gets an entry, described by its own
 * description and assumed to write - assuming it cannot write would be the
 * failure that lets a write through the gate unasked.
 */
export function buildCatalog(agents: AgentLike[] = []): Record<string, RouteOption> {
	const catalog: Record<string, RouteOption> = { ...UNIVERSAL_ROUTES }
	for (const agent of agents) {
		const name = agent.name
		if (!name || catalog[name]) continue
		// Routing to something the user cannot switch to is a dead end.
		if (agent.mode === "subagent") continue
		const known = KNOWN_AGENTS[name]
		if (known) {
			catalog[name] = { ...known, agent: name }
			continue
		}
		const description = (agent.description ?? "").replace(/\s+/g, " ").trim()
		if (!description) continue
		catalog[name] = {
			what: description.slice(0, 300),
			not_for: ["Anything another option describes more precisely"],
			examples: [],
			agent: name,
			writes: true,
		}
	}
	return catalog
}

/**
 * The fallback roster when the host reports nothing.
 *
 * `scenario-runner` and `qa-dev` are in KNOWN_AGENTS so that a host that DOES
 * report them gets a curated entry, but they are subagents in a stock ywai
 * install, so they stay out of the fallback: routing to an agent the user
 * cannot switch to is worse than routing to `human`.
 */
const SUBAGENT_ONLY = new Set(["scenario-runner", "advisor"])

export function defaultCatalog(): Record<string, RouteOption> {
	return buildCatalog(
		Object.keys(KNOWN_AGENTS)
			.filter((name) => !SUBAGENT_ONLY.has(name))
			.map((name) => ({ name })),
	)
}
