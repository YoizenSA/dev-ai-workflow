// @ts-nocheck
/** @jsxImportSource @opentui/solid */
import { useTerminalDimensions } from "@opentui/solid"
import { RGBA } from "@opentui/core"
import { createMemo, createSignal, onCleanup, Index, Show } from "solid-js"
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

// Smaller typeface of the same box-drawing family (single-stroke, ~15 cols).
// The 6-line mark is filled ██╗; this is the light ╦╚╝ cut for mid-height
// terminals where the heavy 3-line crop still collided with the prompt.
const smallMark = [
  "╦ ╦ ╦   ╦ ╔═╗ ╦",
  "╚╦╝ ╦ ╦ ╦ ╠═╣ ║",
  " ╩   ╩ ╩  ╩ ╩ ╩",
]

const mediumMark = "◆  Ywai  ◆"
const compactArt = "✦ ywai"

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

type Egg = { at: number; text: string; toast?: string }

// Click ladder. Toasts fire only on the click that lands on `at` (not on load,
// so a persisted gentleman does not dump the whole campaign at startup).
const eggs: Egg[] = [
  { at: 3, text: "hey — the logo is a button.", toast: "you found me" },
  { at: 7, text: "yoizen ai · keep going", toast: "ywai" },
  { at: 13, text: "still not a skill.", toast: "nice try" },
  { at: 21, text: "certified ywai gentleman", toast: "title unlocked" },
  { at: 34, text: "ponytail says YAGNI. you clicked anyway.", toast: "ponytail disapproves" },
  { at: 42, text: "42 clicks. the answer was ywai.", toast: "don't panic" },
  { at: 64, text: "the orchestrator filed a complaint.", toast: "delegation refused" },
  { at: 89, text: "i-have-adhd wants this loop back.", toast: "focus check" },
  { at: 100, text: "ok. go ship.", toast: "achievement: stop" },
]

const encore = [
  "the logo is a button. you made it one.",
  "ywai update --beta won't fix this",
  "delegate this urge to a subagent",
  "gentleman-programming would never",
  "one more for the workflow retro",
  "still not a skill",
]

const lastEgg = eggs[eggs.length - 1]

const eggFor = (clicks: number): Egg | null => {
  let found: Egg | null = null
  for (const egg of eggs) if (clicks >= egg.at) found = egg
  return found
}

const taglineFor = (clicks: number): string | null => {
  if (clicks < eggs[0].at) return null
  if (clicks >= lastEgg.at) return encore[(clicks - lastEgg.at) % encore.length]
  return eggFor(clicks)?.text ?? null
}

const speedFor = (clicks: number) => {
  if (clicks >= 100) return 3.2
  if (clicks >= 42) return 2.2
  if (clicks >= 21) return 1.4
  return 1
}

type VersionInfo = {
  installed?: string
  latest?: string
  latestStable?: string
  latestBeta?: string
  channel?: string
  updateCommand?: string
  updateAvailable?: boolean
  stableNewer?: boolean
}

const readVersionInfo = (): VersionInfo => {
  try {
    const raw = readFileSync(join(homedir(), ".ywai", "version.json"), "utf8")
    return JSON.parse(raw) as VersionInfo
  } catch {
    return {}
  }
}

const tag = (v?: string) => (v ? (v.startsWith("v") ? v : `v${v}`) : "")

const isBetaVer = (v?: string) => /-(beta|rc|pre|alpha)/i.test(v ?? "")

const channelOf = (info: VersionInfo) =>
  info.channel || (isBetaVer(info.installed) ? "beta" : "stable")

const updateCommand = (info: VersionInfo) =>
  info.updateCommand || (channelOf(info) === "beta" ? "ywai update --beta" : "ywai update")

const clip = (s: string, width: number) => {
  const max = Math.max(8, width - 4)
  return s.length > max ? `${s.slice(0, max - 1)}…` : s
}

const updateLines = (info: VersionInfo, width: number): string[] => {
  const lines: string[] = []
  const cmd = updateCommand(info)
  if (info.updateAvailable && info.latest) {
    const long = `↑ ${tag(info.latest)} — ${cmd}`
    if (width >= long.length + 2) lines.push(long)
    else {
      lines.push(`↑ ${tag(info.latest)}`)
      if (width >= cmd.length + 2) lines.push(cmd)
    }
  }
  if (channelOf(info) === "beta" && info.stableNewer && info.latestStable) {
    const long = `stable ${tag(info.latestStable)} — ywai update`
    if (width >= long.length + 2) lines.push(long)
    else lines.push(`stable ${tag(info.latestStable)}`)
  }
  return lines
}

// v2 ResolvedTheme is nested (theme.text.subdued, theme.text.feedback.*).
// Older hosts passed flat keys. Prefer v2; fall back so a missing token
// never paints the default (white) fg.
const themeMuted = (theme) => theme?.text?.subdued ?? theme?.textMuted ?? theme?.text?.default
const themeAccent = (theme) =>
  theme?.text?.feedback?.info?.default ?? theme?.accent ?? theme?.text?.action?.primary?.default ?? theme?.text?.default
