import { useState, useEffect, useMemo } from "react";
import { RefreshCw, Save, Plus, Search, Zap, AlertTriangle, Boxes } from "lucide-react";
import { configApi, missionsApi } from "../../api/client";
import type { OrchestratorProfilesResponse, OrchestratorProfile, ModelInfo } from "../../api/types";
import ModelCombobox from "../missions/ModelCombobox";

// Preferred display order for the real agents/ folders. Unknown folders append
// alphabetically so a new group still shows up without a code change.
const GROUP_ORDER = ["core", "planning", "qa-automation", "qa-exploratory", "social-refactor"];

// Human label for a folder slug; falls back to a Title-Case version of the slug.
function groupLabel(slug: string): string {
	switch (slug) {
		case "core":
			return "Core";
		case "planning":
			return "Planning";
		case "qa-automation":
			return "QA Automation";
		case "qa-exploratory":
			return "QA Exploratory";
		case "social-refactor":
			return "Social Refactor";
		default:
			return slug
				.split("-")
				.map((w) => w.charAt(0).toUpperCase() + w.slice(1))
				.join(" ");
	}
}

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
	// Editable draft of the active profile's omp modelRoles (like the agents).
	const [ompDraft, setOmpDraft] = useState<Record<string, string>>({});
	const [ompDirty, setOmpDirty] = useState(false);
	// Filter agents by name.
	const [agentFilter, setAgentFilter] = useState("");
	// Bulk model selector value.
	const [bulkModel, setBulkModel] = useState("");
	// Active group sub-tab.
	const [activeGroup, setActiveGroup] = useState<string>("core");

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
	// Editing one of these is allowed but does not last: install rewrites them so
	// they pick up newly added agents. Saying so beforehand costs a line; finding
	// out afterwards costs the edit.
	const isShipped = (data?.shipped ?? []).includes(activeProfile);
	const currentProfile: OrchestratorProfile | null =
		data && data.profiles[activeProfile] ? data.profiles[activeProfile] : null;

	// Reset the draft whenever the active profile (or the loaded data) changes.
	useEffect(() => {
		const agents = currentProfile?.agents ?? {};
		setDraft(
			Object.fromEntries(Object.entries(agents).map(([name, m]) => [name, m.model ?? ""])),
		);
		setDirty(false);
		setOmpDraft(data?.omp_model_roles ?? {});
		setOmpDirty(false);
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
				omp_model_roles: ompDraft,
			})
			.then((res) => {
				// Merge so we keep shipped/omp_model_roles/agent_groups that the
				// update response omits. The old code replaced `data` wholesale and
				// silently dropped them, blanking the OMP section after a save.
				setData((prev) =>
					prev
						? { ...prev, profiles: res.profiles, active: res.active }
						: { profiles: res.profiles, active: res.active },
				);
				setDirty(false);
				setOmpDirty(false);
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
				setData((prev) =>
					prev
						? { ...prev, profiles: res.profiles, active: res.active }
						: { profiles: res.profiles, active: res.active },
				);
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
			.then((res) => setData(res))
			.catch((err) => setMessage(`Error: ${err.message}`))
			.finally(() => setResyncing(false));
	};

	// An empty modelId is a real choice, not a missing one: it clears the
	// per-agent override so the agent follows the lead model. Only an empty
	// target list is a no-op.
	const applyModelToAgents = (modelId: string, names: string[]) => {
		if (names.length === 0) return;
		setDraft((prev) => {
			const next = { ...prev };
			for (const name of names) {
				next[name] = modelId;
			}
			return next;
		});
		setDirty(true);
	};

	const profileNames = data ? Object.keys(data.profiles) : [];
	const lowerFilter = agentFilter.toLowerCase();
	const allAgentNames = useMemo(
		() => Object.keys(draft).sort((a, b) => a.localeCompare(b)),
		[draft],
	);

	// Folder for an agent, from the backend's agent_groups map. Fall back to the
	// legacy "core" bucket so an unknown agent is still editable somewhere.
	const agentGroups = data?.agent_groups ?? {};
	const folderOf = (name: string) => agentGroups[name] ?? "core";

	// All groups present in the data, in the preferred order, unknowns appended.
	const groups = useMemo(() => {
		const present = new Set(Object.values(agentGroups));
		// Also include any group referenced by an agent the map missed.
		for (const name of allAgentNames) present.add(folderOf(name));
		present.delete("");
		const ordered = GROUP_ORDER.filter((g) => present.has(g));
		for (const g of [...present].sort()) {
			if (!ordered.includes(g)) ordered.push(g);
		}
		return ordered;
	}, [agentGroups, allAgentNames, agentGroups]);

	const agentsByGroup = useMemo(() => {
		const acc: Record<string, string[]> = {};
		for (const name of allAgentNames) {
			const g = folderOf(name);
			(acc[g] ||= []).push(name);
		}
		return acc;
	}, [allAgentNames, agentGroups]);

	// Keep a valid active group when the roster changes (e.g. profile switch).
	useEffect(() => {
		if (groups.length === 0) return;
		if (!groups.includes(activeGroup)) setActiveGroup(groups[0]);
	}, [groups, activeGroup]);

	// Agents visible in the current sub-tab, after the name filter.
	const groupAgents = agentsByGroup[activeGroup] ?? [];
	const visibleAgents = useMemo(
		() => groupAgents.filter((name) => name.toLowerCase().includes(lowerFilter)),
		[groupAgents, lowerFilter],
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

	if (loading && !data) {
		return (
			<div className="settings-section">
				<div className="settings-section-body">
					<div className="loading-inline">
						<div className="spinner" />
						<span>Loading profiles…</span>
					</div>
				</div>
			</div>
		);
	}

	const isDirty = dirty || ompDirty;

	return (
		<div className="profiles-root">
			{message && (
				<div
					className={`alert ${message.startsWith("Error") ? "alert-danger" : "alert-success"}`}
				>
					{message}
				</div>
			)}

			{/* ── Profile selector ─────────────────────────────────────────── */}
			<section className="settings-section">
				<div className="settings-section-header">
					<h2>
						<Boxes size={14} style={{ verticalAlign: "-2px", marginRight: 6 }} />
						Orchestrator Profiles
					</h2>
					<button
						type="button"
						className="btn btn-sm"
						onClick={handleSave}
						disabled={saving || !isDirty}
					>
						<Save size={14} />
						{saving ? "Saving…" : "Save"}
					</button>
				</div>
				<div className="settings-section-body">
					<div className="profiles-pills">
						{profileNames.map((name) => {
							const p = data?.profiles[name];
							const isActive = activeProfile === name;
							return (
								<button
									key={name}
									type="button"
									className={`profiles-pill ${isActive ? "active" : ""}`}
									onClick={() => handleActiveChange(name)}
									disabled={saving}
									title={p?.description}
								>
									<span className="profiles-pill-dot" />
									<span className="profiles-pill-name">
										{p?.display_name ?? name}
									</span>
									{(data?.shipped ?? []).includes(name) && (
										<span className="profiles-pill-tag">seed</span>
									)}
								</button>
							);
						})}
						<button
							type="button"
							className="profiles-pill ghost"
							onClick={handleAddProfile}
							disabled={saving}
							title="Create a new profile seeded from the current one"
						>
							<Plus size={14} />
							<span className="profiles-pill-name">New</span>
						</button>
					</div>

					{currentProfile && (
						<p className="profiles-description muted">
							{currentProfile.description}
						</p>
					)}

					{isShipped && (
						<div className="profiles-shipped-note">
							<AlertTriangle size={14} style={{ flexShrink: 0, marginTop: 2 }} />
							<span>
								<strong>{activeProfile}</strong> ships with ywai: its agent roster is
								refreshed on every install (so new agents appear), but your model and
								OMP modelRoles choices persist. For a fully custom setup, save under a
								new profile name.
							</span>
						</div>
					)}
				</div>
			</section>

			{/* ── Quick set (operates on the active group sub-tab) ─────────── */}
			<section className="settings-section">
				<div className="settings-section-header">
					<h2>
						<Zap size={14} style={{ verticalAlign: "-2px", marginRight: 6 }} />
						Quick set
					</h2>
					<span className="profiles-scope muted">
						applies to: {groupLabel(activeGroup)}
					</span>
				</div>
				<div className="settings-section-body">
					<div className="profiles-quickset">
						<div className="profiles-quickset-picker">
							<ModelCombobox
								id="bulk-model"
								label=""
								value={bulkModel}
								models={models}
								onChange={setBulkModel}
							/>
						</div>
						<div className="profiles-quickset-actions">
							<button
								type="button"
								className="btn btn-sm"
								disabled={!bulkModel || groupAgents.length === 0}
								onClick={() => applyModelToAgents(bulkModel, groupAgents)}
							>
								This group ({groupAgents.length})
							</button>
							<button
								type="button"
								className="btn btn-sm"
								disabled={!bulkModel || allAgentNames.length === 0}
								onClick={() => applyModelToAgents(bulkModel, allAgentNames)}
							>
								All agents
							</button>
							{/* Clearing a model is not the same action as setting one: it needs no
							    selection, and it is the way back to "whatever the lead agent uses"
							    after experimenting. Kept separate so it cannot fire by accident. */}
							<button
								type="button"
								className="btn btn-sm btn-ghost"
								disabled={groupAgents.length === 0}
								title="Clear every per-agent model in this group so each follows the lead agent"
								onClick={() => applyModelToAgents("", groupAgents)}
							>
								Inherit here
							</button>
						</div>
					</div>
				</div>
			</section>

			{/* ── OMP modelRoles — editable like the per-agent models. ──────── */}
			{data?.omp_model_roles && Object.keys(data.omp_model_roles).length > 0 && (
				<section className="settings-section">
					<div className="settings-section-header">
						<h2>OMP modelRoles</h2>
						<div className="profiles-section-tags">
							<span className="pill pill-muted">oh-my-pi</span>
							{ompDirty && <span className="pill pill-success">unsaved</span>}
						</div>
					</div>
					<div className="settings-section-body">
						<div className="profiles-omp-grid">
							{Object.keys(data.omp_model_roles)
								.sort()
								.map((role) => (
									<div key={role} className="profiles-omp-row">
										<span className="profiles-omp-role">{role}</span>
										<div className="profiles-omp-picker">
											<ModelCombobox
												id={`omp-role-model-${role}`}
												label=""
												value={ompDraft[role] ?? ""}
												models={models}
												onChange={(v) => {
													setOmpDraft((prev) => ({ ...prev, [role]: v }));
													setOmpDirty(true);
												}}
											/>
										</div>
									</div>
								))}
						</div>
						<p className="muted profiles-omp-help">
							Written to ~/.omp/agent/config.yml on save. Empty value falls back to the
							derived mapping.
						</p>
					</div>
				</section>
			)}

			{/* ── Per-agent models, grouped by folder sub-tabs ─────────────── */}
			<section className="settings-section">
				<div className="settings-section-header">
					<h2>Agent models</h2>
					<button
						type="button"
						className="btn btn-sm btn-ghost"
						onClick={handleResync}
						disabled={resyncing}
					>
						<RefreshCw size={14} className={resyncing ? "spin" : ""} />
						{resyncing ? "Resyncing…" : "Resync from Seed"}
					</button>
				</div>
				<div className="settings-section-body">
					{groups.length > 0 && (
						<div className="profiles-subtabs" role="tablist">
							{groups.map((g) => {
								const count = agentsByGroup[g]?.length ?? 0;
								return (
									<button
										key={g}
										type="button"
										role="tab"
										aria-selected={activeGroup === g}
										className={`profiles-subtab ${activeGroup === g ? "active" : ""}`}
										onClick={() => setActiveGroup(g)}
									>
										{groupLabel(g)}
										<span className="profiles-subtab-count">{count}</span>
									</button>
								);
							})}
						</div>
					)}

					<div className="profiles-filter">
						<Search
							size={14}
							className="profiles-filter-icon"
						/>
						<input
							type="text"
							className="input"
							placeholder="Filter agents…"
							value={agentFilter}
							onChange={(e) => setAgentFilter(e.target.value)}
						/>
					</div>

					{visibleAgents.length > 0 ? (
						<table className="profiles-table">
							<thead>
								<tr>
									<th>Agent</th>
									<th>Model</th>
								</tr>
							</thead>
							<tbody>
								{visibleAgents.map((name) => {
									// Show quick chips for models used by other agents.
									const quickModels = inUseModels
										.filter((m) => m.id !== draft[name])
										.slice(0, 4);
									return (
										<tr key={name}>
											<td className="profiles-table-agent">
												<span className="profiles-table-agent-name">{name}</span>
											</td>
											<td>
												<div className="profiles-model-cell">
													<div className="profiles-model-picker">
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
														<div className="profiles-quickchips">
															{quickModels.map((m) => (
																<button
																	key={m.id}
																	type="button"
																	className="pill pill-muted profiles-quickchip"
																	title={m.name || m.id}
																	onClick={() => {
																		setDraft((prev) => ({
																			...prev,
																			[name]: m.id,
																		}));
																		setDirty(true);
																	}}
																>
																	{m.name || m.id}
																</button>
															))}
														</div>
													)}
												</div>
											</td>
										</tr>
									);
								})}
							</tbody>
						</table>
					) : (
						<p className="muted profiles-empty">
							{agentFilter
								? "No agents match your filter"
								: "No agent model mappings for this group"}
						</p>
					)}
				</div>
			</section>
		</div>
	);
}
