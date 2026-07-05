import { X, ArrowUp, Bot, Loader, CheckCircle, Clock, GitBranch } from "lucide-react";
import type { Session } from "./types";

interface SubagentsPanelProps {
	open: boolean;
	onClose: () => void;
	// The session currently in view — used to highlight it among siblings.
	activeSessionId: string | null;
	// Parent of the active session (null if it's a root).
	parent: Session | null;
	// Other children of the same parent (the active session is excluded from
	// this list; it's shown separately as the active item).
	siblings: Session[];
	// Children of the active session (its subagents).
	children: Session[];
	onSelectSession: (id: string) => void;
}

function formatRel(ms?: number): string {
	if (!ms) return "";
	const d = new Date(ms);
	const now = new Date();
	const sameDay = d.toDateString() === now.toDateString();
	return sameDay
		? d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
		: d.toLocaleDateString([], { day: "2-digit", month: "short" });
}

function SessionCard({
	s,
	active,
	onClick,
}: {
	s: Session;
	active?: boolean;
	onClick: () => void;
}) {
	const running = s.time?.completed == null && !!s.time?.created;
	return (
		<button
			className={`subagent-card ${running ? "running" : "done"} ${active ? "current" : ""}`}
			onClick={onClick}
			disabled={active}
			data-tip={active ? "Current session" : s.title}
		>
			<div className="subagent-card-head">
				<span className="subagent-card-icon">
					{active ? (
						<GithubBranchLike />
					) : running ? (
						<Loader size={16} className="spin" />
					) : (
						<CheckCircle size={16} />
					)}
				</span>
				<span className="subagent-card-title">
					{s.title || s.id.slice(0, 8)}
				</span>
			</div>
			<div className="subagent-card-meta">
				<span className="subagent-card-status">
					{active ? "current" : running ? "running" : "done"}
				</span>
				{(s.time?.created || s.time?.completed) && (
					<span className="subagent-card-time">
						<Clock size={12} />
						{formatRel(s.time?.completed ?? s.time?.created)}
					</span>
				)}
			</div>
		</button>
	);
}

// Small helper to avoid re-typing the branch icon markup for the active card.
function GithubBranchLike() {
	return <GitBranch size={16} />;
}

export default function SubagentsPanel({
	open,
	onClose,
	activeSessionId,
	parent,
	siblings,
	children,
	onSelectSession,
}: SubagentsPanelProps) {
	if (!open) return null;

	const hasFamily = !!parent || siblings.length > 0;

	return (
		<aside className="subagents-panel" aria-label="Related sessions">
			<header className="subagents-panel-header">
				<span className="subagents-panel-title">
					<Bot size={16} /> Sessions
				</span>
				<button
					className="subagents-panel-close"
					onClick={onClose}
					aria-label="Close sessions panel"
					data-tip="Close"
				>
					<X size={18} />
				</button>
			</header>

			<div className="subagents-panel-list">
				{/* Parent — go up */}
				{parent && (
					<div className="subagents-section">
						<div className="subagents-section-label">
							<ArrowUp size={12} /> Parent
						</div>
						<SessionCard
							s={parent}
							onClick={() => onSelectSession(parent.id)}
						/>
					</div>
				)}

				{/* Siblings — other subagents of the same parent */}
				{hasFamily && siblings.length > 0 && (
					<div className="subagents-section">
						<div className="subagents-section-label">
							<GitBranch size={12} /> Siblings
						</div>
						{siblings.map((sib) => (
							<SessionCard
								key={sib.id}
								s={sib}
								active={sib.id === activeSessionId}
								onClick={() => onSelectSession(sib.id)}
							/>
						))}
					</div>
				)}

				{/* Children — subagents of the current session */}
				<div className="subagents-section">
					<div className="subagents-section-label">
						<Bot size={12} /> Subagents
						{children.length > 0 && (
							<span className="subagents-section-count">{children.length}</span>
						)}
					</div>
					{children.length === 0 ? (
						<div className="subagents-panel-empty">No active subagents.</div>
					) : (
						children.map((c) => (
							<SessionCard
								key={c.id}
								s={c}
								active={c.id === activeSessionId}
								onClick={() => onSelectSession(c.id)}
							/>
						))
					)}
				</div>
			</div>
		</aside>
	);
}
