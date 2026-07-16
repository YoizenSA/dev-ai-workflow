/**
 * CSS fallbacks shown while loading before bones are captured.
 * Layout mirrors the real screens so first paint stays stable.
 */
import type { ReactNode } from "react";
import "./bones.css";

function BoneLine({ w = "100%", h = 12, className = "" }: { w?: string; h?: number; className?: string }) {
	return (
		<div
			className={`by-bone by-bone-line ${className}`}
			style={{ width: w, height: h }}
			aria-hidden
		/>
	);
}

function BoneBlock({
	w = "100%",
	h = 80,
	className = "",
}: {
	w?: string;
	h?: number;
	className?: string;
}) {
	return (
		<div
			className={`by-bone by-bone-block ${className}`}
			style={{ width: w, height: h }}
			aria-hidden
		/>
	);
}

export function KanbanBonesFallback() {
	return (
		<div className="by-fallback kanban-page" aria-busy="true" aria-label="Loading kanban board">
			<aside className="by-fallback-sidebar">
				<BoneLine w="70%" h={14} />
				<BoneLine w="90%" h={36} />
				<BoneLine w="85%" h={36} />
				<BoneLine w="80%" h={36} />
			</aside>
			<div className="kanban-main by-fallback-main">
				<div className="page-header">
					<div className="page-title">
						<BoneLine w="42%" h={22} />
						<BoneLine w="18%" h={12} />
					</div>
				</div>
				<div className="board by-fallback-board">
					{Array.from({ length: 5 }).map((_, col) => (
						<div key={col} className="kanban-column by-fallback-col">
							<div className="kanban-column-header">
								<BoneLine w="55%" h={12} />
								<BoneLine w="18px" h={18} className="by-bone-round" />
							</div>
							<div className="kanban-column-cards">
								{Array.from({ length: col % 2 === 0 ? 2 : 1 }).map((__, i) => (
									<div key={i} className="by-fallback-card">
										<BoneLine w="40%" h={16} />
										<BoneLine w="92%" h={12} />
										<BoneLine w="70%" h={12} />
									</div>
								))}
							</div>
						</div>
					))}
				</div>
			</div>
		</div>
	);
}

export function MissionsBonesFallback() {
	return (
		<div className="by-fallback missions" aria-busy="true" aria-label="Loading missions">
			<header className="page-header">
				<div className="page-heading">
					<BoneLine w="80px" h={10} />
					<BoneLine w="220px" h={24} />
					<BoneLine w="360px" h={12} />
				</div>
			</header>
			<div className="by-fallback-grid">
				{Array.from({ length: 4 }).map((_, i) => (
					<div key={i} className="by-fallback-card by-fallback-card-pad">
						<BoneLine w="50%" h={16} />
						<BoneLine w="90%" h={12} />
						<BoneLine w="75%" h={12} />
						<div className="by-fallback-row">
							<BoneLine w="64px" h={20} className="by-bone-round" />
							<BoneLine w="64px" h={20} className="by-bone-round" />
						</div>
					</div>
				))}
			</div>
		</div>
	);
}

export function HubBonesFallback() {
	return (
		<div className="by-fallback hub-page" aria-busy="true" aria-label="Loading projects">
			<div className="by-fallback-grid">
				{Array.from({ length: 3 }).map((_, i) => (
					<div key={i} className="by-fallback-card by-fallback-card-pad">
						<BoneLine w="45%" h={18} />
						<BoneLine w="80%" h={12} />
						<BoneLine w="60%" h={12} />
					</div>
				))}
			</div>
			<BoneBlock w="140px" h={36} className="by-bone-btn" />
		</div>
	);
}

