/**
 * advisor — OpenCode plugin
 *
 * A second model reviews each turn the main agent takes and, when it has
 * something concrete, injects a note the agent reads on its next turn.
 *
 * It watches through `event` (session.idle), thinks in its own child session
 * with its own model, and delivers through `session.prompt({ noReply: true })`
 * so the note is a visible block in the transcript that costs no extra turn,
 * plus a toast so it is noticed while it still matters.
 *
 * The hard part is not the wiring — it is not becoming noise. That lives in
 * `emission-guard.ts`, and it is enforced in code because the same rules stated
 * only in a prompt are demonstrably ignored by real advisor models.
 */

import { type Plugin, type PluginInput, tool } from "@opencode-ai/plugin"
import { EmissionGuard, type Severity } from "./emission-guard"
import { CONFIG_PATH, type ModelRef, loadConfig, loadWatchdog, toastVariant } from "./config"
import { setModel, status, toggle } from "./controls"
import { extractText, fetchMessages } from "./messages"
import { SessionCursor, type TrackedMessage, renderDelta, worthReviewing } from "./delta"
import { ADVISOR_SYSTEM_PROMPT, buildAdvisorPrompt, parseVerdict, renderAdvisory } from "./verdict"

/**
 * Agent profile the review runs as. ywai installs it; it is deliberately not a
 * delegation target — nothing should be able to task the reviewer.
 */
const ADVISOR_AGENT = "advisor"

/** Per-primary-session state. */
type SessionState = {
  cursor: SessionCursor
  guard: EmissionGuard
  /** Serializes reviews so a slow one cannot overlap the next turn. */
  running: boolean
}












const AdvisorPlugin: Plugin = async ({ client, directory }: PluginInput, options?: Record<string, unknown>) => {
  // OpenCode passes plugin options through from the config entry. Taking the
  // config path from there keeps the plugin testable without reaching for the
  // real home directory, and lets an operator point it elsewhere.
  const configPath = typeof options?.configPath === "string" ? options.configPath : CONFIG_PATH
  const config = await loadConfig(configPath)
  // The controls are registered either way. A disabled advisor still has to be
  // reachable from `/advisor on`, or enabling it means leaving the session.
  const controls = buildControls(configPath)

  if (!config.enabled || !config.model) {
    // No model, no reviewing. Same rule as vision-bridge: a background job that
    // silently picks some default model is a surprise on someone's bill.
    return { tool: controls }
  }
  const model = config.model
  const watchdog = await loadWatchdog(directory)

  const sessions = new Map<string, SessionState>()
  /** Sessions this plugin created, so the advisor never reviews itself. */
  const ownSessions = new Set<string>()

  async function review(sessionID: string): Promise<void> {
    const state = sessions.get(sessionID)
    if (!state || state.running) return
    state.running = true
    try {
      const history = await fetchMessages(client, sessionID)
      const delta = state.cursor.take(history)

      // A rewritten transcript invalidates advice given about the old one, so
      // the dedupe history goes with it — otherwise the advisor stays silent
      // about issues that are still present in the rewritten history.
      if (delta.reset) state.guard.reset()

      if (!worthReviewing(delta.messages)) return

      state.guard.beginUpdate()
      const reply = await runAdvisor(client, sessionID, model, renderDelta(delta.messages), watchdog, ownSessions)
      const verdict = parseVerdict(reply)
      if (verdict.severity === "silent") return

      const admitted = state.guard.admit(verdict.note, verdict.severity)
      if (!admitted.accepted) return

      await deliver(client, sessionID, admitted.severity, admitted.note)
    } catch {
      // An advisor failure must never break the session it is watching.
    } finally {
      state.running = false
    }
  }

  return {
    tool: controls,

    /**
     * Replaces the system prompt for the advisor's own sessions.
     *
     * `system` on session.prompt appends — it does not replace. Without this,
     * a review inherits everything OpenCode assembles for a normal session:
     * the user's AGENTS.md/CLAUDE.md persona, SDD orchestration rules, TDD
     * mode, MCP instructions. Measured on a real session, that is thousands of
     * tokens per review telling a reviewer that must answer in two lines to be
     * a passionate architect who loads skills before responding.
     *
     * Only this plugin's own sessions are touched; every other session is left
     * exactly as OpenCode built it.
     */
    "experimental.chat.system.transform": async (input, output) => {
      const id = (input as { sessionID?: string }).sessionID
      if (!id || !ownSessions.has(id)) return
      output.system.length = 0
      output.system.push(ADVISOR_SYSTEM_PROMPT)
    },

    event: async ({ event }) => {
      const type = (event as { type?: string }).type
      const props = (event as { properties?: { sessionID?: string } }).properties
      const sessionID = props?.sessionID
      if (!sessionID || ownSessions.has(sessionID)) return

      if (type === "session.idle") {
        if (!sessions.has(sessionID)) {
          // Seed at the current length: enabling mid-conversation reviews what
          // happens next, not the hour that already happened.
          const history = await fetchMessages(client, sessionID)
          sessions.set(sessionID, {
            cursor: new SessionCursor(history.length),
            guard: new EmissionGuard(),
            running: false,
          })
          return
        }
        await review(sessionID)
        return
      }

      if (type === "session.compacted") {
        sessions.get(sessionID)?.cursor.reset()
        sessions.get(sessionID)?.guard.reset()
        return
      }

      if (type === "session.deleted") {
        sessions.delete(sessionID)
      }
    },
  }
}

