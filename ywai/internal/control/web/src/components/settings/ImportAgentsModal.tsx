import { useMemo, useState } from "react";
import { CheckCircle2, AlertTriangle } from "lucide-react";
import Modal from "../shared/Modal";
import { configApi } from "../../api/client";

/**
 * Import a full opencode.json `agent` block (like the gentle-orchestrator
 * config) and persist each agent through the existing config endpoints — no
 * new backend endpoint required.
 *
 * For each agent entry we build the markdown frontmatter (description, mode,
 * model, permission block incl. the per-subagent task map) + prompt body, then
 * create or update the agent file and apply task-permissions / model via their
 * dedicated endpoints so the graph reflects them on next load.
 */

interface AgentEntry {
	description?: string;
	mode?: string;
	model?: string;
	hidden?: boolean;
	temperature?: number;
	prompt?: string;
	permission?: Record<string, unknown>;
}

interface ParsedAgent {
	name: string;
	mode: string;
	hasModel: boolean;
	taskTargets: string[];
}

const SAMPLE = `{
  "$schema": "https://opencode.ai/config.json",
  "agent": {
    "gentle-orchestrator": {
      "mode": "primary",
      "permission": { "task": { "*": "deny", "dev": "allow" } }
    }
  }
}`;

/** Quote a YAML key if it contains chars that need quoting. */
function yamlKey(k: string): string {
	if (k === "*" || /[:#&!|>',[\]{}%`@*]/.test(k)) return JSON.stringify(k);
	return k;
}

/** Build the agent markdown (frontmatter + prompt body) for createAgent/updateAgent. */
function buildMarkdown(entry: AgentEntry): string {
	const lines: string[] = ["---"];
	if (entry.description) lines.push(`description: ${entry.description}`);
	lines.push(`mode: ${entry.mode || "primary"}`);
	if (entry.hidden) lines.push("hidden: true");
	if (typeof entry.temperature === "number")
		lines.push(`temperature: ${entry.temperature}`);

	const perm = entry.permission ?? {};
	const permKeys = Object.keys(perm);
	if (permKeys.length > 0) {
		lines.push("permission:");
		for (const k of permKeys.sort()) {
			const v = perm[k];
			if (v && typeof v === "object") {
				// Per-subagent map (task). Nest under the key.
				lines.push(`  ${yamlKey(k)}:`);
				for (const [sub, val] of Object.entries(v as Record<string, unknown>)) {
					lines.push(`    ${yamlKey(sub)}: ${String(val)}`);
				}
			} else {
				lines.push(`  ${yamlKey(k)}: ${String(v)}`);
			}
		}
	}
	lines.push("---", "");
	lines.push(entry.prompt || "");
	return lines.join("\n");
}

/** Extract the scalar task map (if any) to apply via the task-permissions endpoint. */
function extractTaskMap(
	perm: Record<string, unknown> | undefined,
): Record<string, string> | null {
	if (!perm?.task || typeof perm.task !== "object") return null;
	const out: Record<string, string> = {};
	for (const [k, v] of Object.entries(perm.task as Record<string, unknown>)) {
		out[k] = String(v);
	}
	return out;
}

export default function ImportAgentsModal({
	open,
	onClose,
	onDone,
}: {
	open: boolean;
	onClose: () => void;
	onDone: () => void;
}) {
	const [text, setText] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [importing, setImporting] = useState(false);
	const [result, setResult] = useState<{ ok: string[]; failed: string[] } | null>(null);

	const parsed = useMemo<{ agents: Record<string, AgentEntry> } | null>(() => {
		if (!text.trim()) return null;
		try {
			const obj = JSON.parse(text) as Record<string, unknown>;
			const agentBlock =
				(obj.agent as Record<string, AgentEntry> | undefined) ?? undefined;
			if (!agentBlock || typeof agentBlock !== "object") {
				// Tolerate a bare agent object pasted without the wrapper.
				const allStringKeys = Object.values(obj).every(
					(v) => typeof v === "object" && v !== null,
				);
				if (allStringKeys) return { agents: obj as Record<string, AgentEntry> };
				return null;
			}
			return { agents: agentBlock };
		} catch {
			return null;
		}
	}, [text]);

	const preview: ParsedAgent[] = useMemo(() => {
		if (!parsed) return [];
		return Object.entries(parsed.agents).map(([name, entry]) => {
			const taskMap = extractTaskMap(entry.permission as Record<string, unknown>);
			// "*" is a catch-all, not a concrete delegation edge — exclude it
			// from the preview count so it reflects real delegations.
			const taskTargets = taskMap ? Object.keys(taskMap).filter((t) => t !== "*") : [];
			return {
				name,
				mode: entry.mode || "primary",
				hasModel: Boolean(entry.model),
				taskTargets,
			};
		});
	}, [parsed]);

	const handleImport = async () => {
		if (!parsed) return;
		setImporting(true);
		setError(null);
		const ok: string[] = [];
		const failed: string[] = [];
		try {
			for (const [name, entry] of Object.entries(parsed.agents)) {
				try {
					const markdown = buildMarkdown(entry);
					// Try create; fall back to update if it already exists.
					try {
						await configApi.createAgent(name, markdown);
					} catch {
						await configApi.updateAgent(name, markdown);
					}
					const taskMap = extractTaskMap(entry.permission as Record<string, unknown>);
					if (taskMap) {
						await configApi.updateAgentTaskPermissions(name, taskMap);
					}
					if (entry.model) {
						await configApi.updateAgentModel(name, entry.model);
					}
					ok.push(name);
				} catch {
					failed.push(name);
				}
			}
			setResult({ ok, failed });
			if (ok.length > 0) onDone();
		} catch (err) {
			setError(err instanceof Error ? err.message : String(err));
		} finally {
			setImporting(false);
		}
	};

	const handleClose = () => {
		setText("");
		setError(null);
		setResult(null);
		onClose();
	};

	return (
		<Modal
			open={open}
			onClose={handleClose}
			title="Import opencode.json agents"
			subtitle="Paste a config with an `agent` block. Each entry becomes an agent file + delegations."
			width="640px"
			footer={
				result ? (
					<button className="btn btn-primary" onClick={handleClose}>
						Done
					</button>
				) : (
					<>
						<button className="btn btn-ghost" onClick={handleClose}>
							Cancel
						</button>
						<button
							className="btn btn-primary"
							onClick={handleImport}
							disabled={!parsed || importing}
						>
							{importing ? "Importing…" : `Import ${preview.length} agent(s)`}
						</button>
					</>
				)
			}
		>
			{result ? (
				<div className="orch-import-result">
					{result.ok.length > 0 && (
						<div className="alert alert-success">
							<CheckCircle2 size={16} /> Imported {result.ok.length}:{" "}
							{result.ok.join(", ")}
						</div>
					)}
					{result.failed.length > 0 && (
						<div className="alert alert-danger">
							<AlertTriangle size={16} /> Failed {result.failed.length}:{" "}
							{result.failed.join(", ")}
						</div>
					)}
				</div>
			) : (
				<>
					<div className="field">
						<label className="field-label" htmlFor="import-json">
							opencode.json content
						</label>
						<textarea
							id="import-json"
							className="input mono"
							rows={12}
							placeholder={SAMPLE}
							value={text}
							onChange={(e) => setText(e.target.value)}
						/>
					</div>
					{text.trim() && !parsed && (
						<div className="alert alert-danger">Invalid JSON. Check the syntax.</div>
					)}
					{preview.length > 0 && (
						<div className="orch-import-preview">
							<p className="field-label">Detected {preview.length} agent(s):</p>
							<ul className="orch-preview-list">
								{preview.map((p) => (
									<li key={p.name}>
										<strong>{p.name}</strong>{" "}
										<span className="pill pill-sm pill-muted">{p.mode}</span>
										{p.hasModel && <span className="pill pill-sm pill-info">model</span>}
										{p.taskTargets.length > 0 && (
											<span className="pill pill-sm pill-success">
												{p.taskTargets.length} delegation(s)
											</span>
										)}
									</li>
								))}
							</ul>
						</div>
					)}
					{error && <div className="alert alert-danger">{error}</div>}
				</>
			)}
		</Modal>
	);
}
