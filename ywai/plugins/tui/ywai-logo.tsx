// @ts-nocheck
/** @jsxImportSource @opentui/solid */
import type { TuiPlugin, TuiThemeCurrent } from "@opencode-ai/plugin/tui"
import { useTerminalDimensions } from "@opentui/solid"
import { createMemo, createSignal } from "solid-js"

const id = "ywai-logo"

// Canonical ywai wordmark — kept in sync with internal/tui/tui.go logoLines.
const wordmark = [
  "██╗   ██╗██╗    ██╗ █████╗ ██╗",
  "╚██╗ ██╔╝██║    ██║██╔══██╗██║",
  " ╚████╔╝ ██║ █╗ ██║███████║██║",
  "  ╚██╔╝  ██║███╗██║██╔══██║██║",
  "   ██║   ╚███╔███╔╝██║  ██║██║",
  "   ╚═╝    ╚══╝╚══╝ ╚═╝  ╚═╝╚═╝",
]

const compactArt = ["✦ ywai ✦"]

// Brand palette from the ywai icon (icon.svg): orange, blue, purple.
const brand = ["#fd6421", "#0068fd", "#4d39c3"]

// Easter eggs unlocked by clicking the logo. Each entry is the tagline shown
// once the click count reaches its threshold; clicking cycles the accent color.
const eggs = [
  { at: 3, text: "you found me 🎉" },
  { at: 7, text: "yoizen ai · keep clicking…" },
  { at: 12, text: "ok ok, you really like clicking" },
  { at: 21, text: "✦ certified ywai gentleman ✦" },
]

const taglineFor = (clicks: number): string | null => {
  let unlocked: string | null = null
  for (const egg of eggs) {
    if (clicks >= egg.at) unlocked = egg.text
  }
  return unlocked
}

const Logo = (props: { theme: TuiThemeCurrent }) => {
  const dim = useTerminalDimensions()
  const [clicks, setClicks] = createSignal(0)

  const lines = createMemo(() => {
    const term = dim()
    return term.height >= wordmark.length + 6 && term.width >= 64 ? wordmark : compactArt
  })

  // Default to the theme accent; once the user starts clicking, cycle the brand
  // palette so every click visibly changes the wordmark color.
  const accent = createMemo(() => {
    const n = clicks()
    return n === 0 ? props.theme.accent : brand[(n - 1) % brand.length]
  })

  const tagline = createMemo(() => taglineFor(clicks()))

  return (
    <box
      flexDirection="column"
      alignItems="center"
      onMouseDown={() => setClicks((n) => n + 1)}
    >
      {lines().map((line) => (
        <text fg={accent()}>{line}</text>
      ))}
      {tagline() ? <text fg={props.theme.accent}>{tagline()}</text> : null}
    </box>
  )
}

const tui: TuiPlugin = async (api) => {
  api.slots.register({
    id,
    order: 100,
    slots: {
      home_logo(ctx) {
        return <Logo theme={ctx.theme.current} />
      },
    },
  })
}

const plugin = { id, tui }
export default plugin
