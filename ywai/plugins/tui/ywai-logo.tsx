// @ts-nocheck
/** @jsxImportSource @opentui/solid */
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

const mediumMark = "◆  Ywai  ◆"
const compactArt = "✦ Ywai ✦"

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

type VersionInfo = { installed?: string; latest?: string; updateAvailable?: boolean }

const readVersionInfo = (): VersionInfo => {
  try {
    const raw = readFileSync(join(homedir(), ".ywai", "version.json"), "utf8")
    return JSON.parse(raw) as VersionInfo
  } catch {
    return {}
  }
}

const tag = (v?: string) => (v ? (v.startsWith("v") ? v : `v${v}`) : "")

// v2 ResolvedTheme is nested (theme.text.subdued, theme.text.feedback.*).
// Older hosts passed flat keys. Prefer v2; fall back so a missing token
// never paints the default (white) fg.
const themeMuted = (theme) => theme?.text?.subdued ?? theme?.textMuted ?? theme?.text?.default
const themeAccent = (theme) =>
  theme?.text?.feedback?.info?.default ?? theme?.accent ?? theme?.text?.action?.primary?.default ?? theme?.text?.default
const themeWarning = (theme) =>
  theme?.text?.feedback?.warning?.default ?? theme?.text?.status?.unread ?? themeAccent(theme)

const aboutMessage = (info: VersionInfo) => {
  const installed = info.installed ? `installed ${tag(info.installed)}` : "installed version unknown"
  if (info.updateAvailable) return `${installed}\n↑ ${tag(info.latest)} available — run \`ywai update\``
  return info.installed ? `${installed}\nup to date` : installed
}

const showAbout = (ctx) => {
  const message = aboutMessage(readVersionInfo())
  if (ctx.ui?.dialog?.alert) {
    void ctx.ui.dialog.alert({ title: "ywai", message })
    return
  }
  ctx.ui?.toast?.show?.({ title: "ywai", message, variant: "info" })
}

const GlyphRow = (props: { line: string; row: number; rows: number; phase: number; flash: number }) => (
  <box flexDirection="row">
    <Index each={props.line.split("")}>
      {(ch, i) => {
        const base = gradient(i / Math.max(props.line.length - 1, 1) + props.row * 0.12 - props.phase)
        const color =
          props.flash > 0 ? rgba(mix(base, [255, 255, 255], (props.flash / 10) * 0.8)) : rgba(base)
        return <text fg={color}>{ch()}</text>
      }}
    </Index>
  </box>
)

const Logo = (props: { ctx: any; clicks: () => number; bump: () => void }) => {
  const dim = useTerminalDimensions()
  const [phase, setPhase] = createSignal(0)
  const [flash, setFlash] = createSignal(0)

  const timer = setInterval(() => {
    setFlash((f) => (f > 0 ? f - 1 : 0))
    setPhase((p) => p + (flash() > 0 ? 0.05 : 0.012))
  }, 70)
  onCleanup(() => clearInterval(timer))

  const size = createMemo(() => {
    const t = dim()
    if (t.height >= wordmark.length + 6 && t.width >= 64) return "full"
    if (t.width >= 28) return "medium"
    return "compact"
  })

  const tagline = createMemo(() => taglineFor(props.clicks()))
  const mark = createMemo(() => {
    if (size() === "full") return wordmark
    if (size() === "medium") return [mediumMark]
    return [compactArt]
  })

  return (
    <box
      flexDirection="column"
      alignItems="center"
      onMouseDown={() => {
        props.bump()
        setFlash(10)
      }}
    >
      <Index each={mark()}>
        {(line, row) => (
          <GlyphRow line={line()} row={row} rows={mark().length} phase={phase()} flash={flash()} />
        )}
      </Index>
      {tagline() ? <text fg={themeAccent(props.ctx.theme)}>{tagline()}</text> : null}
    </box>
  )
}

const VersionChip = (props: { ctx: any }) => {
  const [info, setInfo] = createSignal(readVersionInfo())
  const poll = setInterval(() => setInfo(readVersionInfo()), 30_000)
  onCleanup(() => clearInterval(poll))

  const label = createMemo(() => {
    const v = info()
    if (!v.installed && !v.updateAvailable) return ""
    if (v.updateAvailable) return `ywai ↑ ${tag(v.latest)}`
    return `ywai ${tag(v.installed)}`
  })

  return label() ? (
    <text
      fg={info().updateAvailable ? themeWarning(props.ctx.theme) : themeMuted(props.ctx.theme)}
      onMouseDown={() => {
        if (info().updateAvailable) {
          props.ctx.ui?.toast?.show?.({
            title: "ywai update",
            message: `run \`ywai update\`  (${tag(info().installed)} → ${tag(info().latest)})`,
            variant: "info",
          })
          return
        }
        showAbout(props.ctx)
      }}
    >
      {label()}
    </text>
  ) : null
}

const Commands = (props: { ctx: any }) => {
  props.ctx.keymap?.layer?.(() => ({
    mode: "global",
    commands: [
      {
        id: "ywai.about",
        title: "ywai version",
        group: "ywai",
        palette: true,
        slash: { name: "ywai" },
        run: () => showAbout(props.ctx),
      },
    ],
  }))
  return null
}

// OpenCode v2 TUI contract: `{ id, setup }`. SlotMap (packages/plugin/src/tui/context.ts)
// has no home.logo — a claim on an unpublished path is discarded in silence.
// Wordmark sits `before` home.footer (sibling above the footer, not inside it);
// version lives in home.footer.status (after health, before the host version).
const setup = (ctx) => {
  let clicks = () => 0
  let bump = () => {}
  try {
    const [store, update] = ctx.storage.store("ywai-logo-eggs", { initial: { clicks: 0 } })
    clicks = () => store.clicks
    bump = () => {
      void update((draft) => {
        draft.clicks += 1
      })
    }
  } catch {
    const [n, setN] = createSignal(0)
    clicks = n
    bump = () => setN((c) => c + 1)
  }

  const unsubs = [
    ctx.ui.slot({
      before: "home.footer",
      render: () => <Logo ctx={ctx} clicks={clicks} bump={bump} />,
    }),
    ctx.ui.slot({
      append: "home.footer.status",
      render: () => <VersionChip ctx={ctx} />,
    }),
    ctx.ui.slot({
      append: "app",
      render: () => <Commands ctx={ctx} />,
    }),
  ]

  return () => {
    for (const u of unsubs) {
      try {
        u?.()
      } catch {}
    }
  }
}

const plugin = { id, setup }
export default plugin
