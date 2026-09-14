// Shared reader for ~/.ywai/config.yaml — the tiny flat subset of YAML ywai
// writes (`key: value` lines, optionally quoted). The advisor and both
// vision-bridge entry points read their settings through this module; it is
// the single copy of the parser.
import { readFile } from "node:fs/promises"
import os from "node:os"
import path from "node:path"

export const YWAI_CONFIG_PATH = path.join(os.homedir(), ".ywai", "config.yaml")

/**
 * Pulls specific top-level scalars out of a YAML document.
 *
 * Only column-zero keys are considered, so a nested key that happens to share a
 * name (say `vision_model` under some profile block) cannot be mistaken for
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

export type YwaiConfig = {
  vision_model?: string
  vision_model_override?: string
}

/** Reads the vision model preference from ~/.ywai/config.yaml. */
export async function readYwaiConfig(): Promise<YwaiConfig> {
  try {
    const raw = await readFile(YWAI_CONFIG_PATH, "utf8")
    const fields = readTopLevelFields(raw, ["vision_model", "vision_model_override"])
    return {
      vision_model: fields.vision_model,
      vision_model_override: fields.vision_model_override,
    }
  } catch {
    return {}
  }
}
