kill "mission-planning" is now active.

<skill name="mission-planning" filePath="builtin:mission-planning">
# Mission Planning

This skill guides you through the planning phase.

## Phase 1: Understand & Plan (DYNAMIC, ITERATIVE)

This is the most important phase. Your goal is to arrive at a deep, comprehensive understanding of: what we're building, how it works architecturally, where complexity lives, what user-facing surfaces exist, and what the approach should be.

**Start by asking the user** enough questions to build shared understanding of what we're building and what matters — so that all subsequent investigation has direction. Ask as many as make sense in one go. Don't start investigating until these are answered.

**Then interleave these activities as needed** — the problem dictates the path:
- **Investigate** the codebase and technologies via subagents. Delegate deep investigation — code reading, flow tracing, module analysis, operational discovery. You handle structural overview (READMEs, configs, directory layouts) and synthesize subagent reports.
- **Research** technologies where your training knowledge may be insufficient. Follow the Online Research guidelines — delegate to subagents.
- **Identify testing surfaces** — where behavior can be tested through user-facing boundaries (browser UI, CLI, API). Delegate architectural analysis to subagents when assessing this.
- **Think through the approach** — how will this be built, what are the boundaries, where will workers need the most guidance? For any deep thinking or thorough analysis, delegate to subagents.
- **Ask again** if investigation reveals new ambiguities.

**Always delegate deep investigation and deep thinking to subagents.** Your context window is finite — preserve it for orchestration, synthesis, and user interaction. When you need thorough analysis of any aspect (architectural decomposition, surface identification, technology assessment, edge case enumeration), spawn a subagent.

### Iterative Exploration Loop

Planning is not a single pass of investigation followed by a proposal. After each round of investigation, explicitly enumerate what you still don't know and assess which unknowns matter most. For each high-importance unknown, either investigate via subagent or ask the user. Then re-assess — did exploration surface new unknowns? Keep going until nothing important is left unexplored.

Continue until you can answer these questions about every part of the system you're building:
- What does it do?
- What are its boundaries?
- Where does complexity concentrate?
- How would an independent party verify it works?

If you can't answer these, you don't understand the problem well enough yet. Keep investigating.

Only move forward when you have a clear, deep picture of what success looks like.

## Phase 2: Architectural Design & Decomposition

Design the system to fulfill all user requirements. Delegate deep architectural analysis to subagents if needed.

Present the design to the user and get explicit confirmation before proceeding.

Take care to ensure this design is robust and well thought-out. This is the blueprint for the entire mission.

## Phase 3: Infrastructure & Boundaries

Determine what infrastructure is needed:
- What services? (databases, caches, queues, etc.)
- What processes? (API server, web frontend, workers, etc.)
- What ports will each need?
- Any external APIs or resources?

**IMPORTANT: Proactively check what's already running.**

e.g.
```bash
# Check listening ports
lsof -i -P -n | grep LISTEN

# Check running containers
docker ps

# Check running node/python processes
ps aux | grep -E 'node|python|java' | grep -v grep

etc.
```

Analyze the output to:
- Identify ports already in use (avoid conflicts)
- Find existing services you can reuse (e.g., existing postgres on 5432)
- Discover processes that might conflict with your mission
- Note any ports/directories that should be off-limits

Present needed infrastructure and how they fit with the user's setup:

```
This mission will need:
- Postgres database (may I use the existing one on 5432?)
- API server on port 3100
- [etc.]

Does this setup work for you?
```

**You need explicit user confirmation to proceed.**

## Phase 4: Set Up Credentials & Accounts (INTERACTIVE)

If the mission involves any external dependencies (APIs, databases, auth providers, third-party SDKs), you must set up real credentials and connections so the mission can be validated end-to-end. This is not optional — the default is real integration, not mocks.

For greenfield projects, this likely means all credentials and accounts. For existing codebases, investigate what's already configured and only set up what's missing.

If new credentials/accounts are needed:
1. If they don't already exist, initialize any needed configuration files first (e.g., `.env` files with variable names and placeholder values), so the user has somewhere to put them.
2. Guide the user through the specific steps to create any needed accounts and generate credentials, providing clear instructions and links.

**CRITICAL: During this step, we must set up everything such that the mission can be validated end-to-end with real integrations.** Workers must be able to test against real APIs, real databases, real auth flows. If a feature streams from an LLM API, the real API key must be configured. If a feature processes payments, a real sandbox/test-mode key must be configured. The validation contract will include assertions that exercise these real integration paths.

The user may explicitly choose to defer specific credentials (e.g., "use mocks for now", "I'll add Stripe keys later"). Respect this, but note it in the mission proposal so workers know what's unavailable and which end-to-end assertions are deferred. This is an explicit user opt-out — never silently default to mocks.

Only skip this phase if the mission genuinely has no external credential or account dependencies.

Ensure that you don't commit any secrets or sensitive information. Add these files to `.gitignore`.

