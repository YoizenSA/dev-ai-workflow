/**
 * MCP server front for the v2 delegation engine (stdio JSON-RPC).
 *
 * The current v2 runtimes accept `mcp` config entries but do not yet spawn
 * local servers — this front is wired and ready for the moment upstream
 * turns that on. Tools keep the v1 names (delegate, delegation_steer, …) so
 * agent prompts that reference them keep working; permission actions arrive
 * prefixed as `<server>_<tool>` per the v2 permission model.
 *
 * Minimal protocol surface: initialize, notifications/initialized, ping,
 * tools/list, tools/call.
 */

import * as readline from "node:readline"
import { effortToVariant, formatModelRef, parseModelString } from "./adapter"
import { followEvents, httpSessionIO, serverBaseURL } from "./http-api"
import { DelegationManager, defaultRecordsDir } from "./manager"

interface JsonRpcRequest {
	jsonrpc: string
	id?: number | string
	method: string
	params?: any
}

const TOOLS = [
	{
		name: "delegate",
		description:
			"Delegate a task to an agent, sync or async. mode=sync blocks and returns the result; mode=async returns an ID and notifies on completion. Selectors: agent, model (provider/model-id), effort (high|medium|low), timeout_minutes (0 = none).",
		inputSchema: {
			type: "object",
			properties: {
				prompt: { type: "string", description: "Full prompt for the agent." },
				agent: { type: "string", description: "Agent to delegate to." },
				mode: { type: "string", enum: ["sync", "async"], description: "sync waits inline; async (default) runs in background." },
				model: { type: "string", description: 'Model override "provider/model-id".' },
				effort: { type: "string", enum: ["high", "medium", "low"], description: "Reasoning effort (model variant)." },
				timeout_minutes: { type: "number", description: "Max runtime in minutes (0 = no timeout)." },
			},
			required: ["prompt", "agent"],
		},
	},
	{
		name: "delegation_read",
		description: "Read the full persisted output of a delegation by ID.",
		inputSchema: { type: "object", properties: { id: { type: "string" } }, required: ["id"] },
	},
	{
		name: "delegation_list",
		description: "List all delegations (running and completed).",
		inputSchema: { type: "object", properties: {} },
	},
	{
		name: "delegation_peek",
		description: "Live transcript digest of a RUNNING delegation. Non-blocking.",
		inputSchema: { type: "object", properties: { id: { type: "string" } }, required: ["id"] },
	},
	{
		name: "delegation_steer",
		description: "Inject an instruction into a RUNNING delegation (native v2 steering).",
		inputSchema: {
			type: "object",
			properties: { id: { type: "string" }, message: { type: "string" } },
			required: ["id", "message"],
		},
	},
	{
		name: "delegation_stop",
		description: "Stop a RUNNING delegation; partial output stays readable.",
		inputSchema: { type: "object", properties: { id: { type: "string" } }, required: ["id"] },
	},
	{
		name: "delegation_status",
		description: "Live status of running delegations (elapsed, deadline, activity).",
		inputSchema: { type: "object", properties: {} },
	},
]

function textResult(text: string) {
	return { content: [{ type: "text", text }] }
}

