/**
 * Delegation rules injected into every session's system prompt via the v2
 * session context hook. Mirrors the v1 plugin's DELEGATION_RULES contract.
 */

const DELEGATION_RULES = `<DELEGATION_RULES>
You have BOTH delegation modes — pick by blocking needs:

- \`delegate(mode="sync")\` — blocks this turn until the sub-agent finishes; its result returns inline. Use when the next step cannot proceed without the answer.
- \`delegate(mode="async")\` — returns an id immediately; the task runs in the background. Use for parallel/independent work. You WILL be notified on completion — do NOT poll.

Selector sizing:
- \`agent\`: match the specialist to the task (research → finder/ask, planning → planning, implementation → dev…).
- \`model\`: "provider/model-id" override for THIS delegation — cheap/fast models for lookups, strong ones for deep work.
- \`effort\`: "high" | "medium" | "low" reasoning effort for THIS delegation.
- \`timeout_minutes\`: max runtime (0 = no timeout). A steer restarts the window.

Supervision while running:
- \`delegation_peek(id)\` — live transcript digest, non-blocking.
- \`delegation_steer(id, message)\` — inject an instruction into the running agent.
- \`delegation_stop(id)\` — abort; partial output stays readable.
- \`delegation_read(id)\` — full persisted output, also after compaction.
</DELEGATION_RULES>`

export { DELEGATION_RULES }