const themeWarning = (theme) =>
  theme?.text?.feedback?.warning?.default ?? theme?.text?.status?.unread ?? themeAccent(theme)

const aboutMessage = (info: VersionInfo) => {
  const channel = channelOf(info)
  const installed = info.installed
    ? `installed ${tag(info.installed)} (${channel})`
    : "installed version unknown"
  const lines = [installed]
  if (info.updateAvailable && info.latest) {
    lines.push(`↑ ${tag(info.latest)} — ${updateCommand(info)}`)
  } else if (info.installed) {
    lines.push(channel === "beta" ? "up to date on beta" : "up to date")
  }
  if (channel === "beta" && info.stableNewer && info.latestStable) {
    lines.push(`stable ${tag(info.latestStable)} — ywai update`)
  }
  return lines.join("\n")
}

const showAbout = (ctx) => {
  const message = aboutMessage(readVersionInfo())
  if (ctx.ui?.dialog?.alert) {
    void ctx.ui.dialog.alert({ title: "ywai", message })
    return
  }
  ctx.ui?.toast?.show?.({ title: "ywai", message, variant: "info" })
}

const Logo = (props: { ctx: any; clicks: () => number; bump: () => void }) => {
  const dim = useTerminalDimensions()
  const [phase, setPhase] = createSignal(0)
  const [flash, setFlash] = createSignal(0)

  // Animation loop. ~14fps keeps a home screen lively without burning CPU.
  // Milestones speed the sweep up; the click flash still outruns the idle step.
  const timer = setInterval(() => {
    setFlash((f) => (f > 0 ? f - 1 : 0))
    const step = 0.012 * speedFor(props.clicks())
    setPhase((p) => p + (flash() > 0 ? Math.max(0.05, step) : step))
  }, 70)
  onCleanup(() => clearInterval(timer))

  const size = createMemo(() => {
    const t = dim() || { width: 80, height: 24 }
    const w = t.width ?? 80
    const h = t.height ?? 24
    // Host home already uses ~16 rows. The 6-line mark needs leftover
    // height or it paints over the prompt (the screenshot case).
    if (h >= 46 && w >= 70) return "full"
    if (h >= 34 && w >= 32) return "small"
    if (h >= 24 && w >= 22) return "medium"
    if (h >= 18) return "compact"
    return "hidden"
  })

  const tagline = createMemo(() => {
    const raw = taglineFor(props.clicks())
    if (!raw) return null
    return clip(raw, dim()?.width ?? 80)
  })
  const [version, setVersion] = createSignal(readVersionInfo())
  const versionPoll = setInterval(() => setVersion(readVersionInfo()), 30_000)
  onCleanup(() => clearInterval(versionPoll))
  const mark = createMemo(() => {
    if (size() === "full") return wordmark
    if (size() === "small") return smallMark
    if (size() === "medium") return [mediumMark]
    return [compactArt]
  })
  const hints = createMemo(() => updateLines(version(), dim()?.width ?? 80))
  const showMeta = () => size() === "full" || size() === "small" || size() === "medium"

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

  return (
    <Show when={size() !== "hidden"}>
    <box
      flexDirection="column"
      alignItems="center"
      onMouseDown={() => {
        const next = props.clicks() + 1
        const hit = eggs.find((e) => e.at === next)
        if (hit) {
          props.ctx.ui?.toast?.show?.({
            title: hit.toast ?? "ywai",
            message: hit.text,
            variant: "success",
          })
        } else if (version().updateAvailable) {
          props.ctx.ui?.toast?.show?.({
            title: channelOf(version()) === "beta" ? "ywai beta" : "ywai update",
            message: `${updateCommand(version())}  (${tag(version().installed)} → ${tag(version().latest)})`,
            variant: "info",
          })
        }
        props.bump()
        setFlash(hit ? 16 : 10)
      }}
    >
      <Index each={mark()}>
        {(line, row) => <text fg={rowColor(row, mark().length)}>{line()}</text>}
      </Index>
      {showMeta() && version().installed ? (
        <text fg={themeMuted(props.ctx.theme)}>{`ywai ${tag(version().installed)}`}</text>
      ) : null}
      <Index each={showMeta() ? hints() : []}>
        {(line) => <text fg={themeWarning(props.ctx.theme)}>{line()}</text>}
      </Index>
      {tagline() ? <text fg={themeAccent(props.ctx.theme)}>{tagline()}</text> : null}
    </box>
    </Show>
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
          const v = info()
          const extra =
            channelOf(v) === "beta" && v.stableNewer && v.latestStable
              ? `\nstable ${tag(v.latestStable)} — ywai update`
              : ""
          props.ctx.ui?.toast?.show?.({
            title: channelOf(v) === "beta" ? "ywai beta" : "ywai update",
            message: `${updateCommand(v)}  (${tag(v.installed)} → ${tag(v.latest)})${extra}`,
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
