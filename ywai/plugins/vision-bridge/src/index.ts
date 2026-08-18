/**
 * vision-bridge — OpenCode v2 plugin
 *
 * When the active model cannot accept image input, attached images are
 * analyzed by a vision model and replaced with text.
 *
 * v2 API: { id, setup }. Local `define` identity — no runtime import of
 * @opencode-ai/plugin (local-file plugins cannot resolve node_modules).
 */

import * as fs from "node:fs/promises"
import * as os from "node:os"
import * as path from "node:path"
import {
  buildVisionPrompt,
  catalogEntrySupportsImage,
  collectUserText,
  extractText,
  formatModel,
  isImagePart,
  messageParts,
  modelSupportsImage,
  resolveImageSupport,
  resolveVisionModel,
  type AnyPart,
  type CatalogModel,
  type CatalogProvider,
  type ModelRef,
} from "./capabilities.js"

interface V2Plugin {
  id: string
  setup: (ctx: any) => Promise<(() => Promise<void>) | void> | (() => Promise<void>) | void
}
const define = (plugin: V2Plugin): V2Plugin => plugin

type YwaiConfig = {
  vision_model?: string
  vision_model_override?: string
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
      if (key === "vision_model" || key === "vision_model_override") {
        out[key as keyof YwaiConfig] = val
      }
    }
    return out
  } catch {
    return {}
  }
}

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

function providersFromCatalog(ctx: any): CatalogProvider[] {
  try {
    const list = ctx.catalog?.provider?.list?.() ?? []
    const out: CatalogProvider[] = []
    for (const rec of list) {
      const id = rec.provider?.id ?? rec.id
      if (!id) continue
      const models = rec.models instanceof Map ? Object.fromEntries(rec.models) : rec.models
      out.push({ id, models })
    }
    return out
  } catch {
    return []
  }
}

function catalogModelFromCtx(ctx: any, providerID: string, modelID: string): CatalogModel | undefined {
  try {
    return ctx.catalog?.model?.get?.(providerID, modelID) as CatalogModel | undefined
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

function imageUri(part: AnyPart): string {
  if (typeof part.url === "string" && part.url) return part.url
  if (typeof part.data === "string" && part.data) return part.data
  return ""
}

const plugin = define({
  id: "vision-bridge",
  async setup(ctx: any) {
    const bridgeSessions = new Set<string>()
    const bridgeText = new Map<string, string>()

    const subscribe = ctx.event?.subscribe?.bind(ctx.event)
    if (subscribe) {
      await subscribe((event: { type?: string; data?: Record<string, unknown> }) => {
        const sessionID = String(event?.data?.sessionID ?? "")
        if (!sessionID || !bridgeSessions.has(sessionID)) return
        const parts = (event.data?.parts ?? event.data?.content ?? []) as Array<{
          type?: string
          text?: string
        }>
        const text = extractText(parts)
        if (text) bridgeText.set(sessionID, text)
      })
    }

    const hook = ctx.session?.hook?.bind(ctx.session)
    if (!hook) return

    await hook("context", async (session: {
      sessionID?: string
      model?: ModelRef
      messages?: Array<{ role?: string; content?: AnyPart[]; parts?: AnyPart[] }>
    }) => {
      if (session.sessionID && bridgeSessions.has(session.sessionID)) return

      const model = session.model
      if (!model?.modelID || !model.providerID) return

      const messages = session.messages ?? []
      let imageCount = 0
      for (const msg of messages) {
        imageCount += messageParts(msg).filter(isImagePart).length
      }
      if (imageCount === 0) return

      const live = catalogModelFromCtx(ctx, model.providerID, model.modelID)
      const catalog = providersFromCatalog(ctx)
      const supports = resolveImageSupport(
        await opencodeConfigSupportsImage(model.providerID, model.modelID),
        catalogEntrySupportsImage(live ?? {}) ?? modelSupportsImage(catalog, model.providerID, model.modelID),
      )
      if (supports) return

      const cfg = await readYwaiConfig()
      const visionModel = resolveVisionModel(
        catalog,
        cfg.vision_model_override || cfg.vision_model || "",
      )

      for (const msg of messages) {
        const parts = messageParts(msg)
        if (!parts.some(isImagePart)) continue
        const userText = collectUserText(parts)
        const next: AnyPart[] = []
        for (const part of parts) {
          if (!isImagePart(part)) {
            next.push(part)
            continue
          }
          if (!visionModel) {
            next.push({
              type: "text",
              text:
                `[Vision bridge] ${formatModel(model)} cannot accept images and no vision model ` +
                `is available. Set one in ywai Settings → Vision bridge.`,
            })
            continue
          }
          try {
            const analysis = await analyzeImage(ctx, bridgeSessions, bridgeText, visionModel, part, userText)
            next.push({
              type: "text",
              text:
                `[Image analysis via ${formatModel(visionModel)}]\n` +
                `The user attached an image. Below is a description produced by the vision model ` +
                `(the chat model should treat this as ground truth about the image):\n\n${analysis}`,
            })
          } catch (err) {
            const detail = err instanceof Error ? err.message : String(err)
            next.push({
              type: "text",
              text:
                `[Vision bridge error] Could not analyze the attached image with ${formatModel(visionModel)}. ` +
                `Details: ${detail}`,
            })
          }
        }
        if (Array.isArray(msg.content)) {
          msg.content.length = 0
          msg.content.push(...next)
        } else if (Array.isArray(msg.parts)) {
          msg.parts.length = 0
          msg.parts.push(...next)
        }
      }
    })
  },
})

async function analyzeImage(
  ctx: any,
  bridgeSessions: Set<string>,
  bridgeText: Map<string, string>,
  visionModel: ModelRef,
  imagePart: AnyPart,
  userText: string,
): Promise<string> {
  const created = await ctx.session.create({
    title: "vision-bridge",
    model: { id: visionModel.modelID, providerID: visionModel.providerID },
  })
  const sessionID = created?.id as string | undefined
  if (!sessionID) throw new Error("could not create the analysis session")

  bridgeSessions.add(sessionID)
  bridgeText.delete(sessionID)
  try {
    await ctx.session.prompt({
      sessionID,
      text: `${VISION_SYSTEM_PROMPT}\n\n${buildVisionPrompt(userText)}`,
      files: [
        {
          uri: imageUri(imagePart),
          ...(typeof imagePart.filename === "string" ? { name: imagePart.filename } : {}),
        },
      ],
    })
    if (typeof ctx.session.wait === "function") {
      try {
        await ctx.session.wait({ sessionID })
      } catch {
        // beta wait may be unavailable; events may already have the text
      }
    }
    const text = bridgeText.get(sessionID)
    if (!text) throw new Error("the vision model returned no text")
    return text
  } finally {
    bridgeSessions.delete(sessionID)
    bridgeText.delete(sessionID)
  }
}

export default Object.freeze(plugin)
