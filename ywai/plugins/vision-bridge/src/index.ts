/**
 * vision-bridge — OpenCode plugin
 *
 * When the active model cannot accept image input (e.g. deepseek-v4-flash,
 * tokenbank's gpt-5.6-luna), images attached by the user are analyzed by a
 * vision model and replaced with text so the text-only model can reason
 * about them.
 *
 * The analysis runs through OpenCode itself (client.session.prompt), so the
 * vision model is any model OpenCode already knows — provider resolution,
 * credentials and wire format are OpenCode's job, not ours. No separate
 * endpoint or API key is configured here.
 *
 * IMPORTANT: this file must export the plugin as its ONLY export. OpenCode's
 * loader calls every exported function as a plugin factory, so a stray helper
 * export throws during registration and the plugin never loads. Helpers and
 * their unit-test surface live in ./capabilities.
 */

import * as fs from "node:fs/promises"
import * as os from "node:os"
import * as path from "node:path"
import type { Plugin, PluginInput } from "@opencode-ai/plugin"
import {
  buildVisionPrompt,
  catalogEntrySupportsImage,
  collectUserText,
  extractText,
  formatModel,
  modelSupportsImage,
  resolveImageSupport,
  resolveVisionModel,
  type AnyPart,
  type CatalogModel,
  type CatalogProvider,
  type ModelRef,
} from "./capabilities.js"

type YwaiConfig = {
  vision_model?: string
  vision_model_override?: string
}

/** Reads the vision model preference from ~/.ywai/config.yaml. */
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
      if (key === "vision_model" || key === "vision_model_override") {
        out[key as keyof YwaiConfig] = val
      }
    }
    return out
  } catch {
    return {}
  }
}

/** Fetches OpenCode's live provider catalog; empty on any failure. */
async function loadCatalog(client: PluginInput["client"]): Promise<CatalogProvider[]> {
  try {
    const result = await client.provider.list()
    return (result.data?.all ?? []) as CatalogProvider[]
  } catch {
    return []
  }
}

/** opencode.json capability declaration — authoritative over the live catalog. */
async function opencodeConfigSupportsImage(
  providerID: string,
  modelID: string,
): Promise<boolean | undefined> {
  try {
    const ocPath = path.join(os.homedir(), ".config", "opencode", "opencode.json")
    const oc = JSON.parse(await fs.readFile(ocPath, "utf8")) as {
      provider?: Record<string, { models?: Record<string, CatalogModel> }>
    }
    const entry = oc.provider?.[providerID]?.models?.[modelID]
    if (!entry) return undefined
    return catalogEntrySupportsImage(entry)
  } catch {
    return undefined
  }
}

const VISION_SYSTEM_PROMPT = [
  "You are a vision assistant. Look at the attached image carefully.",
  "Respond in clear English.",
  "First describe exactly what you see: layout, UI elements, text (transcribe it), colors, icons, logos, errors, and any important visual details.",
  "Be specific and factual. Do not invent content that is not visible.",
  "When the user's own message is included, treat it as the reason the image was",
  "attached: answer it directly after the description, and let it guide which",
  "details matter most.",
].join("\n")

/**
 * Analyzes one image by prompting the vision model through OpenCode.
 *
 * Runs in a throwaway child session so the analysis never lands in the user's
 * conversation, and with tools disabled so it is a plain completion.
 */
async function analyzeImage(
  client: PluginInput["client"],
  bridgeSessions: Set<string>,
  parentID: string,
  visionModel: ModelRef,
  imagePart: AnyPart,
  userText: string,
): Promise<string> {
  const created = await client.session.create({
    body: { parentID, title: "vision-bridge" },
  })
  const sessionID = (created.data as { id?: string } | undefined)?.id
  if (!sessionID) {
    throw new Error("could not create the analysis session")
  }

  // Marked before prompting: the nested chat.message hook must skip this
  // session instead of trying to bridge the image a second time.
  bridgeSessions.add(sessionID)
  try {
    const res = await client.session.prompt({
      path: { id: sessionID },
      body: {
        model: visionModel,
        system: VISION_SYSTEM_PROMPT,
        tools: {},
        parts: [
          { type: "text", text: buildVisionPrompt(userText) },
          {
            type: "file",
            mime: imagePart.mime || "image/png",
            url: imagePart.url ?? "",
            ...(imagePart.filename ? { filename: imagePart.filename } : {}),
          },
        ],
      },
    })

    const parts = (res.data?.parts ?? []) as Array<{ type?: string; text?: string }>
    const text = extractText(parts)
    if (!text) throw new Error("the vision model returned no text")
    return text
  } finally {
    bridgeSessions.delete(sessionID)
    // Best-effort cleanup — a leftover child session is noise, not a failure.
    try {
      await client.session.delete({ path: { id: sessionID } })
    } catch {
      // ignore
    }
  }
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
  // Sessions this plugin drives itself. Prompting the vision model re-enters
  // chat.message with the same image part, so without this the bridge would
  // recurse whenever the vision model is misconfigured as non-vision.
  const bridgeSessions = new Set<string>()

  return {
    "chat.message": async (input, output) => {
      if (input.sessionID && bridgeSessions.has(input.sessionID)) return

      const model = input.model
      if (!model?.modelID || !model.providerID) return

      const parts = output.parts as AnyPart[]
      const imageParts = parts.filter(
        (p) => p.type === "file" && typeof p.mime === "string" && p.mime.startsWith("image/"),
      )
      if (imageParts.length === 0) return

      const catalog = await loadCatalog(client)
      const supports = resolveImageSupport(
        await opencodeConfigSupportsImage(model.providerID, model.modelID),
        modelSupportsImage(catalog, model.providerID, model.modelID),
      )
      if (supports) return

      const cfg = await readYwaiConfig()
      const visionModel = resolveVisionModel(
        catalog,
        cfg.vision_model_override || cfg.vision_model || "",
      )
      const userText = collectUserText(parts)

      const n = imageParts.length
      if (!visionModel) {
        // Say which model triggered the bridge — otherwise the user cannot tell
        // why it ran at all.
        await showToast(
          client,
          `${formatModel(model)} cannot see images and no vision model is configured`,
          "error",
        )
        const notice =
          `[Vision bridge] ${formatModel(model)} cannot accept images and no vision model ` +
          `is available. Set one in ywai Settings → Vision bridge.`
        output.parts.length = 0
        output.parts.push(
          ...(parts.map((p) =>
            p.type === "file" && p.mime?.startsWith("image/")
              ? {
                  id: p.id,
                  type: "text",
                  messageID: p.messageID,
                  sessionID: p.sessionID,
                  synthetic: true,
                  text: notice,
                }
              : p,
          ) as typeof output.parts),
        )
        return
      }

      const modelLabel = formatModel(visionModel)
      await showToast(
        client,
        n === 1
          ? `${formatModel(model)} has no vision — analyzing image with ${modelLabel}…`
          : `${formatModel(model)} has no vision — analyzing ${n} images with ${modelLabel}…`,
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
          const analysis = await analyzeImage(
            client,
            bridgeSessions,
            input.sessionID,
            visionModel,
            part,
            userText,
          )
          ok++
          next.push({
            id: part.id,
            type: "text",
            messageID: part.messageID,
            sessionID: part.sessionID,
            synthetic: true,
            // Keep wording positive: "cannot see images" made chat models claim the bridge failed.
            text:
              `[Image analysis via ${modelLabel}]\n` +
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
              `[Vision bridge error] Could not analyze the attached image with ${modelLabel}. ` +
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
