import { describe, expect, mock, test } from "bun:test"

mock.module("node:fs/promises", () => ({
  readFile: async () => {
    throw new Error("no config in tests")
  },
}))

import plugin from "../src/index"

function fakeCtx(opts: {
  catalogModel?: unknown
  promptText?: string
}) {
  let contextHook: ((session: Record<string, unknown>) => Promise<void>) | undefined
  let eventHandler: ((event: { type: string; data?: Record<string, unknown> }) => void) | undefined
  const prompts: unknown[] = []

  const ctx = {
    catalog: {
      model: {
        get: () => opts.catalogModel,
      },
      provider: {
        list: () => [
          {
            provider: { id: "opencode-admin" },
            models: new Map([
              [
                "kimi-k3",
                {
                  capabilities: { tools: true, input: ["text", "image"], output: ["text"] },
                },
              ],
            ]),
          },
        ],
      },
    },
    event: {
      subscribe: async (fn: typeof eventHandler) => {
        eventHandler = fn
      },
    },
    session: {
      hook: async (name: string, fn: typeof contextHook) => {
        if (name === "context") contextHook = fn
      },
      create: async () => ({ id: "bridge-ses" }),
      prompt: async (input: unknown) => {
        prompts.push(input)
        eventHandler?.({
          type: "message.updated",
          data: {
            sessionID: "bridge-ses",
            parts: [{ type: "text", text: "a login form" }],
          },
        })
        eventHandler?.({
          type: "session.idle",
          data: { sessionID: "bridge-ses" },
        })
      },
      wait: async () => {},
    },
    _prompts: prompts,
    runContext: async (session: Record<string, unknown>) => {
      if (!contextHook) throw new Error("context hook not registered")
      await contextHook(session)
    },
  }
  return ctx
}

describe("v2 hook mapping", () => {
  test("setup registers session.hook(context) and event.subscribe", async () => {
    const ctx = fakeCtx({
      catalogModel: {
        capabilities: { tools: true, input: ["text"], output: ["text"] },
      },
    })
    await plugin.setup(ctx)
    const session = {
      sessionID: "user-ses",
      model: { providerID: "opencode-admin", modelID: "deepseek-v4-flash" },
      system: [],
      messages: [
        {
          role: "user",
          content: [
            { type: "text", text: "what is this?" },
            { type: "media", mediaType: "image/png", data: "file:///shot.png" },
          ],
        },
      ],
    }
    await ctx.runContext(session)
    const media = (session.messages[0].content as Array<{ type: string; text?: string }>).find(
      (p) => p.type === "text" && p.text?.includes("Image analysis"),
    )
    expect(media?.text).toContain("Image analysis")
    expect(ctx._prompts).toHaveLength(1)
  })

  test("passes through when catalog says the model accepts images", async () => {
    const ctx = fakeCtx({
      catalogModel: {
        capabilities: { tools: true, input: ["text", "image"], output: ["text"] },
      },
    })
    await plugin.setup(ctx)
    const image = { type: "media", mediaType: "image/png", data: "file:///shot.png" }
    const session = {
      sessionID: "user-ses",
      model: { providerID: "opencode-admin", modelID: "kimi-k3" },
      system: [],
      messages: [{ role: "user", content: [image] }],
    }
    await ctx.runContext(session)
    expect(session.messages[0].content).toEqual([image])
    expect(ctx._prompts).toHaveLength(0)
  })
})
