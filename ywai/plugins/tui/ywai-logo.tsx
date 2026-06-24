// @ts-nocheck
/** @jsxImportSource @opentui/solid */
import type { TuiPlugin, TuiThemeCurrent } from "@opencode-ai/plugin/tui"
import { useTerminalDimensions } from "@opentui/solid"
import { RGBA } from "@opentui/core"
import { createMemo, createSignal, onCleanup, Index } from "solid-js"
import { readFileSync } from "node:fs"
import { homedir } from "node:os"
import { join } from "node:path"

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

const compactArt = "✦ ywai ✦"

// Brand palette from the ywai icon (icon.svg): orange, blue, purple.
type RGB = [number, number, number]
const brand: RGB[] = [
  [0xfd, 0x64, 0x21], // #fd6421 orange
  [0x00, 0x68, 0xfd], // #0068fd blue
  [0x4d, 0x39, 0xc3], // #4d39c3 purple
]

const lerp = (a: number, b: number, t: number) => a + (b - a) * t
const mix = (a: RGB, b: RGB, t: number): RGB => [
  lerp(a[0], b[0], t),
  lerp(a[1], b[1], t),
  lerp(a[2], b[2], t),
]

// fg expects an RGBA at runtime; build it from 0-255 ints to avoid any string
// parsing ambiguity (raw hex strings rendered as plain white).
const rgba = ([r, g, b]: RGB) => RGBA.fromInts(Math.round(r), Math.round(g), Math.round(b), 255)

// Sample a flowing gradient over the brand palette at position p (wraps in [0,1)).
const gradient = (p: number): RGB => {
  const n = brand.length
  const x = ((p % 1) + 1) % 1
  const seg = x * n
  const i = Math.floor(seg)
  return mix(brand[i % n], brand[(i + 1) % n], seg - i)
}

// Easter eggs unlocked by clicking the logo.
const eggs = [
  { at: 3, text: "you found me 🎉" },
  { at: 7, text: "yoizen ai · keep clicking…" },
  { at: 12, text: "ok ok, you really like clicking" },
  { at: 21, text: "✦ certified ywai gentleman ✦" },
]

const taglineFor = (clicks: number): string | null => {
  let unlocked: string | null = null
  for (const egg of eggs) if (clicks >= egg.at) unlocked = egg.text
  return unlocked
}

// ywai writes ~/.ywai/version.json (see internal/versionfile). Read it once at
// startup so the logo can show the installed version and flag updates. Best
// effort: if ywai was never installed or the file is missing, show nothing.
type VersionInfo = { installed?: string; latest?: string; updateAvailable?: boolean }

const readVersionInfo = (): VersionInfo => {
  try {
    const raw = readFileSync(join(homedir(), ".ywai", "version.json"), "utf8")
    return JSON.parse(raw) as VersionInfo
  } catch {
    return {}
  }
}

// Normalize a tag (GitHub prepends "v") for display.
const tag = (v?: string) => (v ? (v.startsWith("v") ? v : `v${v}`) : "")

const Logo = (props: { theme: TuiThemeCurrent }) => {
  const dim = useTerminalDimensions()
  const [clicks, setClicks] = createSignal(0)
  const [phase, setPhase] = createSignal(0)
  const [flash, setFlash] = createSignal(0) // remaining click-flash frames

  // Animation loop. ~14fps keeps a home screen lively without burning CPU.
  const timer = setInterval(() => {
    setFlash((f) => (f > 0 ? f - 1 : 0))
    setPhase((p) => p + (flash() > 0 ? 0.05 : 0.012))
  }, 70)
  onCleanup(() => clearInterval(timer))

  const big = createMemo(() => {
    const t = dim()
    return t.height >= wordmark.length + 6 && t.width >= 64
  })

  const tagline = createMemo(() => taglineFor(clicks()))
  const version = readVersionInfo()

  // Color for wordmark row `row` of `rows`: a vertical gradient sweep. During a
  // click flash each row brightens toward white for a satisfying burst.
  const rowColor = (row: number, rows: number) => {
    const base = gradient(row / rows - phase())
    if (flash() > 0) {
      const glow = flash() / 10 // 1 → 0 over the flash
      return rgba(mix(base, [255, 255, 255], glow * 0.8))
    }
    return rgba(base)
  }

  const full = () => (
    <Index each={wordmark}>
      {(line, row) => <text fg={rowColor(row, wordmark.length)}>{line()}</text>}
    </Index>
  )

  const compact = () => <text fg={rowColor(0, 1)}>{compactArt}</text>

  return (
    <box
      flexDirection="column"
      alignItems="center"
      onMouseDown={() => {
        setClicks((n) => n + 1)
        setFlash(10)
      }}
    >
      {big() ? full() : compact()}
      {version.installed ? (
        <text fg={props.theme.textMuted}>{`ywai ${tag(version.installed)}`}</text>
      ) : null}
      {version.updateAvailable ? (
        <text fg={props.theme.accent}>{`↑ ${tag(version.latest)} available — run \`ywai update\``}</text>
      ) : null}
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
