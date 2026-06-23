import { useEffect, useCallback, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { ChevronDown } from "lucide-react";
import { useKanbanStore } from "../../stores/kanbanStore";
import { useWebSocket } from "../../hooks/useWebSocket";
import type { Delegation, DelegationColumn, WSMessage } from "../../api/types";
import DelegationDetailModal from "./DelegationDetailModal";
import "./Kanban.css";

const COLUMNS: { id: DelegationColumn; label: string; color: string }[] = [
	{ id: "backlog", label: "Backlog", color: "var(--text-muted)" },
	{ id: "ready", label: "Ready", color: "var(--info)" },
	{ id: "in_progress", label: "In Progress", color: "var(--warning)" },
	{ id: "review", label: "Review", color: "var(--tint-purple-dot)" },
	{ id: "done", label: "Done", color: "var(--success)" },
];

const AGENT_PILL: Record<string, string> = {
	dev: "pill-info",
	qa: "pill-warning",
	architect: "pill-accent",
	reviewer: "pill-primary",
	devops: "pill-danger",
};

// Palette used to derive a stable color for agents that aren't hardcoded above.
const PILL_PALETTE = [
	"pill-info",
	"pill-warning",
	"pill-accent",
	"pill-primary",
	"pill-danger",
	"pill-success",
];

// agentPillClass returns the hardcoded pill for known agents, or a deterministic
// color derived from the agent name so every distinct name gets a stable hue.
function agentPillClass(agent: string): string {
	if (AGENT_PILL[agent]) return AGENT_PILL[agent];
	let hash = 0;
	for (let i = 0; i < agent.length; i++) {
		hash = (hash * 31 + agent.charCodeAt(i)) >>> 0;
	}
	return PILL_PALETTE[hash % PILL_PALETTE.length];
}

function DelegationCard({ delegation }: { delegation: Delegation }) {
	const { fetchActivities, resolveActivity, moveDelegation } = useKanbanStore();
	const [expanded, setExpanded] = useState(false);
	const [showDetail, setShowDetail] = useState(false);
	const [dragging, setDragging] = useState(false);
	const activities = useKanbanStore((s) => s.activities[delegation.id]);

	const handleDragStart = (e: React.DragEvent) => {
		e.dataTransfer.setData("text/plain", delegation.id);
		e.dataTransfer.effectAllowed = "move";
		setDragging(true);
	};

	const handleDragEnd = () => setDragging(false);

	const toggleExpand = () => {
		const next = !expanded;
		setExpanded(next);
		if (next && !activities) fetchActivities(delegation.id);
	};

	const pillClass = agentPillClass(delegation.agent);
	const shortId = delegation.id.split("-")[0]?.slice(0, 8) ?? delegation.id;

	return (
		<div
			className={`delegation-card${dragging ? " dragging" : ""}${expanded ? " expanded" : ""}`}
			draggable
			onDragStart={handleDragStart}
			onDragEnd={handleDragEnd}
		>
			<div className="delegation-header" onClick={toggleExpand}>
				<div className="delegation-header-top">
					<span className={`pill ${pillClass} delegation-agent-pill`}>
						<span className="dot"></span>
						{delegation.agent}
					</span>
					<ChevronDown
						className={`delegation-chevron${expanded ? " open" : ""}`}
						size={16}
					/>
				</div>
				<p className="delegation-summary">{delegation.task_summary}</p>
			</div>

			{delegation.handoff_preview && (
				<div className="delegation-handoff">
					<span className="delegation-handoff-text">
						{delegation.handoff_preview}
					</span>
				</div>
			)}

			{delegation.blocker && (
				<div className="delegation-blocker">
					<span className="delegation-blocker-text">
						Blocked: {delegation.blocker}
					</span>
				</div>
			)}

			{expanded && (
				<div className="delegation-details">
					<div className="delegation-meta">
						<span className="delegation-meta-id" title={delegation.id}>
							#{shortId}
						</span>
						<span className="delegation-meta-sep">·</span>
						<span className="delegation-meta-status">{delegation.status}</span>
					</div>

					{delegation.handoff && (
						<div className="delegation-handoff-full">
							<h4 className="activity-title">Handoff</h4>
							<p className="delegation-handoff-full-text">{delegation.handoff}</p>
						</div>
					)}

					<div className="activity-section">
						<h4 className="activity-title">Activities</h4>
						{activities && activities.length > 0 ? (
							<div className="activity-list">
								{activities.map((a) => (
									<div key={a.id} className="activity-item">
										<div className="activity-content">
											<span className={`pill ${a.type === "decision" ? "pill-warning" : "pill-muted"} activity-type-pill`}>
												{a.type}
											</span>
											<span className="activity-text">{a.content}</span>
										</div>
										{a.type === "decision" && !a.resolution && (
											<div className="activity-actions">
												<button
													className="btn btn-sm btn-primary"
													onClick={() => resolveActivity(delegation.id, a.id, "approved")}
												>
													Approve
												</button>
												<button
													className="btn btn-sm btn-danger"
													onClick={() => resolveActivity(delegation.id, a.id, "rejected")}
												>
													Reject
												</button>
											</div>
										)}
										{a.resolution && (
											<span className={`pill ${a.resolution === "approved" ? "pill-success" : "pill-danger"} activity-resolution-pill`}>
												{a.resolution}
											</span>
										)}
									</div>
								))}
							</div>
						) : (
							<span className="activity-empty">
								{activities ? "No activities yet" : "Loading…"}
							</span>
						)}
					</div>

					<div className="delegation-move-row">
						<button
							className="quick-move-btn"
							onClick={() => setShowDetail(true)}
						>
							⤢ Details
						</button>
						{COLUMNS.filter((c) => c.id !== delegation.column).map((c) => (
							<button
								key={c.id}
								className="quick-move-btn"
								onClick={() => moveDelegation(delegation.id, c.id)}
							>
								→ {c.label}
							</button>
						))}
					</div>
				</div>
			)}

			<DelegationDetailModal
				delegation={delegation}
				open={showDetail}
				onClose={() => setShowDetail(false)}
			/>
		</div>
	);
}

export default function Kanban() {
	const [dragOverColumn, setDragOverColumn] = useState<DelegationColumn | null>(null);

	const {
		board,
		activeSession,
		loading,
		fetchSessions,
		moveDelegation,
		handleWSMessage,
	} = useKanbanStore();

	const [searchParams, setSearchParams] = useSearchParams();

	const onWSMessage = useCallback(
		(msg: WSMessage) => handleWSMessage(msg),
		[handleWSMessage],
	);

	useWebSocket("/api/events", onWSMessage);

	// On first load, honor a ?session=<id> deep-link (e.g. after F5) and fall
	// back to the first active session otherwise.
	useEffect(() => {
		fetchSessions(searchParams.get("session") ?? undefined);
		// Only on mount — the URL is kept in sync by the effect below.
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [fetchSessions]);

	// Keep the URL in sync with the selected session so reloads/sharing work.
	useEffect(() => {
		if (!activeSession) return;
		if (searchParams.get("session") === activeSession.id) return;
		const next = new URLSearchParams(searchParams);
		next.set("session", activeSession.id);
		setSearchParams(next, { replace: true });
	}, [activeSession, searchParams, setSearchParams]);

	const handleDrop = (e: React.DragEvent, col: DelegationColumn) => {
		e.preventDefault();
		const id = e.dataTransfer.getData("text/plain");
		if (id) moveDelegation(id, col);
		setDragOverColumn(null);
	};

	const handleDragOver = (e: React.DragEvent, col: DelegationColumn) => {
		e.preventDefault();
		setDragOverColumn(col);
	};

	const handleDragLeave = (e: React.DragEvent, col: DelegationColumn) => {
		const related = e.relatedTarget as HTMLElement | null;
		if (!related || !e.currentTarget.contains(related)) {
			setDragOverColumn((prev) => (prev === col ? null : prev));
		}
	};

	if (loading && !board) {
		return (
			<div className="loading-inline">
				<div className="spinner"></div>
				<span>Loading kanban board…</span>
		</div>
	);
}

	return (
		<div className="kanban-page">
			<div className="kanban-main">
				<div className="page-header">
					<div className="page-title">
						<h2>{activeSession?.goal || 'Select a session'}</h2>
						{activeSession && (
							<span className="page-title-project">{activeSession.project}</span>
						)}
					</div>
				</div>

			{/* Kanban board */}
			<div className="board">
				{COLUMNS.map((col) => {
					const delegations = board?.[col.id] ?? [];
					return (
						<div
							key={col.id}
							className={`kanban-column${dragOverColumn === col.id ? " drag-over" : ""}`}
							data-column={col.id}
							onDrop={(e) => handleDrop(e, col.id)}
							onDragOver={(e) => handleDragOver(e, col.id)}
							onDragLeave={(e) => handleDragLeave(e, col.id)}
						>
							<div className="kanban-column-header">
								<h2 className="kanban-column-title">{col.label}</h2>
								<span className="kanban-column-count">
									{delegations.length}
								</span>
							</div>
							<div className="kanban-column-cards">
								{delegations.map((d) => (
									<DelegationCard key={d.id} delegation={d} />
								))}
								{delegations.length === 0 && (
									<div className="kanban-empty-col">
										<span className="muted" style={{ fontSize: "0.82rem" }}>
											No delegations
										</span>
									</div>
								)}
							</div>
						</div>
					);
				})}
			</div>

			</div>
		</div>
	);
}
