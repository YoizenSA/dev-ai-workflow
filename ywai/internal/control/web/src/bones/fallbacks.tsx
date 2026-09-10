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
