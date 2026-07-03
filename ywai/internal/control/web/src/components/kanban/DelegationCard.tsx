import { useState, useRef } from "react";
import { ChevronDown } from "lucide-react";
import { useKanbanStore } from "../../stores/kanbanStore";
import type { Delegation, DelegationColumn } from "../../api/types";
import DelegationDetailModal from "./DelegationDetailModal";
import HandoffModal from "./HandoffModal";

export function DelegationCard({ delegation }: { delegation: Delegation }) {
	const { fetchActivities, resolveActivity, moveDelegation } =
		useKanbanStore();
	const [expanded, setExpanded] = useState(false);
	const [showDetail, setShowDetail] = useState(false);
	const [showHandoff, setShowHandoff] = useState(false);
	const [dragging, setDragging] = useState(false);
	const handoffTriggerRef = useRef<HTMLButtonElement>(null);
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
					<button
						type="button"
						className="delegation-expand-btn"
						onClick={(e) => {
							e.stopPropagation();
							setShowDetail(true);
						}}
						aria-label="View details"
						title="Open detail view"
					>
						⤢
					</button>
					<ChevronDown
						className={`delegation-chevron${expanded ? " open" : ""}`}
						size={16}
					/>
				</div>
				<p className="delegation-summary">{delegation.task_summary}</p>
			</div>

			{delegation.handoff_preview && (
				<button
					type="button"
					ref={handoffTriggerRef}
					className="delegation-handoff"
					onClick={(e) => {
						e.stopPropagation();
						setShowHandoff(true);
					}}
					aria-label="View full handoff"
					title="Click to view the full handoff"
				>
					<span className="delegation-handoff-text">
						{delegation.handoff_preview}
					</span>
				</button>
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
							<p className="delegation-handoff-full-text">
								{delegation.handoff}
							</p>
						</div>
					)}

					<div className="activity-section">
						<h4 className="activity-title">Activity</h4>
						{activities && activities.length > 0 ? (
							<div className="activity-list">
								{activities.map((a) => (
									<div key={a.id} className="activity-item">
										<div className="activity-header">
											<span
												className={`pill pill-${a.type} activity-type-pill`}
											>
												{a.type}
											</span>
											<span className="activity-text">{a.content}</span>
										</div>
										{a.type === "decision" && !a.resolution && (
											<div className="activity-actions">
												<button
													className="btn btn-sm btn-primary"
													onClick={() =>
														resolveActivity(delegation.id, a.id, "approved")
													}
												>
													Approve
												</button>
												<button
													className="btn btn-sm btn-danger"
													onClick={() =>
														resolveActivity(delegation.id, a.id, "rejected")
													}
												>
													Reject
												</button>
											</div>
										)}
										{a.resolution && (
											<span
												className={`pill ${a.resolution === "approved" ? "pill-success" : "pill-danger"} activity-resolution-pill`}
											>
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

			<HandoffModal
				open={showHandoff}
				onClose={() => setShowHandoff(false)}
				triggerRef={handoffTriggerRef}
				title={delegation.task_summary}
				subtitle={delegation.agent}
				handoff={delegation.handoff || delegation.handoff_preview || ""}
			/>
		</div>
	);
}

// ─── Shared helpers (also used by Kanban.tsx) ──────────────────────────

const COLUMNS: { id: DelegationColumn; label: string; color: string }[] = [
	{ id: "backlog", label: "Backlog", color: "var(--text-muted)" },
	{ id: "ready", label: "Ready", color: "var(--info)" },
	{ id: "in_progress", label: "In Progress", color: "var(--warning)" },
	{ id: "review", label: "Review", color: "var(--tint-purple-dot)" },
	{ id: "done", label: "Done", color: "var(--success)" },
];

export { COLUMNS };

const AGENT_PILL: Record<string, string> = {
	dev: "pill-info",
	qa: "pill-warning",
	architect: "pill-accent",
	reviewer: "pill-primary",
	devops: "pill-danger",
};

const PILL_PALETTE = [
	"pill-info",
	"pill-warning",
	"pill-accent",
	"pill-primary",
	"pill-danger",
	"pill-success",
];

function agentPillClass(agent: string): string {
	if (AGENT_PILL[agent]) return AGENT_PILL[agent];
	let hash = 0;
	for (let i = 0; i < agent.length; i++) {
		hash = (hash * 31 + agent.charCodeAt(i)) >>> 0;
	}
	return PILL_PALETTE[hash % PILL_PALETTE.length];
}

export { agentPillClass };
