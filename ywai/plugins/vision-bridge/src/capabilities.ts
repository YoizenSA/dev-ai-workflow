/**
 * vision-bridge — pure capability and text helpers.
 *
 * These live outside src/index.ts on purpose. OpenCode's plugin loader treats
 * EVERY function exported by a plugin entry file as a plugin factory and calls
 * it with the plugin input. Exporting helpers from the entry made the loader
 * invoke resolveVisionModel(input), which threw on `preference.trim()` and
 * aborted registration — the plugin silently never loaded, so images reached
 * text-only models untouched. The entry must default-export the plugin and
 * nothing else; unit tests import from here.
 */

export type AnyPart = {
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

/** A model reference as OpenCode addresses it. */
export type ModelRef = { providerID: string; modelID: string }

/** A model entry as returned by client.provider.list(), across OpenCode versions. */
export type CatalogModel = {
  // Current shape (Provider.Model): capabilities.attachment + capabilities.input.image
  capabilities?: {
    attachment?: boolean
    tools?: boolean
    input?: { image?: boolean; audio?: boolean; video?: boolean; pdf?: boolean } | string[]
    output?: string[]
  }
  // Legacy/config shape: flat attachment + modalities.input as a string array
  attachment?: boolean
  modalities?: { input?: string[] }
}

export type CatalogProvider = { id: string; models?: Record<string, CatalogModel> }

/**
 * Read image support off a catalog entry, tolerating both OpenCode shapes.
 * Returns undefined when the entry carries no capability info at all, so the
 * caller falls through to the next source instead of assuming "no vision".
 */
export function isImagePart(p: AnyPart): boolean {
  const mime = String(p.mime ?? p.mediaType ?? "")
  if (!mime.startsWith("image/")) return false
  return p.type === "file" || p.type === "media"
}

export function catalogEntrySupportsImage(model: CatalogModel): boolean | undefined {
  const caps = model.capabilities
  if (caps) {
    if (Array.isArray(caps.input)) {
      return caps.input.includes("image")
    }
    if (caps.input?.image === true) return true
    if (caps.attachment === true) return true
    // capabilities present but no image input → genuinely text-only
    if (caps.input || caps.attachment !== undefined) return false
  }
  if (model.modalities?.input) {
    return model.modalities.input.includes("image")
  }
  if (typeof model.attachment === "boolean") return model.attachment
  return undefined
}

/**
 * Whether the chat model can accept images, according to OpenCode's live catalog.
 *
 * NOTE: the live catalog is NOT authoritative for custom providers. OpenCode
 * merges models.dev data by model id, so a proxied model inherits the upstream
 * vendor's capabilities and an explicit `attachment: false` in opencode.json is
 * silently overridden. Observed with tokenbank's `opencode-admin/gpt-5.6-luna`:
 * opencode.json declares `attachment: false` / input `["text","pdf"]` while the
 * live catalog reports `capabilities.input.image: true`. See resolveImageSupport.
 */
export function modelSupportsImage(
  catalog: CatalogProvider[],
  providerID: string,
  modelID: string,
): boolean | undefined {
  const prov = catalog.find((p) => p.id === providerID)
  const model = prov?.models?.[modelID]
  if (!model) return undefined
  return catalogEntrySupportsImage(model)
}

/**
 * Decides image support from both sources.
 *
 * opencode.json wins: it is the explicit, locally-declared capability (written
 * by tokenbank/ywai for the proxied provider), whereas the live catalog carries
 * models.dev defaults that ignore it. The catalog only fills in models the
 * config says nothing about. Unknown to both → assume vision, so images pass
 * through untouched rather than being needlessly bridged.
 */
export function resolveImageSupport(
  configSupport: boolean | undefined,
  catalogSupport: boolean | undefined,
): boolean {
  return configSupport ?? catalogSupport ?? true
}

/**
 * Picks the vision model to analyze with.
 *
 * The configured preference wins (accepting "provider/model" or a bare model id
 * resolved against the catalog). With no preference, the first vision-capable
 * model in OpenCode's catalog is used.
 */
export function resolveVisionModel(
  catalog: CatalogProvider[],
  preference: string,
): ModelRef | undefined {
  const pref = preference.trim()
  if (pref) {
    const slash = pref.indexOf("/")
    if (slash > 0) {
      const providerID = pref.slice(0, slash)
      const modelID = pref.slice(slash + 1)
      // Trust an explicit provider/model even when absent from the catalog:
      // the user may have configured it outside the listed providers.
      return { providerID, modelID }
    }
    // Bare model id — find the provider that offers it.
    for (const prov of catalog) {
      if (prov.models?.[pref]) return { providerID: prov.id, modelID: pref }
    }
    return undefined
  }

  for (const prov of catalog) {
    for (const [modelID, model] of Object.entries(prov.models ?? {})) {
      if (catalogEntrySupportsImage(model) === true) {
        return { providerID: prov.id, modelID }
      }
    }
  }
  return undefined
}

/** Formats a model ref the way OpenCode displays it. */
export function formatModel(m: ModelRef): string {
  return `${m.providerID}/${m.modelID}`
}

/**
 * Collects the text the user actually typed with the image.
 *
 * Synthetic parts are skipped: OpenCode injects its own text (agent hints and
 * similar) into the same message, and forwarding those as "the user's message"
 * would misdirect the vision model.
 */
export function messageParts(message: { parts?: AnyPart[]; content?: AnyPart[] }): AnyPart[] {
  if (Array.isArray(message.content)) return message.content
  if (Array.isArray(message.parts)) return message.parts
  return []
}

export function collectUserText(parts: AnyPart[]): string {
  return parts
    .filter((p) => p.type === "text" && !p.synthetic && typeof p.text === "string")
    .map((p) => p.text!.trim())
    .filter(Boolean)
    .join("\n")
}

/**
 * Builds the user turn sent alongside the image.
 *
 * The text the user typed with the image is forwarded verbatim — it is usually
 * what makes the image interpretable ("why does this button look wrong?"), and
 * without it the vision model describes the picture blind to the actual
 * question.
 */
export function buildVisionPrompt(userText: string): string {
  const text = userText.trim()
  if (!text) {
    return "Describe this image, then end with a short one-paragraph summary."
  }
  return [
    "The user attached this image along with the following message:",
    "",
    text,
    "",
    "Describe the image, then answer that message using what you can see.",
  ].join("\n")
}

/** Pulls the assistant's text out of a session.prompt response. */
export function extractText(parts: Array<{ type?: string; text?: string }>): string {
  return parts
    .filter((p) => p?.type === "text" && typeof p.text === "string")
    .map((p) => p.text!.trim())
    .filter(Boolean)
    .join("\n")
    .trim()
}
