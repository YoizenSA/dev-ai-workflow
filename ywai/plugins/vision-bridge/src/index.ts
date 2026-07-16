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
import type { Plugin, PluginInput } from "@opencode-ai/plugin"

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
// Check whether a model supports image input using the provider-agnostic
// live catalog from OpenCode itself (client.provider.list), then falling
// back to TokenBank and opencode.json. Returns true (don't intercept)
// when the model's capability cannot be determined.
// Priority:
//   1. client.provider.list() — OpenCode's own live model catalog
//   2. TokenBank live catalog (when configured)
//   3. opencode.json provider entry
//   4. Unknown → assume vision (let images pass through)
async function modelSupportsImage(
  client: PluginInput["client"],
  providerID: string,
  modelID: string,
): Promise<boolean> {
  // 1. OpenCode's live provider catalog — authoritative, no hardcoded lists
  try {
    const result = await client.provider.list()
    const rawProviders = result.data?.all ?? []
    const providers = rawProviders as Array<{
      id: string
      models?: Record<string, { attachment?: boolean; modalities?: { input?: string[] } }>
    }>
    const prov = providers.find((p) => p.id === providerID)
    const model = prov?.models?.[modelID]
    if (model) {
      // OpenCode catalog found the model — its capability is authoritative.
      return model.modalities?.input?.includes("image") === true || model.attachment === true
    }
    } catch {
  }

  // 2. TokenBank live catalog
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
          return false // explicitly cataloged as non-vision
        }
      }
    } catch {
      // fall through
    }
  }

  // 3. opencode.json fallback
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
    if (entry) {
      if (entry.modalities?.input?.includes("image")) return true
      if (entry.attachment) return true
    }
  } catch {
    // fall through
  }

  // 4. Unknown → assume vision (safe default: let images pass through)
  return true
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
    choices?: Array<{
      message?: {
        content?: string | null | Array<{ type?: string; text?: string }>
        reasoning_content?: string | null
      }
    }>
  }
  const message = body.choices?.[0]?.message
  const content = normalizeAssistantText(message?.content, message?.reasoning_content)
  if (!content) {
    throw new Error("TokenBank vision returned empty content")
  }
  return content
}

/** Flatten OpenAI-style content + reasoning_content into a single string. */
function normalizeAssistantText(
  content: string | null | undefined | Array<{ type?: string; text?: string }>,
  reasoning?: string | null,
): string {
  let text = ""
  if (typeof content === "string") {
    text = content.trim()
  } else if (Array.isArray(content)) {
    text = content
      .map((part) => (typeof part?.text === "string" ? part.text : ""))
      .join("")
      .trim()
  }
  if (text) return text
  if (typeof reasoning === "string" && reasoning.trim()) {
    return reasoning.trim()
  }
  return ""
}

type ToastVariant = "info" | "success" | "warning" | "error"

/** Best-effort TUI toast; no-ops when no TUI is attached (headless). */
async function showToast(
  client: PluginInput["client"],
  message: string,
  variant: ToastVariant,
): Promise<void> {
  try {
    // client.tui is an optional OpenCode UI API.
    const clientAny = client as { tui?: { showToast?: (opts: unknown) => Promise<unknown> } }
    const tui = clientAny.tui
    if (!tui?.showToast) return
    await tui.showToast({
      body: { title: "Vision bridge", message, variant },
    })
  } catch {
    // No TUI attached; nothing to do.
  }
}

const VisionBridgePlugin: Plugin = async (ctx) => {
  const { client } = ctx

  return {
    "chat.message": async (input, output) => {
      const model = input.model
      if (!model?.modelID || !model.providerID) return

      const parts = output.parts as AnyPart[]
      const imageParts = parts.filter(
        (p) => p.type === "file" && typeof p.mime === "string" && p.mime.startsWith("image/"),
      )
      if (imageParts.length === 0) return

      const supports = await modelSupportsImage(client, model.providerID, model.modelID)
      if (supports) return
      const cfg = await readYwaiConfig()
      const visionModel = await resolveVisionModel(cfg)
      const userText = parts
        .filter((p) => p.type === "text" && p.text)
        .map((p) => p.text!.trim())
        .filter(Boolean)
        .join("\n")
      // Always ask the vision model (in English) for a concrete visual description.
      // If the user also wrote a question, answer it after describing what is visible.
      const prompt = [
        "You are a vision assistant. Look at the attached image carefully.",
        "Respond in clear English.",
        "First describe exactly what you see: layout, UI elements, text (transcribe it), colors, icons, logos, errors, and any important visual details.",
        "Be specific and factual. Do not invent content that is not visible.",
        userText
          ? `Then answer the user's request about the image:\n${userText}`
          : "If there is no further question, end with a short one-paragraph summary of the image.",
      ].join("\n")

      const n = imageParts.length
      const modelLabel = visionModel || "vision model"
      await showToast(
        client,
        n === 1
          ? `Analyzing image with ${modelLabel}…`
          : `Analyzing ${n} images with ${modelLabel}…`,
        "info",
      )

      const next: AnyPart[] = []
      let ok = 0
      let failed = 0
      for (const part of parts) {
        if (!(part.type === "file" && part.mime?.startsWith("image/"))) {
          next.push(part)
          continue
        }

        try {
          const dataURI = await toDataURI(part)
          if (!dataURI) {
            failed++
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
          ok++
          next.push({
            id: part.id,
            type: "text",
            messageID: part.messageID,
            sessionID: part.sessionID,
            synthetic: true,
            // Keep wording positive: "cannot see images" made chat models claim the bridge failed.
            text:
              `[Image analysis via ${visionModel}]\n` +
              `The user attached an image. Below is a description produced by the vision model ` +
              `(the chat model should treat this as ground truth about the image):\n\n${analysis}`,
          })
        } catch (err) {
          failed++
          const msg = err instanceof Error ? err.message : String(err)
          next.push({
            id: part.id,
            type: "text",
            messageID: part.messageID,
            sessionID: part.sessionID,
            synthetic: true,
            text:
              `[Vision bridge error] Could not analyze the attached image with the configured vision model. ` +
              `Details: ${msg}`,
          })
        }
      }

      if (failed > 0 && ok === 0) {
        await showToast(client, `Vision analysis failed (${failed})`, "error")
      } else if (failed > 0) {
        await showToast(
          client,
          `Analyzed ${ok}, failed ${failed} with ${modelLabel}`,
          "warning",
        )
      } else {
        await showToast(
          client,
          ok === 1
            ? `Image analyzed with ${modelLabel}`
            : `${ok} images analyzed with ${modelLabel}`,
          "success",
        )
      }

      // Mutate in place — OpenCode reads output.parts after the hook.
      output.parts.length = 0
      output.parts.push(...(next as typeof output.parts))
    },
  }
}

export default VisionBridgePlugin
