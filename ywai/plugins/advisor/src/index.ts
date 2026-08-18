/**
 * advisor — OpenCode v2 plugin
 *
 * A second model reviews each turn the main agent takes and, when it has
 * something concrete, injects a note the agent reads on its next turn.
 *
 * It watches through `ctx.event.subscribe` (session.idle), thinks in its own
 * child session with its own model, and delivers through `session.synthetic`
 * so the note is a visible block that does not spend a model turn, plus a
 * toast when a TUI is attached.
 *
 * The hard part is not the wiring — it is not becoming noise. That lives in
 * `emission-guard.ts`.
 */

import { EmissionGuard, type Severity } from "./emission-guard"
import { CONFIG_PATH, type ModelRef, loadConfig, loadWatchdog, toastVariant } from "./config"
import { setModel, status, toggle } from "./controls"
import { extractText, fetchMessages } from "./messages"
import { SessionCursor, renderDelta, worthReviewing } from "./delta"
import { ADVISOR_SYSTEM_PROMPT, buildAdvisorPrompt, parseVerdict, renderAdvisory } from "./verdict"

const ADVISOR_AGENT = "advisor"

type SessionState = {
  cursor: SessionCursor
  guard: EmissionGuard
  running: boolean
}

interface V2Plugin {
  id: string
  setup: (ctx: any) => Promise<(() => Promise<void>) | void> | (() => Promise<void>) | void
}

const define = (plugin: V2Plugin): V2Plugin => plugin

const plugin = define({
  id: "advisor",
  async setup(ctx: any) {
    const configPath = typeof ctx.options?.configPath === "string" ? ctx.options.configPath : CONFIG_PATH
    const config = await loadConfig(configPath)

    await ctx.tool.transform((draft: { add: (name: string, spec: unknown) => void }) => {
      for (const spec of controlTools(configPath)) {
        draft.add(spec.name, spec)
      }
    })

    if (!config.enabled || !config.model) {
      return
    }
    const model = config.model
    const watchdog = await loadWatchdog(ctx.directory ?? process.cwd())

    const sessions = new Map<string, SessionState>()
    const ownSessions = new Set<string>()

    try {
      await ctx.session?.hook?.("context", (session: { sessionID?: string; system: unknown[] }) => {
        const id = session.sessionID
        if (!id || !ownSessions.has(id)) return
        session.system.length = 0
        session.system.push({ type: "text", text: ADVISOR_SYSTEM_PROMPT })
      })
    } catch {
      // hook optional
    }

    async function review(sessionID: string): Promise<void> {
      const state = sessions.get(sessionID)
      if (!state || state.running) return
      state.running = true
      try {
        const history = await fetchMessages(ctx.session, sessionID)
        const delta = state.cursor.take(history)

        if (delta.reset) state.guard.reset()
        if (!worthReviewing(delta.messages)) return

        state.guard.beginUpdate()
        const reply = await runAdvisor(ctx.session, sessionID, model, renderDelta(delta.messages), watchdog, ownSessions)
        const verdict = parseVerdict(reply)
        if (verdict.severity === "silent") return

        const admitted = state.guard.admit(verdict.note, verdict.severity)
        if (!admitted.accepted) return

        await deliver(ctx, sessionID, admitted.severity, admitted.note)
      } catch {
        // An advisor failure must never break the session it is watching.
      } finally {
        state.running = false
      }
    }

    const subscribe = ctx.event?.subscribe?.bind(ctx.event)
    if (subscribe) {
      await subscribe(async (event: { type?: string; data?: Record<string, unknown>; properties?: { sessionID?: string } }) => {
        const type = event.type
        const sessionID =
          (typeof event.data?.sessionID === "string" && event.data.sessionID) ||
          event.properties?.sessionID
        if (!sessionID || ownSessions.has(sessionID)) return

        if (type === "session.idle") {
          if (!sessions.has(sessionID)) {
            const history = await fetchMessages(ctx.session, sessionID)
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
      })
    }
  },
})

async function deliver(ctx: any, sessionID: string, severity: Severity, note: string): Promise<void> {
  try {
    await ctx.tui?.showToast?.({
      body: { variant: toastVariant(severity), title: `Advisor (${severity})`, message: note },
    })
  } catch {
    // headless
  }

  const text = renderAdvisory(severity, note)
  if (typeof ctx.session?.synthetic === "function") {
    await ctx.session.synthetic({ sessionID, text })
    return
  }
  await ctx.session.prompt({ sessionID, text, noReply: true })
}

function controlTools(configPath: string) {
  return [
    {
      name: "advisor_status",
      description: "Report whether the advisor is enabled and which model it reviews with.",
      input: { type: "object", properties: {} },
      async execute() {
        return await status(configPath)
      },
    },
    {
      name: "advisor_set_model",
      description:
        "Set the model the advisor reviews with. Requires a provider-qualified reference such as anthropic/claude-sonnet-5.",
      input: {
        type: "object",
        properties: {
          model: { type: "string", description: "provider/model, e.g. anthropic/claude-sonnet-5" },
        },
        required: ["model"],
      },
      async execute(args: { model: string }) {
        return (await setModel(args.model, configPath)).message
      },
    },
    {
      name: "advisor_toggle",
      description: "Turn the advisor on or off for future sessions.",
      input: {
        type: "object",
        properties: {
          enabled: { type: "boolean", description: "true to enable, false to disable" },
        },
        required: ["enabled"],
      },
      async execute(args: { enabled: boolean }) {
        return (await toggle(args.enabled, configPath)).message
      },
    },
  ]
}

async function runAdvisor(
  session: any,
  parentID: string,
  model: ModelRef,
  delta: string,
  watchdog: string | undefined,
  ownSessions: Set<string>,
): Promise<string> {
  const created = await session.create({
    title: "advisor",
    agent: ADVISOR_AGENT,
    model: { id: model.modelID, providerID: model.providerID },
    parentID,
  })
  const sessionID = created?.id ?? created?.data?.id
  if (!sessionID) return ""
  ownSessions.add(sessionID)

  try {
    const text = buildAdvisorPrompt(delta, watchdog)
    const res = await session.prompt({
      sessionID,
      text,
      agent: ADVISOR_AGENT,
      model: { id: model.modelID, providerID: model.providerID },
      system: ADVISOR_SYSTEM_PROMPT,
    })
    if (typeof res?.text === "string" && res.text.trim()) return res.text
    return extractText(res?.data?.parts ?? res?.parts ?? [])
  } finally {
    ownSessions.delete(sessionID)
    try {
      if (typeof session.remove === "function") {
        await session.remove({ sessionID })
      } else if (typeof session.delete === "function") {
        await session.delete({ sessionID, path: { id: sessionID } })
      }
    } catch {
      // a leaked child session is not worth failing the turn over
    }
  }
}

export default plugin