/**
 * Delivers a note two ways, because they answer different needs: the toast is
 * seen while the work is still fresh, the transcript block is what the agent
 * reads on its next turn and what remains afterwards.
 */
async function deliver(
  client: PluginInput["client"],
  sessionID: string,
  severity: Severity,
  note: string,
): Promise<void> {
  try {
    await (client as any).tui?.showToast?.({
      body: { variant: toastVariant(severity), title: `Advisor (${severity})`, message: note },
    })
  } catch {
    // headless or no TUI attached — the transcript block still lands
  }

  await (client as any).session.prompt({
    path: { id: sessionID },
    body: {
      // The whole point: a visible message that does not spend a model turn.
      noReply: true,
      parts: [{ type: "text", text: renderAdvisory(severity, note) }],
    },
  })
}

/**
 * Tools behind the `/advisor` command. Kept as tools rather than prose in the
 * command file because the command has to actually change something.
 */
function buildControls(configPath: string) {
  return {
    advisor_status: tool({
      description: "Report whether the advisor is enabled and which model it reviews with.",
      args: {},
      async execute() {
        return await status(configPath)
      },
    }),
    advisor_set_model: tool({
      description:
        "Set the model the advisor reviews with. Requires a provider-qualified reference such as anthropic/claude-sonnet-5.",
      args: { model: tool.schema.string().describe("provider/model, e.g. anthropic/claude-sonnet-5") },
      async execute(args) {
        return (await setModel(args.model, configPath)).message
      },
    }),
    advisor_toggle: tool({
      description: "Turn the advisor on or off for future sessions.",
      args: { enabled: tool.schema.boolean().describe("true to enable, false to disable") },
      async execute(args) {
        return (await toggle(args.enabled, configPath)).message
      },
    }),
  }
}

/** Runs one advisor pass in an isolated child session and disposes of it. */
async function runAdvisor(
  client: PluginInput["client"],
  parentID: string,
  model: ModelRef,
  delta: string,
  watchdog: string | undefined,
  ownSessions: Set<string>,
): Promise<string> {
  const created = await (client as any).session.create({ body: { parentID, title: "advisor" } })
  const sessionID = created?.data?.id ?? created?.id
  if (!sessionID) return ""
  ownSessions.add(sessionID)

  try {
    const res = await (client as any).session.prompt({
      path: { id: sessionID },
      body: {
        model,
        // The review runs as its own agent so it cannot inherit the permissions
        // of the session it is reviewing. Measured in OpenCode 1.18.5: a prompt
        // with tools:{} saw 63 tools (including delegate and task); {"*":false}
        // alone still left 44; scoping to an agent brought it to 1. Both are
        // kept — the profile is the boundary, the wildcard is the belt.
        agent: ADVISOR_AGENT,
        system: ADVISOR_SYSTEM_PROMPT,
        tools: { "*": false },
        parts: [{ type: "text", text: buildAdvisorPrompt(delta, watchdog) }],
      },
    })
    return extractText(res?.data?.parts ?? res?.parts ?? [])
  } finally {
    ownSessions.delete(sessionID)
    try {
      await (client as any).session.delete({ path: { id: sessionID } })
    } catch {
      // a leaked child session is not worth failing the turn over
    }
  }
}



export default AdvisorPlugin
