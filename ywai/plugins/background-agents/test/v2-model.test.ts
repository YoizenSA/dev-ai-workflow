import { describe, expect, test } from "bun:test"
import { createV1ShapedClient } from "../src/plugin/v2"

function fakeCtx() {
	const calls: Record<string, any[]> = { prompt: [], switchModel: [], wait: [] }
	const ctx = {
		location: { directory: "/tmp" },
		session: {
			async prompt(input: any) {
				calls.prompt.push(input)
			},
			async switchModel(input: any) {
				calls.switchModel.push(input)
			},
			async wait(input: any) {
				calls.wait.push(input)
			},
		},
		agent: {},
		tool: {},
		event: { subscribe: () => ({ [Symbol.asyncIterator]: async function* () {} }) },
	} as any
	return { ctx, calls }
}

describe("v2 model override", () => {
	// v2's SessionPromptInput has no model field: the override only lands via
	// session.switchModel. Passing it to prompt() was silently dropped, so
	// every delegation ran on the agent's configured model.
	test("routes the model through switchModel, not prompt", async () => {
		const { ctx, calls } = fakeCtx()
		const client = createV1ShapedClient(ctx)

		await client.session.prompt({
			path: { id: "ses_1" },
			body: {
				agent: "dev",
				model: { providerID: "opencode-admin", modelID: "glm-5.3-flash" },
				parts: [{ type: "text", text: "hello" }],
			},
		} as any)

		expect(calls.switchModel).toHaveLength(1)
		expect(calls.switchModel[0]).toEqual({
			sessionID: "ses_1",
			model: { providerID: "opencode-admin", id: "glm-5.3-flash" },
		})
		expect(calls.prompt[0].model).toBeUndefined()
		expect(calls.prompt[0].text).toBe("hello")
	})

	// effort is expressed as the model's variant, so dropping it silently
	// downgrades the run to the model's default reasoning.
	test("carries the variant that effort maps to", async () => {
		const { ctx, calls } = fakeCtx()
		const client = createV1ShapedClient(ctx)

		await client.session.prompt({
			path: { id: "ses_2" },
			body: {
				model: { providerID: "anthropic", modelID: "claude-opus-5", variant: "high" },
				parts: [{ type: "text", text: "hi" }],
			},
		} as any)

		expect(calls.switchModel[0].model.variant).toBe("high")
	})

	test("no model means no switch: the agent keeps its configured one", async () => {
		const { ctx, calls } = fakeCtx()
		const client = createV1ShapedClient(ctx)

		await client.session.prompt({
			path: { id: "ses_3" },
			body: { agent: "dev", parts: [{ type: "text", text: "hi" }] },
		} as any)

		expect(calls.switchModel).toHaveLength(0)
	})
})
