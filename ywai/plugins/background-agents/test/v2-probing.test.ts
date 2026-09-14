import { describe, expect, test } from "bun:test"
import { createV1ShapedClient, probeV2Capabilities } from "../../shared/v2"

describe("probeV2Capabilities", () => {
	test("identifies all present methods", () => {
		const fullCtx: any = {
			session: {
				get: async () => {},
				create: async () => {},
				prompt: async () => {},
				synthetic: async () => {},
				switchModel: async () => {},
				switchAgent: async () => {},
				interrupt: async () => {},
				context: async () => [],
				wait: async () => {},
				delete: async () => {},
				hook: async () => {},
			},
			tool: { transform: async () => {} },
			provider: { list: async () => [] },
			agent: { list: async () => [] },
		}

		const caps = probeV2Capabilities(fullCtx)
		expect(caps.sessionGet).toBe(true)
		expect(caps.sessionCreate).toBe(true)
		expect(caps.sessionPrompt).toBe(true)
		expect(caps.sessionSynthetic).toBe(true)
		expect(caps.sessionSwitchModel).toBe(true)
		expect(caps.sessionSwitchAgent).toBe(true)
		expect(caps.sessionInterrupt).toBe(true)
		expect(caps.sessionContext).toBe(true)
		expect(caps.sessionWait).toBe(true)
		expect(caps.sessionDelete).toBe(true)
		expect(caps.sessionHook).toBe(true)
		expect(caps.toolTransform).toBe(true)
		expect(caps.providerList).toBe(true)
		expect(caps.agentList).toBe(true)
	})

	test("degrades cleanly on a minimal context without throwing", async () => {
		const bareCtx: any = { session: {}, event: { subscribe: async function* () {} } }
		const caps = probeV2Capabilities(bareCtx)

		expect(caps.sessionPrompt).toBe(false)
		expect(caps.sessionGet).toBe(false)
		expect(caps.toolTransform).toBe(false)

		const client = createV1ShapedClient(bareCtx)

		// Each method degrades gracefully without unhandled exceptions
		const getRes = await client.session.get({ path: { id: "1" } })
		expect(getRes.data).toBeUndefined()

		const abortRes = await client.session.abort({ path: { id: "1" } })
		expect(abortRes.data).toBeUndefined()

		const msgsRes = await client.session.messages({ path: { id: "1" } })
		expect(msgsRes.data).toEqual([])

		const promptRes = await client.session.prompt({ path: { id: "1" }, body: { noReply: true } })
		expect(promptRes.data).toEqual({ parts: [] })

		const provRes = await client.provider.list()
		expect(provRes.data).toEqual({ all: [] })

		const agentsRes = await client.app.agents()
		expect(agentsRes.data).toEqual([])
	})

	test("promptAsync routes noReply through synthetic, not a visible prompt", async () => {
		// The parent-notification path uses promptAsync, which used to flatten
		// parts to text and always steer-prompt them. That renders the
		// notification as an ordinary user turn, which is how raw
		// <task-notification> XML ended up in the human's transcript. The
		// routing existed in `prompt` and had simply never been mirrored here.
		const synthetics: any[] = []
		const prompts: any[] = []
		const ctx: any = {
			session: {
				synthetic: async (args: any) => {
					synthetics.push(args)
				},
				prompt: async (args: any) => {
					prompts.push(args)
				},
			},
		}

		const client = createV1ShapedClient(ctx)
		await client.session.promptAsync({
			path: { id: "ses_1" },
			body: { noReply: true, parts: [{ type: "text", text: "delegation done" }] },
		})

		expect(synthetics.length).toBe(1)
		expect(synthetics[0]).toMatchObject({ sessionID: "ses_1", text: "delegation done" })
		expect(prompts.length).toBe(0)
	})

	test("promptAsync treats a synthetic-marked part as synthetic too", async () => {
		const synthetics: any[] = []
		const ctx: any = {
			session: {
				synthetic: async (args: any) => {
					synthetics.push(args)
				},
				prompt: async () => {},
			},
		}

		const client = createV1ShapedClient(ctx)
		await client.session.promptAsync({
			path: { id: "ses_1" },
			body: { parts: [{ type: "text", text: "note", synthetic: true }] },
		})

		expect(synthetics.length).toBe(1)
		expect(synthetics[0].text).toBe("note")
	})

	test("promptAsync still steers when the host has no synthetic", async () => {
		const prompts: any[] = []
		const ctx: any = { session: { prompt: async (args: any) => prompts.push(args) } }

		const client = createV1ShapedClient(ctx)
		await client.session.promptAsync({
			path: { id: "ses_1" },
			body: { noReply: true, parts: [{ type: "text", text: "note" }] },
		})

		expect(prompts.length).toBe(1)
		expect(prompts[0].delivery).toBe("steer")
	})

	test("noReply falls back to steer prompt when synthetic is unavailable", async () => {
		const prompts: any[] = []
		const steerOnlyCtx: any = {
			session: {
				prompt: async (args: any) => {
					prompts.push(args)
				},
			},
		}

		const client = createV1ShapedClient(steerOnlyCtx)
		await client.session.prompt({
			path: { id: "ses_1" },
			body: {
				noReply: true,
				parts: [{ type: "text", text: "advisory note" }],
			},
		})

		expect(prompts.length).toBe(1)
		expect(prompts[0].delivery).toBe("steer")
		expect(prompts[0].text).toBe("advisory note")
	})
})
