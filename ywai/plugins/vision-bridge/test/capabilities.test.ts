import { describe, expect, test } from "bun:test"
import {
  buildVisionPrompt,
  collectUserText,
  catalogEntrySupportsImage,
  extractText,
  modelSupportsImage,
  resolveVisionModel,
} from "../src/index"

// OpenCode's live catalog (Provider.Model) nests capabilities:
//   capabilities.attachment + capabilities.input.image
// An earlier gate read a flat `attachment` / `modalities` that does not exist
// on that shape, so every model read as text-only and the bridge intercepted
// images for vision models too. These cases pin both shapes.

const visionCaps = {
  capabilities: {
    attachment: true,
    input: { text: true, image: true, audio: false, video: false, pdf: true },
  },
} as never

const textCaps = {
  capabilities: {
    attachment: false,
    input: { text: true, image: false, audio: false, video: false, pdf: false },
  },
} as never

describe("catalogEntrySupportsImage — current OpenCode shape", () => {
  test("vision model via capabilities.input.image", () => {
    expect(catalogEntrySupportsImage(visionCaps)).toBe(true)
  })

  test("text-only model is not treated as vision", () => {
    expect(catalogEntrySupportsImage(textCaps)).toBe(false)
  })

  test("attachment alone counts as vision", () => {
    expect(catalogEntrySupportsImage({ capabilities: { attachment: true } } as never)).toBe(true)
  })
})

describe("catalogEntrySupportsImage — legacy flat shape", () => {
  test("modalities.input containing image", () => {
    expect(
      catalogEntrySupportsImage({ attachment: true, modalities: { input: ["text", "image"] } }),
    ).toBe(true)
  })

  test("modalities.input without image", () => {
    expect(catalogEntrySupportsImage({ attachment: false, modalities: { input: ["text"] } })).toBe(
      false,
    )
  })
})

describe("catalogEntrySupportsImage — undeterminable", () => {
  // Must return undefined (not false) so the caller falls through to
  // opencode.json instead of bridging every model.
  test("entry with no capability info", () => {
    expect(catalogEntrySupportsImage({} as never)).toBeUndefined()
  })
})

const catalog = [
  {
    id: "opencode-admin",
    models: {
      "deepseek-v4-flash": textCaps,
      "kimi-k3": visionCaps,
      "mimo-v2.5": visionCaps,
    },
  },
]

describe("modelSupportsImage", () => {
  test("resolves a known vision model", () => {
    expect(modelSupportsImage(catalog, "opencode-admin", "kimi-k3")).toBe(true)
  })

  test("resolves a known text-only model", () => {
    expect(modelSupportsImage(catalog, "opencode-admin", "deepseek-v4-flash")).toBe(false)
  })

  test("unknown model falls through", () => {
    expect(modelSupportsImage(catalog, "opencode-admin", "nope")).toBeUndefined()
    expect(modelSupportsImage(catalog, "other", "kimi-k3")).toBeUndefined()
  })
})

describe("resolveVisionModel", () => {
  test("qualified provider/model preference is used verbatim", () => {
    expect(resolveVisionModel(catalog, "opencode-admin/mimo-v2.5")).toEqual({
      providerID: "opencode-admin",
      modelID: "mimo-v2.5",
    })
  })

  test("bare model id resolves its provider from the catalog", () => {
    expect(resolveVisionModel(catalog, "mimo-v2.5")).toEqual({
      providerID: "opencode-admin",
      modelID: "mimo-v2.5",
    })
  })

  test("bare model id absent from the catalog is unresolved", () => {
    expect(resolveVisionModel(catalog, "not-here")).toBeUndefined()
  })

  test("qualified preference works even outside the catalog", () => {
    expect(resolveVisionModel(catalog, "openai/gpt-5.6-terra")).toEqual({
      providerID: "openai",
      modelID: "gpt-5.6-terra",
    })
  })

  test("no preference picks the first vision-capable model", () => {
    const picked = resolveVisionModel(catalog, "")
    expect(picked).toEqual({ providerID: "opencode-admin", modelID: "kimi-k3" })
  })

  test("no preference and no vision model available", () => {
    expect(
      resolveVisionModel([{ id: "p", models: { "text-only": textCaps } }], ""),
    ).toBeUndefined()
  })
})

describe("extractText", () => {
  test("joins text parts and ignores the rest", () => {
    expect(
      extractText([
        { type: "text", text: " A screenshot " },
        { type: "tool", text: "ignored" } as never,
        { type: "text", text: "of a login form" },
      ]),
    ).toBe("A screenshot\nof a login form")
  })

  test("empty when the model returned no text", () => {
    expect(extractText([{ type: "reasoning" } as never])).toBe("")
  })
})

describe("collectUserText — text sent with the image", () => {
  test("forwards what the user typed", () => {
    expect(
      collectUserText([
        { type: "text", text: "  why is this button misaligned?  " },
        { type: "file", mime: "image/png", url: "file:///shot.png" },
      ]),
    ).toBe("why is this button misaligned?")
  })

  test("joins multiple text parts", () => {
    expect(
      collectUserText([
        { type: "text", text: "first" },
        { type: "file", mime: "image/png", url: "x" },
        { type: "text", text: "second" },
      ]),
    ).toBe("first\nsecond")
  })

  test("skips synthetic parts injected by OpenCode", () => {
    expect(
      collectUserText([
        { type: "text", text: "real question" },
        { type: "text", text: "Use the above message to call the task tool", synthetic: true },
      ]),
    ).toBe("real question")
  })

  test("empty when only an image was sent", () => {
    expect(collectUserText([{ type: "file", mime: "image/png", url: "x" }])).toBe("")
  })
})

describe("buildVisionPrompt", () => {
  test("includes the user's message verbatim", () => {
    const out = buildVisionPrompt("why is this button misaligned?")
    expect(out).toContain("why is this button misaligned?")
    expect(out).toContain("answer that message")
  })

  test("falls back to a plain description when there is no text", () => {
    const out = buildVisionPrompt("   ")
    expect(out).toContain("Describe this image")
    expect(out).not.toContain("the following message")
  })
})
