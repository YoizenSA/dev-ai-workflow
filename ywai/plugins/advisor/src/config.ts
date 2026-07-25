/**
 * Advisor settings, read from ywai's config, plus the small helpers that decide
 * how a note is presented.
 *
 * These live outside `index.ts` on purpose: OpenCode's plugin loader treats
 * every exported function of a plugin module as a plugin factory, so a module
 * that exports helpers — let alone classes — fails to load entirely, and does
 * so silently.
 */

import * as fs from "node:fs/promises"
import * as os from "node:os"
import * as path from "node:path"
import type { Severity } from "./emission-guard"

export type ModelRef = { providerID: string; modelID: string }

export type AdvisorConfig = {
  enabled: boolean
  model?: ModelRef
}

export const CONFIG_PATH = path.join(os.homedir(), ".ywai", "config.yaml")

/**
 * Reads the advisor settings from ywai's config.
 *
 * The file is YAML, and both keys sit at the top level, so a line scan reads
 * them without pulling a YAML parser into the bundle — the same approach
 * vision-bridge uses for its model preference.
 */
export async function loadConfig(configPath = CONFIG_PATH): Promise<AdvisorConfig> {
  try {
    const raw = await fs.readFile(configPath, "utf8")
    const fields = readTopLevelFields(raw, ["advisor_enabled", "advisor_model"])
    const model = parseModelRef(fields.advisor_model)
    return { enabled: fields.advisor_enabled === "true" && model !== undefined, model }
  } catch {
    return { enabled: false }
  }
}

/**
 * Pulls specific top-level scalars out of a YAML document.
 *
 * Only column-zero keys are considered, so a nested key that happens to share a
 * name (say `advisor_model` under some profile block) cannot be mistaken for
 * the global setting.
 */
export function readTopLevelFields(raw: string, keys: string[]): Record<string, string> {
  const wanted = new Set(keys)
  const out: Record<string, string> = {}
  for (const line of raw.split("\n")) {
    const m = /^([a-z_]+):\s*(.*)$/.exec(line)
    if (!m) continue
    const key = m[1]
    if (!key || !wanted.has(key)) continue
    let value = (m[2] ?? "").trim()
    if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1)
    }
    out[key] = value
  }
  return out
}

/**
 * Accepts "provider/model" or a bare id. A bare id cannot be resolved without
 * the catalog, so it is rejected rather than guessed — advising with the wrong
 * model is worse than not advising.
 */
export function parseModelRef(value: unknown): ModelRef | undefined {
  if (typeof value !== "string") return undefined
  const trimmed = value.trim()
  const slash = trimmed.indexOf("/")
  if (slash <= 0 || slash === trimmed.length - 1) return undefined
  return { providerID: trimmed.slice(0, slash), modelID: trimmed.slice(slash + 1) }
}

/**
 * Project-specific review priorities: things worth flagging to a reviewer but
 * too noisy to put in the executing agent's own prompt.
 */
export async function loadWatchdog(directory: string): Promise<string | undefined> {
  for (const candidate of [path.join(directory, "WATCHDOG.md"), path.join(directory, ".ywai", "WATCHDOG.md")]) {
    try {
      const text = await fs.readFile(candidate, "utf8")
      if (text.trim()) return text
    } catch {
      // absent is the normal case
    }
  }
  return undefined
}

/** Toast variant per severity — a nit must not look like a blocker. */
export function toastVariant(severity: Severity): "info" | "warning" | "error" {
  if (severity === "blocker") return "error"
  if (severity === "concern") return "warning"
  return "info"
}
