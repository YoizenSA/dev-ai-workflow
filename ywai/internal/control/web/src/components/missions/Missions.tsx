import React, { useEffect, useCallback, useState, useRef } from "react";
import { useSearchParams } from "react-router-dom";
import { useMissionsStore } from "../../stores/missionsStore";
import { useWebSocket } from "../../hooks/useWebSocket";
import type { Mission, WSMessage } from "../../api/types";
import CreateMissionModal from "./CreateMissionModal";
import "./Missions.css";

const STATUS_PILL: Record<string, string> = {
	pending: "pill-muted",
	planning: "pill-primary",
	active: "pill-info",
	paused: "pill-warning",
	completed: "pill-success",
	failed: "pill-danger",
	cancelled: "pill-muted",
	validating: "pill-accent",
};

const STATUS_LABEL: Record<string, string> = {
	pending: "Pending",
	planning: "Planning",
	active: "Active",
	paused: "Paused",
	completed: "Completed",
	failed: "Failed",
	cancelled: "Cancelled",
	validating: "Validating",
};

function MissionCard({ mission }: { mission: Mission }) {
	const {
		runMission,
		pauseMission,
		resumeMission,
		cancelMission,
		deleteMission,
		selectMission,
		selectedMission,
	} = useMissionsStore();

	const isActive = selectedMission?.id === mission.id;
	const completedFeatures = (mission.features ?? []).filter(
		(f) => f.status === "completed",
	).length;
	const totalFeatures = mission.features?.length ?? mission.featureCount ?? 0;
	const progress =
		totalFeatures > 0
			? Math.round((completedFeatures / totalFeatures) * 100)
			: 0;

	const pillClass = STATUS_PILL[mission.status] ?? "pill-muted";

	return (
		<div
			className={`mission-card${isActive ? " active" : ""}`}
			onClick={() => selectMission(mission.id)}
		>
			<div className="mission-card-header">
				<div className="row wrap" style={{ gap: "var(--space-2)" }}>
					<span className={`pill ${pillClass}`}>
						<span className="dot"></span>
						{STATUS_LABEL[mission.status] ?? mission.status}
					</span>
					{mission.agent && (
						<span className="pill pill-muted">{mission.agent}</span>
					)}
				</div>
				<div className="row" style={{ gap: "var(--space-1)" }}>
					{["pending", "planning"].includes(mission.status) && (
						<button
							className="btn btn-sm btn-primary"
							onClick={(e) => {
								e.stopPropagation();
								runMission(mission.id);
							}}
						>
							Run
						</button>
					)}
					{mission.status === "active" && (
						<button
							className="btn btn-sm"
							onClick={(e) => {
								e.stopPropagation();
								pauseMission(mission.id);
							}}
						>
							Pause
						</button>
					)}
					{mission.status === "paused" && (
						<button
							className="btn btn-sm"
							onClick={(e) => {
								e.stopPropagation();
								resumeMission(mission.id);
							}}
						>
							Resume
						</button>
					)}
					{["pending", "active", "paused", "planning"].includes(mission.status) && (
						<button
							className="btn btn-sm btn-danger"
							onClick={(e) => {
								e.stopPropagation();
								cancelMission(mission.id);
							}}
						>
							Cancel
						</button>
					)}
					{["completed", "cancelled", "failed"].includes(mission.status) && (
						<button
							className="btn btn-sm btn-danger"
							onClick={(e) => {
								e.stopPropagation();
								deleteMission(mission.id);
							}}
						>
							Delete
						</button>
					)}
				</div>
			</div>

			<h3 className="mission-card-name">{mission.name}</h3>
			{mission.project && (
				<p className="muted" style={{ margin: 0, fontSize: "0.82rem" }}>
					{mission.project}
				</p>
			)}

			{totalFeatures > 0 && (
				<div className="mission-card-progress">
					<div className="row" style={{ justifyContent: "space-between" }}>
						<span className="muted" style={{ fontSize: "0.78rem" }}>
							{completedFeatures}/{totalFeatures} features
						</span>
						<span className="muted tnum" style={{ fontSize: "0.78rem" }}>
							{progress}%
						</span>
					</div>
					<div className="progress-track">
						<div className="progress-fill" style={{ width: `${progress}%` }} />
					</div>
				</div>
			)}
		</div>
	);
}

