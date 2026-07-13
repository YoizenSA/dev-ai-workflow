/**
 * vision-bridge — OpenCode plugin
 *
 * When the active model cannot accept image input (e.g. deepseek-v4-flash),
 * images attached by the user are analyzed via TokenBank's vision models and
 * replaced with text so the text-only model can reason about them.
 *
 * Covers "I pasted an image" for text-only models without requiring native
 * vision support or a separate MCP server.
 */

import * as fs from "node:fs/promises"
import * as os from "node:os"
import * as path from "node:path"
import type { Plugin } from "@opencode-ai/plugin"

type AnyPart = {
  id?: string
  type: string
  text?: string
  mime?: string
  url?: string
  filename?: string
  messageID?: string
  sessionID?: string
  synthetic?: boolean
  [key: string]: unknown
}

type YwaiConfig = {
  tokenbank_url?: string
  tokenbank_api_key?: string
  vision_model?: string
  vision_model_override?: string
}

function stripProviderPrefix(id: string): string {
  const i = id.lastIndexOf("/")
  return i >= 0 ? id.slice(i + 1) : id
}

async function readYwaiConfig(): Promise<YwaiConfig> {
  const cfgPath = path.join(os.homedir(), ".ywai", "config.yaml")
  try {
    const raw = await fs.readFile(cfgPath, "utf8")
    const out: YwaiConfig = {}
    for (const line of raw.split("\n")) {
      const m = line.match(/^([a-z_]+):\s*(.*)$/)
      if (!m) continue
      const key = m[1]
      let val = m[2].trim()
      if (
        (val.startsWith('"') && val.endsWith('"')) ||
        (val.startsWith("'") && val.endsWith("'"))
      ) {
        val = val.slice(1, -1)
      }
      if (
        key === "tokenbank_url" ||
        key === "tokenbank_api_key" ||
        key === "vision_model" ||
        key === "vision_model_override"
      ) {
        out[key] = val
      }
    }
    return out
  } catch {
    return {}
  }
}

async function modelSupportsImage(
  providerID: string,
  modelID: string,
): Promise<boolean> {
  // Prefer live TokenBank metadata when available.
  const cfg = await readYwaiConfig()
  if (cfg.tokenbank_url && cfg.tokenbank_api_key) {
    try {
      const res = await fetch(
        `${cfg.tokenbank_url.replace(/\/$/, "")}/api/setup/models`,
        { headers: { Authorization: `Bearer ${cfg.tokenbank_api_key}` } },
      )
      if (res.ok) {
        const body = (await res.json()) as {
          models?: Array<{
            id: string
            vision?: boolean
            modalities?: { input?: string[] }
          }>
        }
        const m = body.models?.find((x) => x.id === modelID)
        if (m) {
          if (m.vision) return true
          if (m.modalities?.input?.includes("image")) return true
          return false
        }
      }
    } catch {
      // fall through to opencode.json
    }
  }

  // Fallback: opencode.json provider entry.
  try {
    const ocPath = path.join(os.homedir(), ".config", "opencode", "opencode.json")
    const oc = JSON.parse(await fs.readFile(ocPath, "utf8")) as {
      provider?: Record<
        string,
        {
          models?: Record<
            string,
            { attachment?: boolean; modalities?: { input?: string[] } }
          >
        }
      >
    }
    const entry = oc.provider?.[providerID]?.models?.[modelID]
    if (!entry) return false
    if (entry.modalities?.input?.includes("image")) return true
    return Boolean(entry.attachment && entry.modalities?.input?.includes("image"))
  } catch {
    return false
  }
}

async function resolveVisionModel(cfg: YwaiConfig): Promise<string> {
  // Settings is the source of truth: vision_model_override > vision_model.
  const preferred = stripProviderPrefix(
    (cfg.vision_model_override || cfg.vision_model || "").trim(),
  )
  if (preferred) {
    return preferred
  }
  // Empty setting → first vision model from TokenBank catalog.
  if (!cfg.tokenbank_url || !cfg.tokenbank_api_key) {
    return ""
  }
  try {
    const res = await fetch(
      `${cfg.tokenbank_url.replace(/\/$/, "")}/api/setup/models`,
      { headers: { Authorization: `Bearer ${cfg.tokenbank_api_key}` } },
    )
    if (!res.ok) return ""
    const body = (await res.json()) as {
      models?: Array<{
        id: string
        vision?: boolean
        modalities?: { input?: string[] }
      }>
    }
    const vision = (body.models ?? []).filter(
      (m) => m.vision || m.modalities?.input?.includes("image"),
    )
    return vision[0]?.id ?? ""
  } catch {
    return ""
  }
}

