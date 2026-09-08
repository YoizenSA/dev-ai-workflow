// @ts-nocheck
/** @jsxImportSource @opentui/solid */
import { createSignal, onCleanup, createMemo } from "solid-js"
import { readdirSync, readFileSync, statSync } from "node:fs"
import { homedir } from "node:os"
import { join } from "node:path"

const id = "ywai-statusline"

type TuiTheme = {
  textMuted: string
  accent: string
}

// The published opencode-subagent-statusline peers on `@opencode-ai/plugin
// >=1.14.50 <2`, so it cannot load here. This is the v2 replacement, and it
// reads the same delegation state the background-agents plugin already writes
// rather than talking to the server: the TUI runs in the client process and
// shares no memory with the plugin host.
//
// The background-agents DelegationManager mirrors every ACTIVE delegation to
// `<id>.state.json` beside its artifact and deletes it on finalization, so the
// set of state files on disk is exactly the set of running delegations.
const delegationsRoot = () =>
  join(homedir(), ".local", "share", "opencode", "delegations")

type Active = {
  id: string
  agent?: string
  model?: string
}

/**
 * Collect running delegations. Every read is defensive: the directory may not
 * exist yet, and a state file can be caught mid-write, in which case skipping
 * it simply undercounts for one tick instead of tearing down the statusline.
 */
function readActive(): Active[] {
  const out: Active[] = []
  let projects: string[]
  try {
    projects = readdirSync(delegationsRoot())
  } catch {
    return out
  }
  for (const project of projects) {
    const projectDir = join(delegationsRoot(), project)
    let sessions: string[]
    try {
      if (!statSync(projectDir).isDirectory()) continue
      sessions = readdirSync(projectDir)
    } catch {
      continue
    }
    for (const session of sessions) {
      const sessionDir = join(projectDir, session)
      let files: string[]
      try {
        files = readdirSync(sessionDir)
      } catch {
        continue
      }
      for (const file of files) {
        if (!file.endsWith(".state.json")) continue
        try {
          const parsed = JSON.parse(readFileSync(join(sessionDir, file), "utf8"))
          const record = parsed?.record
          if (!record?.id) continue
          out.push({ id: record.id, agent: record.agent, model: record.model })
        } catch {
          continue
        }
      }
    }
  }
  return out
}

function Statusline(props: { theme: TuiTheme }) {
  const [active, setActive] = createSignal<Active[]>(readActive())

  // 1s is well under the time any delegation takes and costs a few directory
  // reads; the TUI redraws only when the rendered text actually changes.
  const timer = setInterval(() => setActive(readActive()), 1000)
  onCleanup(() => clearInterval(timer))

  const label = createMemo(() => {
    const running = active()
    if (running.length === 0) return ""
    if (running.length === 1) {
      const one = running[0]
      const model = one.model ? ` · ${one.model.split("/").pop()}` : ""
      return `⣾ ${one.agent ?? "agent"}${model} · ${one.id}`
    }
    const agents = [...new Set(running.map((d) => d.agent ?? "agent"))].join(", ")
    return `⣾ ${running.length} delegations · ${agents}`
  })

  return label() ? <text fg={props.theme.accent}>{label()}</text> : null
}

// OpenCode2 beta uses the v2 TUI module contract: `{ id, setup }`.
const setup = (ctx: { theme: TuiTheme; ui: { slot: (claim: unknown) => unknown } }) => {
  ctx.ui.slot({
    replace: "prompt.footer.status",
    render: () => <Statusline theme={ctx.theme} />,
  })
}

const plugin = { id, setup }
export default plugin