function MissionReport({ missionId }: { missionId: string }) {
	const { reportContent, reportLoading, fetchReport } = useMissionsStore();
	const [open, setOpen] = useState(false);

	useEffect(() => {
		if (open && reportContent === "" && !reportLoading) {
			fetchReport(missionId);
		}
	}, [open, missionId, reportContent, reportLoading, fetchReport]);

	return (
		<div className="section-head" style={{ marginTop: "var(--space-4)", cursor: "pointer" }} onClick={() => setOpen((v) => !v)}>
			<span className="section-tick" style={{ background: "var(--info)", boxShadow: "0 0 8px rgba(var(--info-rgb), 0.5)" }}></span>
			<span className="section-title">Mission Report</span>
			<span className="expand-chevron">{open ? "▼" : "▶"}</span>
			{open && (
				<div className="card" style={{ marginTop: "var(--space-3)", width: "100%" }} onClick={(e) => e.stopPropagation()}>
					{reportLoading ? (
						<div className="loading-inline">
							<div className="spinner"></div>
							<span>Loading report…</span>
						</div>
					) : reportContent ? (
						<pre className="report-viewer">{reportContent}</pre>
					) : (
						<div className="empty-state" style={{ padding: "var(--space-3)" }}>
							<span className="empty-desc">
								No report generated yet. Reports are produced when a mission completes successfully.
							</span>
						</div>
					)}
				</div>
			)}
		</div>
	);
}

