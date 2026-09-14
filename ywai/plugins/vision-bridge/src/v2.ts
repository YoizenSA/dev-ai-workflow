/**
 * vision-bridge — v2 promise-plugin setup.
 *
 * In OpenCode v2, session message transformations happen inside the
 * `ctx.session.hook("context")` callback instead of the v1 "chat.message" hook.
 *
 * When an image reaches a text-only model, this setup analyzes the image
 * through a throwaway child session using OpenCode's vision model, and
 * replaces the file part with the text description in place.
 */

import * as fs from "node:fs/promises"
import * as os from "node:os"
import * as path from "node:path"
import {
	buildVisionPrompt,
	catalogEntrySupportsImage,
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
import {
	createV1ShapedClient,
	type V2PluginContext,
	type V2SessionContextEvent,
} from "../../shared/v2"
import { readYwaiConfig } from "../../shared/ywai-config"

async function opencodeConfigSupportsImage(providerID: string, modelID: string): Promise<boolean | undefined> {
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

async function analyzeImage(
	client: any,
	bridgeSessions: Set<string>,
	parentID: string,
	visionModel: ModelRef,
	imagePart: AnyPart,
	userText: string,
): Promise<string> {
	const created = await client.session.create({
		body: { parentID, title: "vision-bridge" },
	})
	const sessionID = created?.data?.id ?? created?.id
	if (!sessionID) throw new Error("could not create the analysis session")

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
						url: imagePart.url ?? (imagePart as any).uri ?? "",
						...(imagePart.filename ? { filename: imagePart.filename } : {}),
					},
				],
			},
		})

		const parts = (res.data?.parts ?? res.parts ?? []) as Array<{ type?: string; text?: string }>
		const text = extractText(parts)
		if (!text) throw new Error("the vision model returned no text")
		return text
	} finally {
		bridgeSessions.delete(sessionID)
		try {
			await client.session.delete({ path: { id: sessionID } })
		} catch {
			// ignore
		}
	}
}

export async function setupV2(ctx: V2PluginContext): Promise<(() => void) | undefined> {
	if (typeof ctx.session?.hook !== "function") return undefined

	const client = createV1ShapedClient(ctx)
	const bridgeSessions = new Set<string>()

	await ctx.session.hook("context", async (event: V2SessionContextEvent) => {
		try {
			if (bridgeSessions.has(event.sessionID)) return

			const model = event.model as { providerID?: string; id?: string; modelID?: string } | undefined
			const providerID = model?.providerID
			const modelID = model?.id ?? model?.modelID
			if (!providerID || !modelID) return

			const trailing = event.messages?.at(-1)
			if (!trailing || trailing.role !== "user" || !Array.isArray(trailing.content)) return

			const parts = trailing.content as AnyPart[]
			const hasImages = parts.some(
				(p) =>
					(p.type === "file" || p.type === "image") &&
					(typeof p.mime === "string" ? p.mime.startsWith("image/") : Boolean(p.url || (p as any).uri)),
			)
			if (!hasImages) return

			const catalogResult = await client.provider.list()
			const catalog = (catalogResult.data?.all ?? []) as CatalogProvider[]

			const supports = resolveImageSupport(
				await opencodeConfigSupportsImage(providerID, modelID),
				modelSupportsImage(catalog, providerID, modelID),
			)
			if (supports) return

			const cfg = await readYwaiConfig()
			const visionModel = resolveVisionModel(catalog, cfg.vision_model_override || cfg.vision_model || "")
			if (!visionModel) return

			const userText = parts
				.filter((p) => p.type === "text" && typeof p.text === "string")
				.map((p) => p.text as string)
				.join("\n")

			const modelLabel = formatModel(visionModel)
			const next: AnyPart[] = []

			for (const part of parts) {
				const isImage =
					(part.type === "file" || part.type === "image") &&
					(typeof part.mime === "string" ? part.mime.startsWith("image/") : Boolean(part.url || (part as any).uri))
				if (!isImage) {
					next.push(part)
					continue
				}

				try {
					const analysis = await analyzeImage(client, bridgeSessions, event.sessionID, visionModel, part, userText)
					next.push({
						type: "text",
						synthetic: true,
						text:
							`[Image analysis via ${modelLabel}]\n` +
							`The user attached an image. Below is a description produced by the vision model ` +
							`(the chat model should treat this as ground truth about the image):\n\n${analysis}`,
					})
				} catch (err) {
					const msg = err instanceof Error ? err.message : String(err)
					next.push({
						type: "text",
						synthetic: true,
						text: `[Vision bridge error] Could not analyze the attached image with ${modelLabel}. Details: ${msg}`,
					})
				}
			}

			trailing.content = next as typeof trailing.content
		} catch {
			// context hook must never fail the model run
		}
	})

	return undefined
}
