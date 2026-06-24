import { useState, useEffect } from "react";
import { RefreshCw, Check } from "lucide-react";
import { configApi } from "../../api/client";
import type { OrchestratorProfilesResponse, OrchestratorProfile } from "../../api/types";

type Group = "agents" | "tasks";

const AGENT_ROLES = ["default", "scout", "finder", "explore", "implement", "review", "deploy"];
const TASK_ROLES = ["planning", "ask", "research", "test"];

const ROLE_LABELS: Record<string, string> = {
	default: "Default / Orchestrator",
	scout: "Scout / Finder",
	finder: "Finder",
	explore: "Explore",
	implement: "Implement (Dev)",
	review: "Reviewer",
	deploy: "Devops / Deploy",
	planning: "Planning",
	ask: "Ask / Research",
	research: "Research",
	test: "Test (QA)",
};

export default function ProfilesTab() {
	const [data, setData] = useState<OrchestratorProfilesResponse | null>(null);
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [resyncing, setResyncing] = useState(false);
	const [message, setMessage] = useState<string | null>(null);
	const [group, setGroup] = useState<Group>("agents");

	const fetchProfiles = () => {
		setLoading(true);
		configApi
			.getOrchestratorProfiles()
			.then((res) => {
				setData(res);
			})
			.catch((err) => {
				setMessage(`Error loading profiles: ${err.message}`);
			})
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
				setMessage("Active profile updated");
			})
			.catch((err) => {
				setMessage(`Error: ${err.message}`);
			})
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
			.catch((err) => {
				setMessage(`Error: ${err.message}`);
			})
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

	const filteredRoles =
		group === "agents" ? AGENT_ROLES : TASK_ROLES;

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

			{/* Current profile info */}
			{currentProfile && (
				<div style={{ marginBottom: "1.5rem" }}>
					<p className="muted" style={{ margin: "0 0 0.25rem" }}>
						{currentProfile.description}
					</p>
				</div>
			)}

			{/* Group tabs */}
			<div className="tabs" style={{ marginBottom: "1rem" }}>
				<button
					type="button"
					className={`tab ${group === "agents" ? "active" : ""}`}
					onClick={() => setGroup("agents")}
				>
					Agents
				</button>
				<button
					type="button"
					className={`tab ${group === "tasks" ? "active" : ""}`}
					onClick={() => setGroup("tasks")}
				>
					Tasks
				</button>
			</div>

			{/* Mappings table */}
			{currentProfile && currentProfile.role_defaults ? (
				<table style={{ width: "100%", borderCollapse: "collapse" }}>
					<thead>
						<tr>
							<th style={{ textAlign: "left", padding: "0.5rem 0.75rem", borderBottom: "1px solid var(--color-border, #ddd)" }}>Role</th>
							<th style={{ textAlign: "left", padding: "0.5rem 0.75rem", borderBottom: "1px solid var(--color-border, #ddd)" }}>Agent</th>
							<th style={{ textAlign: "left", padding: "0.5rem 0.75rem", borderBottom: "1px solid var(--color-border, #ddd)" }}>Model</th>
						</tr>
					</thead>
					<tbody>
						{filteredRoles.map((role) => {
							const mapping = currentProfile.role_defaults?.[role];
							if (!mapping) return null;
							return (
								<tr key={role}>
									<td style={{ padding: "0.4rem 0.75rem", borderBottom: "1px solid var(--color-border, #eee)", verticalAlign: "top" }}>
										{ROLE_LABELS[role] ?? role}
									</td>
									<td style={{ padding: "0.4rem 0.75rem", borderBottom: "1px solid var(--color-border, #eee)" }}>
										<span className="pill pill-muted">{mapping.agent}</span>
									</td>
									<td style={{ padding: "0.4rem 0.75rem", borderBottom: "1px solid var(--color-border, #eee)" }}>
										<span className="pill pill-muted">{mapping.model}</span>
									</td>
								</tr>
							);
						})}
						{filteredRoles.every((r) => !currentProfile.role_defaults?.[r]) && (
							<tr>
								<td colSpan={3} style={{ padding: "1rem", textAlign: "center" }} className="muted">
									No {group} role mappings defined
								</td>
							</tr>
						)}
					</tbody>
				</table>
			) : (
				<p className="muted">No role mappings for this profile</p>
			)}

			{/* Resync button */}
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