async function main(): Promise<void> {
	const base = serverBaseURL()
	const recordsDir = defaultRecordsDir(process.env.BACKGROUND_AGENTS_PROJECT_KEY ?? "default")
	const manager = new DelegationManager(httpSessionIO(base), recordsDir, (m) =>
		process.stderr.write(`[background-agents-v2] ${m}\n`),
	)
	await manager.start()
	const events = followEvents((e) => manager.handleEvent(e), { base })

	const iface = readline.createInterface({ input: process.stdin })
	iface.on("line", (line) => {
		let req: JsonRpcRequest
		try {
			req = JSON.parse(line)
		} catch {
			return
		}
		const reply = (result: unknown) =>
			process.stdout.write(JSON.stringify({ jsonrpc: "2.0", id: req.id, result }) + "\n")
		const replyError = (code: number, message: string) =>
			process.stdout.write(JSON.stringify({ jsonrpc: "2.0", id: req.id, error: { code, message } }) + "\n")

		switch (req.method) {
			case "initialize":
				reply({
					protocolVersion: "2025-06-18",
					capabilities: { tools: {} },
					serverInfo: { name: "background-agents", version: "0.1.0" },
				})
				break
			case "notifications/initialized":
			case "notifications/cancelled":
				break
			case "ping":
				reply({})
				break
			case "tools/list":
				reply({ tools: TOOLS })
				break
			case "tools/call": {
				const tool = String(req.params?.name ?? "")
				const args = (req.params?.arguments ?? {}) as Record<string, any>
				void handleCall(manager, tool, args, base)
					.then((out) => reply(textResult(out)))
					.catch((error) => replyError(-32000, String(error)))
				break
			}
			default:
				if (req.id !== undefined) replyError(-32601, `unknown method ${req.method}`)
		}
	})
	iface.on("close", () => {
		events.stop()
		void manager.stop()
	})
}

async function handleCall(
	manager: DelegationManager,
	tool: string,
	args: Record<string, any>,
	base: string,
): Promise<string> {
	// The parent session is the MCP client's active session; fall back to the
	// env-provided one, and finally "default" (records still readable).
	const parent = process.env.BACKGROUND_AGENTS_PARENT ?? "ses_unknown"

	switch (tool) {
		case "delegate": {
			const mode = args.mode === "sync" ? "sync" : "async"
			let model
			if (typeof args.model === "string") {
				model = parseModelString(args.model)
				if (!model) return `❌ Invalid model "${args.model}". Expected "provider/model-id".`
			}
			if (typeof args.effort === "string") {
				if (!model) return `❌ effort requires a model selector.`
				model = { ...model, variant: effortToVariant(args.effort) }
			}
			const outcome = await manager.delegate({
				parentSessionID: parent,
				prompt: String(args.prompt ?? ""),
				agent: String(args.agent ?? ""),
				mode,
				model,
				maxRunTimeMs:
					args.timeout_minutes !== undefined ? Number(args.timeout_minutes) * 60_000 : undefined,
			})
			if (outcome.mode === "sync") {
				const label = outcome.status === "completed" ? "COMPLETED" : outcome.status.toUpperCase()
				return `${outcome.id} (${args.agent}) ${label}${outcome.error ? `\nError: ${outcome.error}` : ""}\n\n${outcome.result || "_(no output)_"}`
			}
			const modelPart = model ? `\nModel: ${formatModelRef(model)}` : ""
			const timeoutLabel =
				outcome.timeoutMs > 0
					? `${Math.round(outcome.timeoutMs / 60_000)}min (a steer resets the window)`
					: "none"
			return `Delegation started: ${outcome.id}\nAgent: ${args.agent}${modelPart}\nMode: async\nTimeout: ${timeoutLabel}\nYou WILL be notified when complete. Do NOT poll.`
		}
		case "delegation_read":
			return manager.read(parent, String(args.id ?? ""))
		case "delegation_list": {
			const items = await manager.list(parent)
			if (items.length === 0) return "No delegations found for this session."
			return items
				.map((d) => `- **${d.id}** [${d.status}]${d.agent ? ` (${d.agent})` : ""}${d.unread ? " [unread]" : ""}`)
				.join("\n")
		}
		case "delegation_peek":
			return manager.peek(parent, String(args.id ?? ""))
		case "delegation_steer":
			return manager.steer(parent, String(args.id ?? ""), String(args.message ?? ""))
		case "delegation_stop":
			return manager.stopDelegation(parent, String(args.id ?? ""))
		case "delegation_status":
			return manager.statusReport(parent)
		default:
			return `❌ Unknown tool ${tool}`
	}
}

void main().catch((error) => {
	process.stderr.write(`[background-agents-v2] fatal: ${String(error)}\n`)
	process.exit(1)
})