async function toDataURI(part: AnyPart): Promise<string | null> {
  const url = part.url ?? ""
  if (url.startsWith("data:image/")) return url
  if (url.startsWith("file://")) {
    try {
      const filePath = decodeURIComponent(url.replace("file://", ""))
      const buf = await fs.readFile(filePath)
      const mime = part.mime || "image/png"
      return `data:${mime};base64,${buf.toString("base64")}`
    } catch {
      return null
    }
  }
  // Absolute path without scheme (rare)
  if (url.startsWith("/") || /^[A-Za-z]:\\/.test(url)) {
    try {
      const buf = await fs.readFile(url)
      const mime = part.mime || "image/png"
      return `data:${mime};base64,${buf.toString("base64")}`
    } catch {
      return null
    }
  }
  return null
}

async function analyzeImage(
  cfg: YwaiConfig,
  visionModel: string,
  dataURI: string,
  prompt: string,
): Promise<string> {
  if (!cfg.tokenbank_url || !cfg.tokenbank_api_key) {
    throw new Error("TokenBank not configured in ~/.ywai/config.yaml")
  }
  if (!visionModel) {
    throw new Error("No vision model available from TokenBank")
  }

  const res = await fetch(
    `${cfg.tokenbank_url.replace(/\/$/, "")}/v1/chat/completions`,
    {
      method: "POST",
      headers: {
        Authorization: `Bearer ${cfg.tokenbank_api_key}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        model: visionModel,
        max_tokens: 1500,
        messages: [
          {
            role: "user",
            content: [
              { type: "text", text: prompt },
              { type: "image_url", image_url: { url: dataURI } },
            ],
          },
        ],
      }),
    },
  )

  const text = await res.text()
  if (!res.ok) {
    throw new Error(`TokenBank vision error (${res.status}): ${text.slice(0, 300)}`)
  }
  const body = JSON.parse(text) as {
    choices?: Array<{ message?: { content?: string | null } }>
  }
  const content = body.choices?.[0]?.message?.content
  if (!content) {
    throw new Error("TokenBank vision returned empty content")
  }
  return content
}

const VisionBridgePlugin: Plugin = async () => {
  return {
    "chat.message": async (input, output) => {
      const model = input.model
      if (!model?.modelID || !model.providerID) return

      const parts = output.parts as AnyPart[]
      const imageParts = parts.filter(
        (p) => p.type === "file" && typeof p.mime === "string" && p.mime.startsWith("image/"),
      )
      if (imageParts.length === 0) return

      const supports = await modelSupportsImage(model.providerID, model.modelID)
      if (supports) return

      const cfg = await readYwaiConfig()
      const visionModel = await resolveVisionModel(cfg)
      const userText = parts
        .filter((p) => p.type === "text" && p.text)
        .map((p) => p.text!.trim())
        .filter(Boolean)
        .join("\n")
      const prompt =
        userText ||
        "Describe this image in detail. Note any text, UI, errors, or relevant visual information."

      const next: AnyPart[] = []
      for (const part of parts) {
        if (!(part.type === "file" && part.mime?.startsWith("image/"))) {
          next.push(part)
          continue
        }

        try {
          const dataURI = await toDataURI(part)
          if (!dataURI) {
            next.push({
              id: part.id,
              type: "text",
              messageID: part.messageID,
              sessionID: part.sessionID,
              synthetic: true,
              text: `[Vision bridge] Could not read image ${part.filename ?? "(unnamed)"}.`,
            })
            continue
          }
          const analysis = await analyzeImage(cfg, visionModel, dataURI, prompt)
          next.push({
            id: part.id,
            type: "text",
            messageID: part.messageID,
            sessionID: part.sessionID,
            synthetic: true,
            text:
              `[Vision bridge → ${visionModel}] The active model (${model.modelID}) cannot see images. ` +
              `Analysis from the vision model:\n\n${analysis}`,
          })
        } catch (err) {
          const msg = err instanceof Error ? err.message : String(err)
          next.push({
            id: part.id,
            type: "text",
            messageID: part.messageID,
            sessionID: part.sessionID,
            synthetic: true,
            text: `[Vision bridge failed] ${msg}`,
          })
        }
      }

      // Mutate in place — OpenCode reads output.parts after the hook.
      output.parts.length = 0
      output.parts.push(...(next as typeof output.parts))
    },
  }
}

export default VisionBridgePlugin
