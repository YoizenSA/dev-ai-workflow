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
import * as path from "node:path"
import {
  YWAI_CONFIG_PATH,
  readTopLevelFields,
} from "../../shared/ywai-config"
import type { Severity } from "./emission-guard"

export type ModelRef = { providerID: string; modelID: string }

export type AdvisorConfig = {
  enabled: boolean
  model?: ModelRef
}

export const CONFIG_PATH = YWAI_CONFIG_PATH

/**
 * Reads the advisor settings from ywai's config.
 *
 * The file is YAML, and both keys sit at the top level, so the shared line
 * scan in plugins/shared/ywai-config.ts reads them without pulling a YAML
 * parser into the bundle — vision-bridge reads its model preference through
 * the same module.
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

// Re-exported for the advisor's own tests; the implementation lives in the
// shared module so advisor and vision-bridge parse config.yaml identically.
export { readTopLevelFields }

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
