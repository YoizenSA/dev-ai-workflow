import { useState, useEffect, useMemo } from "react";
import { RefreshCw, Check, Save, Plus, Search, Zap } from "lucide-react";
import { configApi, missionsApi } from "../../api/client";
import type { OrchestratorProfilesResponse, OrchestratorProfile, ModelInfo } from "../../api/types";
import ModelCombobox from "../missions/ModelCombobox";

// Group an agent name by its family for readable sectioning.
function agentGroup(name: string): string {
	if (name.startsWith("qa-")) return "qa-automation";
	if (name.startsWith("migration-")) return "social-refactor";
	return "core";
}

const GROUP_ORDER = ["core", "qa-automation", "social-refactor"];
const GROUP_LABELS: Record<string, string> = {
	core: "Core",
	"qa-automation": "QA Automation",
	"social-refactor": "Social Refactor",
};

export default function ProfilesTab() {
	const [data, setData] = useState<OrchestratorProfilesResponse | null>(null);
	const [models, setModels] = useState<ModelInfo[]>([]);
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [resyncing, setResyncing] = useState(false);
	const [message, setMessage] = useState<string | null>(null);
	// Editable draft of the active profile's per-agent models.
	const [draft, setDraft] = useState<Record<string, string>>({});
	const [dirty, setDirty] = useState(false);
	// Filter agents by name.
	const [agentFilter, setAgentFilter] = useState("");
	// Bulk model selector value.
	const [bulkModel, setBulkModel] = useState("");

	const fetchProfiles = () => {
		setLoading(true);
		configApi
			.getOrchestratorProfiles()
			.then((res) => setData(res))
			.catch((err) => setMessage(`Error loading profiles: ${err.message}`))
			.finally(() => setLoading(false));
	};

	useEffect(() => {
		fetchProfiles();
		missionsApi
			.listModels()
			.then((r) => setModels(Object.values(r.modelsByProvider ?? {}).flat()))
			.catch(() => setModels([]));
	}, []);

	const activeProfile = data?.active ?? "";
	const currentProfile: OrchestratorProfile | null =
		data && data.profiles[activeProfile] ? data.profiles[activeProfile] : null;

	// Reset the draft whenever the active profile (or the loaded data) changes.
	useEffect(() => {
		const agents = currentProfile?.agents ?? {};
		setDraft(
			Object.fromEntries(Object.entries(agents).map(([name, m]) => [name, m.model ?? ""])),
		);
		setDirty(false);
		setAgentFilter("");
		setBulkModel("");
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [activeProfile, data]);

	const handleActiveChange = (name: string) => {
		setSaving(true);
		setMessage(null);
		configApi
			.setActiveOrchestratorProfile(name)
			.then(() => {
				setData((prev) => (prev ? { ...prev, active: name } : prev));
				setMessage("Active profile applied — each agent's model written to its config");
			})
			.catch((err) => setMessage(`Error: ${err.message}`))
			.finally(() => setSaving(false));
	};

	const handleSave = () => {
		if (!activeProfile) return;
		setSaving(true);
		setMessage(null);
		configApi
			.updateOrchestratorProfile(activeProfile, {
				description: currentProfile?.description,
				agents: Object.fromEntries(Object.entries(draft).map(([n, m]) => [n, { model: m }])),
			})
			.then((res) => {
				setData({ profiles: res.profiles, active: res.active });
				setDirty(false);
				setMessage(
					activeProfile === res.active
						? `Profile saved — applied to ${res.agents_applied} agent(s)`
						: "Profile saved",
				);
			})
			.catch((err) => setMessage(`Error: ${err.message}`))
			.finally(() => setSaving(false));
	};

	// Create a new profile, seeded from the current profile's agent models so it
	// has rows to edit. The backend creates the profile when the name is new; it
	// does not activate it — the user selects it afterward to apply and edit.
	const handleAddProfile = () => {
		const input = window.prompt("New profile name (e.g. 'Cheap', 'GPT-only'):");
		if (input === null) return;
		const displayName = input.trim();
		if (!displayName) return;
		const key = displayName.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
		if (!key) {
			setMessage("Error: invalid profile name");
			return;
		}
		if (data?.profiles[key]) {
			setMessage(`Error: profile "${key}" already exists`);
			return;
		}
		setSaving(true);
		setMessage(null);
		const agents = Object.fromEntries(Object.entries(draft).map(([n, m]) => [n, { model: m }]));
		configApi
			.updateOrchestratorProfile(key, {
				display_name: displayName,
				description: `${displayName} profile`,
				agents,
			})
			.then((res) => {
				setData({ profiles: res.profiles, active: res.active });
				setMessage(`Profile "${displayName}" created — select it to activate and edit`);
			})
			.catch((err) => setMessage(`Error: ${err.message}`))
			.finally(() => setSaving(false));
	};

	const handleResync = () => {
		setResyncing(true);
		setMessage(null);
		configApi
			.resyncOrchestratorProfiles()
			.then((res) => {
				setData(res);
				setMessage("Profiles resynced from seed");
			})
			.catch((err) => setMessage(`Error: ${err.message}`))
			.finally(() => setResyncing(false));
	};

	const applyModelToAgents = (modelId: string, names: string[]) => {
		if (!modelId || names.length === 0) return;
		setDraft((prev) => {
			const next = { ...prev };
			for (const name of names) {
				next[name] = modelId;
			}
			return next;
		});
		setDirty(true);
	};

	if (loading && !data) {
		return (
			<div className="card card-pad">
				<div className="loading-inline">
					<div className="spinner" />
					<span>Loading profiles…</span>
				</div>
			</div>
		);
	}

	const profileNames = data ? Object.keys(data.profiles) : [];
	const lowerFilter = agentFilter.toLowerCase();
	const allAgentNames = useMemo(
		() =>
			Object.keys(draft).sort((a, b) => {
				const ga = GROUP_ORDER.indexOf(agentGroup(a));
				const gb = GROUP_ORDER.indexOf(agentGroup(b));
				return ga !== gb ? ga - gb : a.localeCompare(b);
			}),
		[draft],
	);
	const agentNames = useMemo(
		() => allAgentNames.filter((name) => name.toLowerCase().includes(lowerFilter)),
		[allAgentNames, lowerFilter],
	);

	// Models currently in use somewhere in this profile, useful as quick chips.
	const inUseModelIds = useMemo(
		() => Array.from(new Set(Object.values(draft).filter(Boolean))),
		[draft],
	);
	const inUseModels = useMemo(
		() => inUseModelIds.map((id) => models.find((m) => m.id === id)).filter(Boolean) as ModelInfo[],
		[inUseModelIds, models],
	);

	const allAgentGroups = useMemo(
		() =>
			allAgentNames.reduce<Record<string, string[]>>((acc, name) => {
				const g = agentGroup(name);
				(acc[g] ||= []).push(name);
				return acc;
			}, {}),
		[allAgentNames],
	);

	return (
		<div className="card card-pad">
			<div className="card-header" style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
				<h3>Orchestrator Profiles</h3>
				<button type="button" className="btn btn-sm" onClick={handleSave} disabled={saving || !dirty}>
					<Save size={14} />
					{saving ? "Saving…" : "Save"}
				</button>
			</div>

			{message && (
				<div
					className={`alert ${message.startsWith("Error") ? "alert-danger" : "alert-success"}`}
					style={{ marginBottom: "1rem" }}
				>
					{message}
				</div>
			)}

			{/* Profile selector */}
			<div style={{ marginBottom: "1.5rem" }}>
				<div style={{ display: "block", marginBottom: "0.5rem", fontWeight: 600 }}>Active Profile</div>
				<div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
					{profileNames.map((name) => (
						<button
							key={name}
							type="button"
							className={`pill ${activeProfile === name ? "pill-success" : "pill-muted"}`}
							style={{ cursor: "pointer" }}
							onClick={() => handleActiveChange(name)}
							disabled={saving}
						>
							{activeProfile === name && <Check size={12} style={{ marginRight: 4 }} />}
							{data?.profiles[name]?.display_name ?? name}
						</button>
					))}
					<button
						type="button"
						className="pill pill-muted"
						style={{ cursor: "pointer" }}
						onClick={handleAddProfile}
						disabled={saving}
						title="Create a new profile seeded from the current one"
					>
						<Plus size={12} style={{ marginRight: 4 }} />
						Add profile
					</button>
				</div>
			</div>

			{currentProfile && (
				<p className="muted" style={{ margin: "0 0 1rem" }}>
					{currentProfile.description}
				</p>
			)}

			{/* Bulk actions */}
			<div
				style={{
					position: "sticky",
					top: 0,
					zIndex: 10,
					backgroundColor: "var(--surface, inherit)",
					padding: "0.75rem",
					margin: "0 -0.75rem 1rem",
					borderRadius: "8px",
					border: "1px solid var(--color-border, #ddd)",
					display: "flex",
					gap: "0.75rem",
					flexWrap: "wrap",
					alignItems: "center",
				}}
			>
				<div style={{ display: "flex", alignItems: "center", gap: "0.5rem", flex: "1 1 260px" }}>
					<Zap size={14} />
					<span style={{ fontWeight: 600, fontSize: "14px" }}>Quick set</span>
					<div style={{ flex: 1, minWidth: 160 }}>
						<ModelCombobox
							id="bulk-model"
							label=""
							value={bulkModel}
							models={models}
							onChange={setBulkModel}
						/>
					</div>
				</div>
				<div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
					<button
						type="button"
						className="btn btn-sm"
						disabled={!bulkModel || allAgentNames.length === 0}
						onClick={() => applyModelToAgents(bulkModel, allAgentNames)}
					>
						All agents
					</button>
					{GROUP_ORDER.filter((g) => allAgentGroups[g]?.length).map((g) => (
						<button
							key={g}
							type="button"
							className="btn btn-sm btn-ghost"
							disabled={!bulkModel}
							onClick={() => applyModelToAgents(bulkModel, allAgentGroups[g])}
						>
							{GROUP_LABELS[g] ?? g}
						</button>
					))}
				</div>
			</div>

			{/* Agent filter */}
			<div style={{ marginBottom: "1rem" }}>
				<div style={{ position: "relative" }}>
					<Search
						size={14}
						style={{
							position: "absolute",
							left: "10px",
							top: "50%",
							transform: "translateY(-50%)",
							color: "var(--text-muted)",
							pointerEvents: "none",
						}}
					/>
					<input
						type="text"
						className="input"
						placeholder="Filter agents…"
						value={agentFilter}
						onChange={(e) => setAgentFilter(e.target.value)}
						style={{ paddingLeft: "32px" }}
					/>
				</div>
			</div>

			{/* Editable per-agent model table */}
			{agentNames.length > 0 ? (
				<table style={{ width: "100%", borderCollapse: "collapse" }}>
					<thead>
						<tr>
							<th style={{ textAlign: "left", padding: "0.5rem 0.75rem", borderBottom: "1px solid var(--color-border, #ddd)" }}>Agent</th>
							<th style={{ textAlign: "left", padding: "0.5rem 0.75rem", borderBottom: "1px solid var(--color-border, #ddd)", width: "60%" }}>Model</th>
						</tr>
					</thead>
					<tbody>
						{agentNames.flatMap((name, i) => {
							const group = agentGroup(name);
							const prevGroup = i > 0 ? agentGroup(agentNames[i - 1]) : null;
							const rows = [];
							if (group !== prevGroup) {
								rows.push(
									<tr key={`grp-${group}`}>
										<td colSpan={2} style={{ padding: "0.6rem 0.75rem 0.2rem", fontWeight: 600 }} className="muted">
											{GROUP_LABELS[group] ?? group}
										</td>
									</tr>,
								);
							}
							// Show quick chips for models used by other agents.
							const quickModels = inUseModels.filter((m) => m.id !== draft[name]).slice(0, 4);
							rows.push(
								<tr key={name}>
									<td style={{ padding: "0.4rem 0.75rem", borderBottom: "1px solid var(--color-border, #eee)", verticalAlign: "middle" }}>
										{name}
									</td>
									<td style={{ padding: "0.4rem 0.75rem", borderBottom: "1px solid var(--color-border, #eee)" }}>
										<div style={{ display: "flex", alignItems: "center", gap: "0.5rem", flexWrap: "wrap" }}>
											<div style={{ flex: "1 1 220px", minWidth: 220 }}>
												<ModelCombobox
													id={`orch-profile-model-${name}`}
													label=""
													value={draft[name] ?? ""}
													models={models}
													onChange={(v) => {
														setDraft((prev) => ({ ...prev, [name]: v }));
														setDirty(true);
													}}
												/>
											</div>
											{quickModels.length > 0 && (
												<div style={{ display: "flex", gap: "0.35rem", flexWrap: "wrap" }}>
													{quickModels.map((m) => (
														<button
															key={m.id}
															type="button"
															className="pill pill-muted"
															title={m.name || m.id}
															onClick={() => {
																setDraft((prev) => ({ ...prev, [name]: m.id }));
																setDirty(true);
															}}
															style={{ fontSize: "11px", padding: "0.15rem 0.45rem" }}
														>
															{m.name || m.id}
														</button>
													))}
												</div>
											)}
										</div>
									</td>
								</tr>,
							);
							return rows;
						})}
					</tbody>
				</table>
			) : (
				<p className="muted">
					{agentFilter ? "No agents match your filter" : "No agent model mappings for this profile"}
				</p>
			)}

			<div style={{ marginTop: "1.5rem" }}>
				<button type="button" className="btn btn-sm btn-ghost" onClick={handleResync} disabled={resyncing}>
					<RefreshCw size={14} className={resyncing ? "spin" : ""} />
					{resyncing ? "Resyncing…" : "Resync from Seed"}
				</button>
			</div>
		</div>
	);
}