The mission readiness check (Phase 6) will actively verify that these credentials and integrations work by exercising the real APIs/services. Do not assume credentials are valid just because they were configured here.

## Phase 5: Testing & Validation Strategy

Use subagents to investigate testing infrastructure and plan the validation strategy. For existing codebases, discover established patterns and conventions. For greenfield, determine what testing infrastructure and validation tooling the mission needs. If the mission's technologies have specific testing patterns or libraries that you don't know by heart (e.g., Convex test helpers, Supabase local dev), reference your online research findings or do targeted follow-up research. Always delegate deep investigation to subagents.

### Testing Infrastructure

Consider whether the mission needs dedicated testing features beyond per-worker TDD:
- Shared test fixtures, seed data, or factories that multiple features depend on
- E2e tests for critical user flows (especially in existing codebases that already have e2e coverage)
- Integration test setup (e.g., test database configuration, mock services)

### Programmatic Validation Plan


  ... (102 lines truncated)


The user testing validator will further constrain parallelization based on its own isolation analysis.

### Encode Findings

These mission artifacts are created later, after the user accepts the proposal and missionDir exists. Keep track of these findings during the readiness check, then persist them into the appropriate destination(s) below when authoring those mission artifacts.

Capture everything validators need in `library/user-testing.md` so they can act without re-deriving it:
- Surface discovery findings under a `## Validation Surface` section, including any user-specified testing skills/tools
- Add a `## Validation Prerequisites` section listing only what is required to execute validation flows, how each prerequisite was verified during the readiness check, and whether any allowlist/whitelist action was required
- Resource cost classification per surface under a `## Validation Concurrency` section (max concurrent validators, with numbers and rationale)

Persist mission-readiness findings in the most authoritative destination(s) for their purpose:
- `AGENTS.md`: mission-wide rules workers must follow
- `skills/`: per-worker-type work procedures and references to the skills/tools used at each step
- `library/user-testing.md`: validator-specific tools, validation prerequisites, setup steps, and testing-surface guidance
- `architecture.md`: how mission-critical dependencies fit into the system and where they are used
- `mission.md`: the finalized mission-level tools, skills, dependencies, services, and other global decisions the mission will rely on
- feature definitions: feature-specific dependency requirements, especially when only certain features depend on a package, SDK, tool, or service
- `library/environment.md`: factual environment/setup/access state only, such as verified availability, allowlist/whitelist status, required accounts, env vars, endpoints, installation notes, and platform-specific setup details

### Confirm with User

If any mission-critical prerequisite remains unresolved, stop here and treat it as a blocker. Do not ask for final confirmation until the prerequisite is resolved or the user has explicitly changed the mission scope/tooling to remove that dependency.

Before concluding this phase, you must align with the user on both the testing and validation strategy and get explicit confirmation on:
- What testing infrastructure will be set up (fixtures, e2e, integration)
- What test types apply (unit, component, integration, e2e)
- Validation surfaces, tools, setup, and resource cost classification

**You need explicit user confirmation to proceed.**

## Phase 7: Identify & Confirm Milestones

Now that you have a deep understanding of requirements, architecture, surfaces, and validation strategy, identify milestones.

Each milestone is a vertical slice of functionality that leaves the product in a coherent, testable state. Milestones control when validation runs — when all features in a milestone complete, the system automatically injects scrutiny + user testing validators.

Present your milestones to the user. Explain the tradeoff - more milestones means a more thorough validation contract and a more granular breakdown of features, resulting in higher quality but increasing mission cost. Fewer milestones means faster execution but less detailed validation and coarser feature decomposition. Let the user decide where they want that balance.

**You need explicit user confirmation to proceed.** Iterate until you have it.

**Milestone Lifecycle:** Once a milestone's validators pass, it is **sealed**. Any subsequent work goes into a new milestone.

## Phase 8: Create Mission Proposal

With the comprehensive plan complete, call `propose_mission` with a detailed markdown proposal.

The proposal should include:
- Plan overview
- Expected functionality (milestones and features, structured for readability)
- Environment setup
- Infrastructure (services, processes, ports) and boundaries
- Testing strategy: how will the mission be tested? Cover which levels apply (unit, component, integration, e2e)
- User testing strategy: how manual user testing will work (what surfaces to test, what tools to use, any setup needed).
- Mission readiness: the verified dependencies/tools/SDKs the mission will use, and confirmation that the validation path is executable.
- Non-functional requirements

The infrastructure section tells workers what's needed and what to avoid. Example:

```markdown
## Infrastructure

**Services:**
- Postgres on localhost:5432 (existing)
- API server on port 3100
- Web frontend on port 3101
- Background worker on port 3102

**Off-limits:**
- Redis on 6379 (other project)
- Ports 3000-3010 (user's dev servers)
- /data directory
```

NOTE: features.json will be much more detailed than the proposal.

After `propose_mission` is accepted, you will have a `missionDir`.

</skill>