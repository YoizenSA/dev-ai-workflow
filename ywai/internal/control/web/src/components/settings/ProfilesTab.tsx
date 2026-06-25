import { useState, useEffect } from "react";
import { RefreshCw, Check, Save } from "lucide-react";
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
	const agentNames = Object.keys(draft).sort((a, b) => {
		const ga = GROUP_ORDER.indexOf(agentGroup(a));
		const gb = GROUP_ORDER.indexOf(agentGroup(b));
		return ga !== gb ? ga - gb : a.localeCompare(b);
	});

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
				<label style={{ display: "block", marginBottom: "0.5rem", fontWeight: 600 }}>Active Profile</label>
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
				</div>
			</div>

			{currentProfile && (
				<p className="muted" style={{ margin: "0 0 1rem" }}>
					{currentProfile.description}
				</p>
			)}

			{/* Editable per-agent model table */}
			{agentNames.length > 0 ? (
				<table style={{ width: "100%", borderCollapse: "collapse" }}>
					<thead>
						<tr>
							<th style={{ textAlign: "left", padding: "0.5rem 0.75rem", borderBottom: "1px solid var(--color-border, #ddd)" }}>Agent</th>
							<th style={{ textAlign: "left", padding: "0.5rem 0.75rem", borderBottom: "1px solid var(--color-border, #ddd)" }}>Model</th>
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
							rows.push(
								<tr key={name}>
									<td style={{ padding: "0.4rem 0.75rem", borderBottom: "1px solid var(--color-border, #eee)" }}>{name}</td>
									<td style={{ padding: "0.4rem 0.75rem", borderBottom: "1px solid var(--color-border, #eee)" }}>
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
									</td>
								</tr>,
							);
							return rows;
						})}
					</tbody>
				</table>
			) : (
				<p className="muted">No agent model mappings for this profile</p>
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
