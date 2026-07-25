/**
 * The `/advisor` controls, exposed as tools so the setting can be changed from
 * inside the session it affects.
 *
 * Reaching for a second terminal to decide whether the reviewer of *this* run
 * should be on is the wrong shape: the decision belongs where the evidence is.
 *
 * The settings are written back to ywai's config.yaml by rewriting the two
 * lines in place. A YAML parser is not pulled into the bundle for it — these
 * are top-level scalars, and preserving the rest of the file byte-for-byte
 * matters more here than generality, since the user's own config lives there.
 */

import * as fs from "node:fs/promises"
import { CONFIG_PATH, loadConfig, parseModelRef } from "./config"

export type ControlResult = { ok: boolean; message: string }

/** Renders the current state for `/advisor` with no argument. */
export async function status(configPath = CONFIG_PATH): Promise<string> {
  const cfg = await loadConfig(configPath)
  const raw = await readModelRaw(configPath)

  if (!raw) {
    return "Advisor: off — no model set.\nSet one with `/advisor model <provider/model>`."
  }
  if (!cfg.enabled) {
    return `Advisor: off — model ${raw} is configured.\nTurn it on with \`/advisor on\`.`
  }
  return `Advisor: on, reviewing every turn with ${raw}.`
}

/** Sets the reviewing model. */
export async function setModel(value: string, configPath = CONFIG_PATH): Promise<ControlResult> {
  const model = parseModelRef(value)
  if (!model) {
    // Guessing a provider would review on a model the user did not choose and
    // bill for it silently.
    return { ok: false, message: `"${value}" is not a model reference. Use provider/model, e.g. anthropic/claude-sonnet-5.` }
  }

  await writeField(configPath, "advisor_model", `${model.providerID}/${model.modelID}`)
  const cfg = await loadConfig(configPath)
  const tail = cfg.enabled ? "" : "\nIt is still off — `/advisor on` enables it."
  return { ok: true, message: `Advisor model set to ${model.providerID}/${model.modelID}.${tail}\nRestart OpenCode to load it.` }
}

/** Turns the advisor on or off. */
export async function toggle(on: boolean, configPath = CONFIG_PATH): Promise<ControlResult> {
  if (on && !(await readModelRaw(configPath))) {
    return { ok: false, message: "No advisor model set — run `/advisor model <provider/model>` first." }
  }
  await writeField(configPath, "advisor_enabled", on ? "true" : "false")
  return {
    ok: true,
    message: `Advisor ${on ? "enabled" : "disabled"}. Restart OpenCode so it reloads the plugin.`,
  }
}

async function readModelRaw(configPath: string): Promise<string> {
  const cfg = await loadConfig(configPath)
  if (cfg.model) return `${cfg.model.providerID}/${cfg.model.modelID}`
  return ""
}

/**
 * Replaces a top-level scalar, appending it when absent.
 *
 * Only a line starting at column zero is touched, so a same-named key nested
 * under some profile block is left alone — and the rest of the file, which is
 * the user's, is never reformatted.
 */
export async function writeField(configPath: string, key: string, value: string): Promise<void> {
  let raw = ""
  try {
    raw = await fs.readFile(configPath, "utf8")
  } catch {
    raw = ""
  }

  const line = `${key}: ${value}`
  const pattern = new RegExp(`^${key}:.*$`, "m")

  const next = pattern.test(raw)
    ? raw.replace(pattern, line)
    : `${raw.replace(/\n*$/, "")}\n${line}\n`

  await fs.writeFile(configPath, next.startsWith("\n") ? next.slice(1) : next, "utf8")
}
