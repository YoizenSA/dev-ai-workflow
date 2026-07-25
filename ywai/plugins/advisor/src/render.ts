/**
 * Renders a turn for the advisor.
 *
 * A reviewer that only reads prose is reviewing the agent's account of itself —
 * the one source it should trust least. "Did it claim done on work it never
 * exercised?" and "is the verification thinner than the risk?" are answerable
 * only from what actually ran, so tool calls belong in the material.
 *
 * What does NOT belong is their output. A tool call renders as one line — name,
 * its most identifying argument, how it ended, and how much came back:
 *
 *     → bash(npm test) ⇒ error · 12 lines — 2 failing
 *
 * That is enough to catch a claim of done sitting on top of a failed command,
 * which is the judgment being asked for. Shipping the output instead would cost
 * hundreds of tokens per call to answer a question the summary already answers.
 * (Shape borrowed from oh-my-pi, whose advisor renders the same way.)
 */

/** A tool invocation as OpenCode records it on a message part. */
export type ToolPart = {
  type: "tool"
  tool?: string
  state?: {
    status?: string
    input?: Record<string, unknown>
    output?: string
    title?: string
  }
}

export type AnyPart = { type?: string; text?: string } & Partial<ToolPart>

/**
 * Tools that change something. A turn containing one is worth reviewing; a turn
 * that only looked around usually is not, and paying a second model to watch an
 * agent read files is how this feature stops being worth its cost.
 */
const MUTATING_TOOLS: ReadonlySet<string> = new Set([
  "edit",
  "write",
  "patch",
  "multiedit",
  "bash",
  "ast_edit",
  "task",
  "delegate",
])

export function isMutatingTool(name: string | undefined): boolean {
  if (!name) return false
  const base = name.toLowerCase()
  if (MUTATING_TOOLS.has(base)) return true
  // Plugin tools namespace themselves (`ado_pr_create`, `codemod_run`), so a
  // mutating verb anywhere in the name counts.
  return /(^|_)(edit|write|patch|create|update|delete|remove|run|exec|apply)(_|$)/.test(base)
}

/**
 * Argument keys that identify a call, most specific first. `read(src/a.ts)`
 * says what happened; `read(offset: 0, limit: 200)` does not.
 */
const PRIMARY_ARG_KEYS = ["command", "filePath", "file_path", "path", "pattern", "query", "prompt", "url"]

const MAX_ARG = 120

function oneLine(value: string, max = MAX_ARG): string {
  const flat = value.replace(/\s+/g, " ").trim()
  return flat.length <= max ? flat : `${flat.slice(0, max)}…`
}

/** Picks the one argument that best identifies a call. */
export function primaryArg(name: string, input: Record<string, unknown> | undefined): string {
  if (!input || typeof input !== "object") return ""

  if (name === "grep") {
    const pattern = str(input.pattern)
    const where = str(input.path) || str(input.include)
    if (pattern && where) return oneLine(`${pattern} @ ${where}`)
    if (pattern) return oneLine(pattern)
  }

  for (const key of PRIMARY_ARG_KEYS) {
    const v = str(input[key])
    if (v) return oneLine(v)
  }

  // Nothing recognizable: the first short string argument, else nothing. A blob
  // of JSON here would cost tokens without identifying anything.
  for (const v of Object.values(input)) {
    const s = str(v)
    if (s) return oneLine(s)
  }
  return ""
}

function str(v: unknown): string {
  return typeof v === "string" ? v.trim() : ""
}

function lineCount(text: string): number {
  if (!text) return 0
  return text.split("\n").length
}

/**
 * One line per tool call.
 *
 * The failure preview is the exception to "no output": a command that failed is
 * the highest-signal thing in a turn, and its first line is usually the reason.
 */
export function renderToolCall(part: AnyPart): string {
  const name = part.tool ?? "tool"
  const state = part.state ?? {}
  const head = `→ ${name}(${primaryArg(name, state.input)})`

  const status = (state.status ?? "").toLowerCase()
  if (status && status !== "completed" && status !== "error") {
    return `${head} ⇒ ${status}`
  }

  const output = typeof state.output === "string" ? state.output : ""
  const lines = lineCount(output)
  const size = `${lines} ${lines === 1 ? "line" : "lines"}`

  if (status === "error") {
    const first = oneLine(output.split("\n").find((l) => l.trim()) ?? "", 160)
    return first ? `${head} ⇒ error · ${size} — ${first}` : `${head} ⇒ error · ${size}`
  }
  return `${head} ⇒ ok · ${size}`
}

/** Plain text of a message, ignoring tool and reasoning parts. */
export function renderText(parts: AnyPart[]): string {
  return (Array.isArray(parts) ? parts : [])
    .filter((p) => p?.type === "text" && typeof p.text === "string")
    .map((p) => (p.text ?? "").trim())
    .filter(Boolean)
    .join("\n")
}

/** Every tool call in a message, one per line. */
export function renderTools(parts: AnyPart[]): string {
  return (Array.isArray(parts) ? parts : [])
    .filter((p) => p?.type === "tool")
    .map(renderToolCall)
    .join("\n")
}

/** Whether a message changed anything. */
export function hasMutatingTool(parts: AnyPart[]): boolean {
  return (Array.isArray(parts) ? parts : []).some((p) => p?.type === "tool" && isMutatingTool(p.tool))
}

/** Whether a message called any tool at all. */
export function hasAnyTool(parts: AnyPart[]): boolean {
  return (Array.isArray(parts) ? parts : []).some((p) => p?.type === "tool")
}

/** Whether any tool call in a message failed. */
export function hasFailedTool(parts: AnyPart[]): boolean {
  return (Array.isArray(parts) ? parts : []).some(
    (p) => p?.type === "tool" && (p.state?.status ?? "").toLowerCase() === "error",
  )
}
