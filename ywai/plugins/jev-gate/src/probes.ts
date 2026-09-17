/**
 * jev-gate Fase 0 probes, kept behind JEV_GATE_PROBE=1
 *
 * Opt-in probes for OpenCode 2, installed by ywai only with the jev-gate
 * flag. It measures what the Jev experiment (ywai/experiments/jev-lab/PLAN.md)
 * needs and changes nothing:
 *
 * - hook "context": per model call, which tools exist and how many characters
 *   their schemas weigh — the ceiling for tool pruning (Pick).
 * - hook permission "evaluate": which actions and effects v2 reports.
 * - one probe tool + one probe command to confirm registration works.
 *
 * Every registration is best effort: a missing or changed v2 API is recorded
 * as a log finding instead of breaking plugin load. Evidence goes to
 * <cwd>/.jev-gate-spike/log.jsonl (one JSON object per line), falling back to
 * the OS temp dir when the project dir is not writable.
 *
 * Same authoring pattern as ywai's other plugins: a plain { id, setup }
 * default export, structural types from plugins/shared/v2, no plugin-API
 * dependency.
 */
import { appendFileSync, mkdirSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import type { V2PluginContext, V2ToolEditor } from "../../shared/v2"

interface ProbeEvent {
	action?: string
	effect?: string
	sessionID?: string
	pattern?: string
}

/** v2 has no shared permission type yet; mirror the shape the spike probes. */
interface PermissionProbeContext extends V2PluginContext {
	permission?: {
		hook?(name: string, cb: (event: ProbeEvent) => Promise<void>): Promise<unknown>
	}
	command?: {
		transform?(cb: (editor: CommandEditor) => void): Promise<unknown>
	}
}

interface CommandEditor {
	add(def: {
		name: string
		description?: string
		execute: (args: any) => Promise<void>
	}): void
}

function logFile(): string {
	const dir = join(process.cwd(), ".jev-gate-spike")
	try {
		mkdirSync(dir, { recursive: true })
		return join(dir, "log.jsonl")
	} catch {
		return join(tmpdir(), "jev-gate-spike-log.jsonl")
	}
}

function log(kind: string, data: unknown) {
	try {
		appendFileSync(
			logFile(),
			JSON.stringify({ ts: new Date().toISOString(), kind, data }) + "\n",
		)
	} catch {
		// Probes must never break the host session.
	}
}

/** Run one probe step; a failure is a FINDINGS.md answer, not a crash. */
async function tryStep(name: string, fn: () => Promise<void> | void) {
	try {
		await fn()
		log("step-ok", { name })
	} catch (err) {
		log("step-failed", { name, error: String(err) })
	}
}

/** Rough ceiling for tool pruning: schema chars are what pruning would save. */
function estimateSchemaChars(tools: unknown): { count: number; chars: number; ids: string[] } {
	const record = (tools ?? {}) as Record<string, unknown>
	const ids = Object.keys(record)
	const chars = ids.reduce((sum, id) => sum + JSON.stringify(record[id] ?? null).length, 0)
	return { count: ids.length, chars, ids }
}

function truncate(value: unknown, max: number): unknown {
	const text = JSON.stringify(value)
	if (text === undefined) return null
	return text.length <= max ? text : text.slice(0, max) + `… (+${text.length - max} chars)`
}

export async function setupProbes(ctx: PermissionProbeContext) {
	log("setup", {
		cwd: process.cwd(),
		location: ctx.location ?? null,
		app: ctx.app ?? null,
	})

	await tryStep("hook-context", async () => {
		await ctx.session.hook?.("context", async (event: any) => {
			const schema = estimateSchemaChars(event?.tools)
			log("context-hook", {
				agent: event?.agent ?? null,
				sessionID: event?.sessionID ?? null,
				toolCount: schema.count,
				schemaChars: schema.chars,
				schemaTokenEstimate: Math.round(schema.chars / 4),
				toolIds: schema.ids,
				lastTwoMessages: truncate(event?.messages?.slice?.(-2), 2000),
			})
		})
	})

	await tryStep("hook-permission", async () => {
		await ctx.permission?.hook?.("evaluate", async (event) => {
			log("permission-hook", {
				action: event?.action ?? null,
				effect: event?.effect ?? null,
				sessionID: event?.sessionID ?? null,
				pattern: event?.pattern ?? null,
			})
		})
	})

	await tryStep("tool-registration", async () => {
		await ctx.tool?.transform?.(async (editor: V2ToolEditor) => {
			editor.add({
				name: "jev_spike_echo",
				description: "Probe tool: echoes its input. Proves tool registration works.",
				input: { type: "object", properties: { text: { type: "string" } } },
				execute: async (input: any) => {
					log("tool-execute", truncate(input, 1000))
					return { content: `echo: ${input?.text ?? "(empty)"}` }
				},
			})
		})
	})

	await tryStep("command-registration", async () => {
		await ctx.command?.transform?.(async (editor: CommandEditor) => {
			editor.add({
				name: "jev-spike",
				description: "Probe command: logs a command invocation.",
				execute: async (args: any) => {
					log("command-execute", truncate(args, 1000))
				},
			})
		})
	})

	log("setup-done", {
		probes: ["hook-context", "hook-permission", "tool-registration", "command-registration"],
	})
}