function MissionDetail({ mission }: { mission: Mission }) {
	const {
		runMission,
		pauseMission,
		resumeMission,
		cancelMission,
		deleteMission,
		featureLogs,
		expandedFeatures,
		toggleFeatureExpanded,
		fetchFeatureLogs,
	} = useMissionsStore();
	const featurePill = (status: string) => STATUS_PILL[status] ?? "pill-muted";

	const formatDate = (d?: string | null) => {
		if (!d) return "—";
		try {
			return new Date(d).toLocaleString();
		} catch {
			return d;
		}
	};

	useEffect(() => {
		if (mission?.status === 'active' && mission.features) {
			mission.features.forEach((f) => {
				if (f.status === 'in_progress' && !expandedFeatures.has(f.id) && f.skillName) {
					toggleFeatureExpanded(f.id)
					fetchFeatureLogs(mission.id, f.id)
				}
			})
		}
	}, [mission?.status, mission?.features])

	return (
		<section className="mission-detail">
			<div className="section-head">
				<span className="section-tick"></span>
				<span className="section-title">{mission.name}</span>
				<span className={`pill ${STATUS_PILL[mission.status] ?? "pill-muted"}`}>
					<span className="dot"></span>
					{STATUS_LABEL[mission.status] ?? mission.status}
				</span>
			</div>

			{/* Mission metadata grid */}
			<div className="card">
				<div className="detail-grid">
					<div className="detail-field">
						<span className="detail-label">Project</span>
						<span className="detail-value">{mission.project ?? "—"}</span>
					</div>
					<div className="detail-field">
						<span className="detail-label">Agent</span>
						<span className="detail-value">{mission.agent ?? "—"}</span>
					</div>
					<div className="detail-field">
						<span className="detail-label">Model</span>
						<span className="detail-value">{mission.model ?? "—"}</span>
					</div>
					<div className="detail-field">
						<span className="detail-label">Features</span>
						<span className="detail-value tnum">
							{mission.features?.length ?? mission.featureCount ?? 0}
						</span>
					</div>
					<div className="detail-field">
						<span className="detail-label">Created</span>
						<span className="detail-value tnum">{formatDate(mission.createdAt)}</span>
					</div>
					<div className="detail-field">
						<span className="detail-label">Updated</span>
						<span className="detail-value tnum">{formatDate(mission.updatedAt)}</span>
					</div>
					{mission.completedAt && (
						<div className="detail-field">
							<span className="detail-label">Completed</span>
							<span className="detail-value tnum">{formatDate(mission.completedAt)}</span>
						</div>
					)}
				</div>

				{/* Action buttons */}
				<div className="row" style={{ gap: "var(--space-2)", marginTop: "var(--space-3)" }}>
					{["pending", "planning"].includes(mission.status) && (
						<button className="btn btn-sm btn-primary" onClick={() => runMission(mission.id)}>
							Run
						</button>
					)}
					{mission.status === "active" && (
						<button className="btn btn-sm btn-warn" onClick={() => pauseMission(mission.id)}>
							Pause
						</button>
					)}
					{mission.status === "paused" && (
						<button className="btn btn-sm btn-primary" onClick={() => resumeMission(mission.id)}>
							Resume
						</button>
					)}
					{(mission.status === "active" || mission.status === "paused" || mission.status === "pending" || mission.status === "planning") && (
						<button className="btn btn-sm btn-danger" onClick={() => cancelMission(mission.id)}>
							Cancel
						</button>
					)}
					{["completed", "cancelled", "failed"].includes(mission.status) && (
						<button className="btn btn-sm btn-danger" onClick={() => deleteMission(mission.id)}>
							Delete
						</button>
					)}
				</div>
			</div>

			{/* Features table */}
			<div className="section-head" style={{ marginTop: "var(--space-4)" }}>
				<span className="section-tick"></span>
				<span className="section-title">Features</span>
			</div>
			<div className="card">
				{mission.features && mission.features.length > 0 ? (
					<table className="data-table">
						<thead>
							<tr>
								<th>Feature</th>
								<th style={{ textAlign: "center" }}>Status</th>
								<th>Skill</th>
							</tr>
						</thead>
						<tbody>
							{mission.features.map((f) => (
								<React.Fragment key={f.id}>
									<tr
										className={f.skillName ? "feature-row feature-row--expandable" : "feature-row"}
										onClick={f.skillName ? () => {
											toggleFeatureExpanded(f.id)
											if (!expandedFeatures.has(f.id)) {
												fetchFeatureLogs(mission.id, f.id)
											}
										} : undefined}
									>
										<td>
											{f.skillName && (
												<span className="expand-chevron">
													{expandedFeatures.has(f.id) ? '▼' : '▶'}
												</span>
											)}
											<span className="cell-strong">{f.description}</span>
										</td>
										<td style={{ textAlign: "center" }}>
											<span className={`pill ${featurePill(f.status)}`}>
												<span className="dot"></span>
												{f.status}
											</span>
										</td>
										<td className="muted">{f.skillName ?? "—"}</td>
									</tr>
									{expandedFeatures.has(f.id) && (
										<tr className="feature-log-row">
											<td colSpan={3}>
												<FeatureLogViewer
													isLive={mission.status === 'active' && f.status === 'in_progress'}
													lines={featureLogs[f.id] ?? []}
												/>
											</td>
										</tr>
									)}
								</React.Fragment>
							))}
						</tbody>
					</table>
				) : (
					<div className="empty-state">
						<div className="empty-icon">
							<svg
								width="20"
								height="20"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								strokeWidth="2"
								strokeLinecap="round"
								strokeLinejoin="round"
							>
								<path d="M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 0 0-2.91-.09z" />
								<path d="m12 15-3-3a22 22 0 0 1 2-3.95A12.88 12.88 0 0 1 22 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 0 1-4 2z" />
							</svg>
						</div>
						<span className="empty-title">No features yet</span>
						<span className="empty-desc">
							Features will appear as the mission progresses
						</span>
						</div>
					)}
				</div>

			{["completed", "failed", "cancelled"].includes(mission.status) && (
				<MissionReport missionId={mission.id} />
			)}
		</section>
	);
}

function FeatureLogViewer({ isLive, lines }: {
	isLive: boolean
	lines: string[]
}) {
	const scrollRef = useRef<HTMLDivElement>(null)

	useEffect(() => {
		if (scrollRef.current) {
			scrollRef.current.scrollTop = scrollRef.current.scrollHeight
		}
	}, [lines])

	return (
		<div className="log-viewer-wrapper">
			{isLive && (
				<div className="live-badge">
					<span className="dot"></span>
					LIVE
				</div>
			)}
			<div className="log-viewer" ref={scrollRef}>
				{lines.length === 0 ? (
					<span className="log-viewer-empty">No output yet</span>
				) : (
					lines.map((line, i) => (
						<div
							key={i}
							className={line.startsWith('[stderr]') ? 'log-viewer-line log-viewer-line--stderr' : 'log-viewer-line'}
						>
							{line}
						</div>
					))
				)}
			</div>
		</div>
	)
}

