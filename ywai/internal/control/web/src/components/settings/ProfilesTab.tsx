import { useState, useEffect } from "react";
import { RefreshCw, Check } from "lucide-react";
import { configApi } from "../../api/client";
import type { OrchestratorProfilesResponse, OrchestratorProfile } from "../../api/types";

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
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [resyncing, setResyncing] = useState(false);
	const [message, setMessage] = useState<string | null>(null);

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
	}, []);

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
	const activeProfile = data?.active ?? "";
	const currentProfile: OrchestratorProfile | null =
		data && data.profiles[activeProfile] ? data.profiles[activeProfile] : null;

	const agents = currentProfile?.agents ?? {};
	const agentNames = Object.keys(agents).sort((a, b) => {
		const ga = GROUP_ORDER.indexOf(agentGroup(a));
		const gb = GROUP_ORDER.indexOf(agentGroup(b));
		return ga !== gb ? ga - gb : a.localeCompare(b);
	});

	return (
		<div className="card card-pad">
			<div className="card-header">
				<h3>Orchestrator Profiles</h3>
			</div>

			{message && (
				<div
					className="pill"
					style={{
						marginBottom: "1rem",
						background: message.startsWith("Error")
							? "var(--color-danger-bg, #fee)"
							: "var(--color-success-bg, #efe)",
					}}
				>
					{message}
				</div>
			)}

			{/* Profile selector */}
			<div style={{ marginBottom: "1.5rem" }}>
				<label style={{ display: "block", marginBottom: "0.5rem", fontWeight: 600 }}>
					Active Profile
				</label>
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

			{/* Per-agent model table */}
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
									<td style={{ padding: "0.4rem 0.75rem", borderBottom: "1px solid var(--color-border, #eee)" }}>
										{name}
									</td>
									<td style={{ padding: "0.4rem 0.75rem", borderBottom: "1px solid var(--color-border, #eee)" }}>
										<span className="pill pill-muted">{agents[name].model}</span>
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
		</div>
	);
}
