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