export function HealthBonesFallback() {
	return (
		<div className="by-fallback health-dashboard" aria-busy="true" aria-label="Loading health status">
			<div className="by-fallback-card by-fallback-card-pad">
				<BoneLine w="30%" h={22} />
				<BoneLine w="45%" h={12} />
			</div>
			<div className="by-fallback-row by-fallback-health-cards">
				<div className="by-fallback-card by-fallback-card-pad by-fallback-flex">
					<BoneLine w="70%" h={14} />
					<BoneLine w="24px" h={24} className="by-bone-round" />
				</div>
				<div className="by-fallback-card by-fallback-card-pad by-fallback-flex">
					<BoneLine w="70%" h={14} />
					<BoneLine w="24px" h={24} className="by-bone-round" />
				</div>
			</div>
		</div>
	);
}

/** Fixture content for boneyard CLI/Vite capture (mirrors real layout). */
export function KanbanCaptureFixture(): ReactNode {
	return (
		<div className="kanban-page">
			<aside className="session-sidebar" style={{ width: 220, padding: 16 }}>
				<p>Sessions</p>
				<div className="delegation-card" style={{ marginBottom: 8, padding: 12 }}>
					<span className="pill">active</span>
					<p className="delegation-summary">Ship feature X to production</p>
				</div>
				<div className="delegation-card" style={{ padding: 12 }}>
					<span className="pill">idle</span>
					<p className="delegation-summary">Investigate flaky tests</p>
				</div>
			</aside>
			<div className="kanban-main" style={{ flex: 1, padding: 16 }}>
				<div className="page-header">
					<div className="page-title">
						<h2>Ship feature X to production</h2>
						<span className="page-title-project">demo-project</span>
					</div>
				</div>
				<div className="board">
					{["Backlog", "Todo", "Doing", "Review", "Done"].map((label) => (
						<div key={label} className="kanban-column">
							<div className="kanban-column-header">
								<h2 className="kanban-column-title">{label}</h2>
								<span className="kanban-column-count">1</span>
							</div>
							<div className="kanban-column-cards">
								<div className="delegation-card" style={{ padding: 12 }}>
									<span className="pill">dev</span>
									<p className="delegation-summary">Implement vertical slice</p>
								</div>
							</div>
						</div>
					))}
				</div>
			</div>
		</div>
	);
}

export function MissionsCaptureFixture(): ReactNode {
	return (
		<div className="missions" style={{ padding: 16 }}>
			<header className="page-header">
				<div className="page-heading">
					<span className="page-eyebrow">Missions</span>
					<h1 className="page-title">Mission Control</h1>
					<p className="page-subtitle">2 active · 1 completed · 3 projects</p>
				</div>
			</header>
			<div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
				{[1, 2, 3, 4].map((n) => (
					<div key={n} className="card card-pad" style={{ padding: 16 }}>
						<h3>Mission {n}</h3>
						<p>Deliver slice {n} with tests and docs</p>
						<span className="pill">active</span>
					</div>
				))}
			</div>
		</div>
	);
}

export function HubCaptureFixture(): ReactNode {
	return (
		<div className="hub-page" style={{ padding: 16 }}>
			<div className="hub-page__list" style={{ display: "grid", gap: 12 }}>
				{[1, 2, 3].map((n) => (
					<div key={n} className="card card-pad" style={{ padding: 16 }}>
						<h3>Project {n}</h3>
						<p>/home/user/projects/app-{n}</p>
						<span className="pill">opencode</span>
					</div>
				))}
			</div>
			<button type="button" className="btn btn-primary">
				Add Project
			</button>
		</div>
	);
}

export function HealthCaptureFixture(): ReactNode {
	return (
		<div className="health-dashboard" style={{ padding: 16 }}>
			<div className="health-summary healthy">
				<h2>Healthy</h2>
				<p className="health-subtitle">Last check: January 2026</p>
			</div>
			<div className="health-cards" style={{ display: "flex", gap: 12 }}>
				<div className="card card-pad health-status-card">
					<span>Daemon</span>
					<span>✓</span>
				</div>
				<div className="card card-pad health-status-card">
					<span>Database</span>
					<span>✓</span>
				</div>
			</div>
			<div className="health-meta">
				<span>3 repos</span>
			</div>
		</div>
	);
}