export default function Missions() {
	const [showNewMission, setShowNewMission] = useState(false);
	const [toast, setToast] = useState<string | null>(null);
	const prevMissionCount = useRef(0);
	const modalWasOpen = useRef(false);

	const {
		missions,
		projects,
		loading,
		selectedMission,
		selectMission,
		fetchMissions,
		fetchProjects,
	} = useMissionsStore();

	const [searchParams, setSearchParams] = useSearchParams();

	const handleWSMessage = useCallback((msg: WSMessage) => {
		useMissionsStore.getState().handleWSMessage(msg);
	}, []);

	useWebSocket("/missions/ws", handleWSMessage);

	useEffect(() => {
		fetchMissions();
		fetchProjects();
	}, [fetchMissions, fetchProjects]);

	// Restore the selected mission from ?mission=<id> once missions are loaded
	// (e.g. after F5 / deep-link).
	useEffect(() => {
		const id = searchParams.get("mission");
		if (!id || selectedMission || missions.length === 0) return;
		if (missions.some((m) => m.id === id)) selectMission(id);
	}, [missions, selectedMission, searchParams, selectMission]);

	// Keep the URL in sync with the selected mission so reloads/sharing work.
	useEffect(() => {
		const current = searchParams.get("mission") ?? "";
		const next = selectedMission?.id ?? "";
		if (current === next) return;
		const params = new URLSearchParams(searchParams);
		if (next) params.set("mission", next);
		else params.delete("mission");
		setSearchParams(params, { replace: true });
	}, [selectedMission, searchParams, setSearchParams]);

	// Track modal open/close
	useEffect(() => {
		if (showNewMission) {
			modalWasOpen.current = true;
			prevMissionCount.current = missions.length;
		}
	}, [showNewMission, missions.length]);

	// Show toast when a new mission is created after modal closes
	useEffect(() => {
		if (modalWasOpen.current && !showNewMission && missions.length > prevMissionCount.current) {
			setToast("Mission created and running!");
			const timer = setTimeout(() => setToast(null), 4000);
			modalWasOpen.current = false;
			return () => clearTimeout(timer);
		}
		prevMissionCount.current = missions.length;
	}, [missions.length, showNewMission]);

	const activeMissions = missions.filter(
		(m) => !["completed", "cancelled", "failed"].includes(m.status),
	);
	const completedMissions = missions.filter((m) =>
		["completed", "cancelled", "failed"].includes(m.status),
	);
	const runningCount = missions.filter((m) => m.status === "active").length;
	const pausedCount = missions.filter((m) => m.status === "paused").length;

	if (loading && missions.length === 0) {
		return (
			<div className="loading-inline">
				<div className="spinner"></div>
				<span>Loading missions…</span>
			</div>
		);
	}

	return (
		<div className="missions">
			{/* Page header */}
			<header className="page-header">
				<div className="page-heading">
					<span className="page-eyebrow">Missions</span>
					<h1 className="page-title">Mission Control</h1>
					<p className="page-subtitle">
						{activeMissions.length} active · {completedMissions.length}{" "}
						completed · {projects.length} projects
					</p>
				</div>
				<div className="page-actions">
					<button
						className="btn btn-primary"
						onClick={() => setShowNewMission(true)}
					>
						<svg
							width="16"
							height="16"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							strokeWidth="2.5"
							strokeLinecap="round"
						>
							<line x1="12" y1="5" x2="12" y2="19" />
							<line x1="5" y1="12" x2="19" y2="12" />
						</svg>
						New Mission
					</button>
				</div>
			</header>

			{/* KPI summary row */}
			<div className="kpi-grid">
				<div
					className="kpi"
					style={
						{
							"--kpi-glow": "rgba(var(--info-rgb), 0.30)",
							"--kpi-icon-bg": "rgba(var(--yz-primary-1-rgb), 0.16)",
							"--kpi-icon-color": "var(--tint-info)",
						} as React.CSSProperties
					}
				>
					<div className="kpi-top">
						<div className="kpi-icon">
							<svg
								width="20"
								height="20"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								strokeWidth="2"
								strokeLinecap="round"
								strokeLinejoin="round"
							>
								<path d="M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 0 0-2.91-.09z" />
								<path d="m12 15-3-3a22 22 0 0 1 2-3.95A12.88 12.88 0 0 1 22 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 0 1-4 2z" />
							</svg>
						</div>
					</div>
					<div className="kpi-value tnum">{missions.length}</div>
					<div className="kpi-label">Total Missions</div>
				</div>

				<div
					className="kpi"
					style={
						{
							"--kpi-glow": "rgba(var(--success-bright-rgb), 0.30)",
							"--kpi-icon-bg": "rgba(var(--success-rgb), 0.16)",
							"--kpi-icon-color": "var(--tint-success)",
						} as React.CSSProperties
					}
				>
					<div className="kpi-top">
						<div className="kpi-icon">
							<svg
								width="20"
								height="20"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								strokeWidth="2"
								strokeLinecap="round"
								strokeLinejoin="round"
							>
								<polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />
							</svg>
						</div>
					</div>
					<div className="kpi-value tnum">{runningCount}</div>
					<div className="kpi-label">Running</div>
				</div>

				<div
					className="kpi"
					style={
						{
							"--kpi-glow": "rgba(var(--yz-yellow-rgb), 0.25)",
							"--kpi-icon-bg": "rgba(var(--yz-yellow-rgb), 0.15)",
							"--kpi-icon-color": "var(--warning)",
						} as React.CSSProperties
					}
				>
					<div className="kpi-top">
						<div className="kpi-icon">
							<svg
								width="20"
								height="20"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								strokeWidth="2"
								strokeLinecap="round"
								strokeLinejoin="round"
							>
								<rect x="6" y="4" width="4" height="16" rx="1" />
								<rect x="14" y="4" width="4" height="16" rx="1" />
							</svg>
						</div>
					</div>
					<div className="kpi-value tnum">{pausedCount}</div>
					<div className="kpi-label">Paused</div>
				</div>

				<div
					className="kpi"
					style={
						{
							"--kpi-glow": "rgba(var(--purple-bright-rgb), 0.25)",
							"--kpi-icon-bg": "rgba(var(--yz-primary-2-rgb), 0.16)",
							"--kpi-icon-color": "var(--tint-purple)",
						} as React.CSSProperties
					}
				>
					<div className="kpi-top">
						<div className="kpi-icon">
							<svg
								width="20"
								height="20"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								strokeWidth="2"
								strokeLinecap="round"
								strokeLinejoin="round"
							>
								<path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
								<polyline points="22 4 12 14.01 9 11.01" />
							</svg>
						</div>
					</div>
					<div className="kpi-value tnum">{completedMissions.length}</div>
					<div className="kpi-label">Completed</div>
				</div>
			</div>

			{/* Active missions */}
			{activeMissions.length > 0 && (
				<section className="missions-section">
					<div className="section-head">
						<span className="section-tick"></span>
						<span className="section-title">Active Missions</span>
					</div>
					<div className="grid-2">
						{activeMissions.map((m) => (
							<MissionCard key={m.id} mission={m} />
						))}
					</div>
				</section>
			)}

			{/* Selected mission detail */}
			{selectedMission && <MissionDetail mission={selectedMission} />}

			{/* Completed missions */}
			{completedMissions.length > 0 && (
				<section className="missions-section">
					<div className="section-head">
						<span
							className="section-tick"
							style={{
								background: "var(--success)",
								boxShadow: "0 0 8px rgba(var(--success-bright-rgb), 0.5)",
							}}
						></span>
						<span className="section-title">Completed</span>
					</div>
					<div className="grid-2">
						{completedMissions.map((m) => (
							<MissionCard key={m.id} mission={m} />
						))}
					</div>
				</section>
			)}

			{/* Empty state */}
			{missions.length === 0 && (
				<div className="empty-state">
					<div className="empty-icon">
						<svg
							width="26"
							height="26"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							strokeWidth="2"
							strokeLinecap="round"
							strokeLinejoin="round"
						>
							<path d="M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 0 0-2.91-.09z" />
							<path d="m12 15-3-3a22 22 0 0 1 2-3.95A12.88 12.88 0 0 1 22 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 0 1-4 2z" />
						</svg>
					</div>
					<span className="empty-title">No missions yet</span>
					<span className="empty-desc">
						Create a mission to get started with automated workflows
					</span>
					<button
						className="btn btn-primary"
						style={{ marginTop: "var(--space-3)" }}
						onClick={() => setShowNewMission(true)}
					>
						Create Mission
					</button>
				</div>
			)}

			<CreateMissionModal
				open={showNewMission}
				onClose={() => setShowNewMission(false)}
			/>

			{toast && (
				<div
					className="alert alert-success"
					style={{
						position: "fixed",
						bottom: "var(--space-4)",
						right: "var(--space-4)",
						zIndex: 1000,
						animation: "fadeIn 0.3s ease",
					}}
				>
					<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
						<polyline points="20 6 9 17 4 12" />
					</svg>
					{toast}
				</div>
			)}
		</div>
	);
}
