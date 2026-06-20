import { useEffect } from "react";
import { useKanbanStore } from "../../stores/kanbanStore";
import type { Delegation } from "../../api/types";
import Modal from "../shared/Modal";

interface Props {
	delegation: Delegation;
	open: boolean;
	onClose: () => void;
}

// DelegationDetailModal shows the full task detail in a roomy view: the complete
// handoff (e.g. an architect's plan) plus the activity timeline. Useful when the
// inline card is too cramped for a long plan.
export default function DelegationDetailModal({
	delegation,
	open,
	onClose,
}: Props) {
	const fetchActivities = useKanbanStore((s) => s.fetchActivities);
	const resolveActivity = useKanbanStore((s) => s.resolveActivity);
	const activities = useKanbanStore((s) => s.activities[delegation.id]);

	useEffect(() => {
		if (open && !activities) fetchActivities(delegation.id);
	}, [open, activities, delegation.id, fetchActivities]);

	return (
		<Modal
			open={open}
			onClose={onClose}
			title={delegation.task_summary}
			subtitle={`${delegation.agent} · ${delegation.status}`}
			width="720px"
		>
			{delegation.handoff ? (
				<div className="detail-section">
					<h4 className="activity-title">Handoff / Plan</h4>
					<p className="delegation-handoff-full-text">{delegation.handoff}</p>
				</div>
			) : delegation.handoff_preview ? (
				<div className="detail-section">
					<h4 className="activity-title">Handoff</h4>
					<p className="delegation-handoff-full-text">
						{delegation.handoff_preview}
					</p>
				</div>
			) : null}

			{delegation.blocker && (
				<div className="detail-section">
					<div className="delegation-blocker">
						<span className="delegation-blocker-text">
							Blocked: {delegation.blocker}
						</span>
					</div>
				</div>
			)}

			<div className="detail-section">
				<h4 className="activity-title">Timeline</h4>
				{activities && activities.length > 0 ? (
					<div className="activity-list">
						{activities.map((a) => (
							<div key={a.id} className="activity-item">
								<div className="activity-content">
									<span
										className={`pill ${
											a.type === "decision" ? "pill-warning" : "pill-muted"
										} activity-type-pill`}
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
										className={`pill ${
											a.resolution === "approved" ? "pill-success" : "pill-danger"
										} activity-resolution-pill`}
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
		</Modal>
	);
}
