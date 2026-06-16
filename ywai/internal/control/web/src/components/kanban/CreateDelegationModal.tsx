import { useState } from "react";
import { useKanbanStore } from "../../stores/kanbanStore";
import Modal from "../shared/Modal";

interface Props {
	open: boolean;
	onClose: () => void;
}

const AGENTS = [
	{ id: "dev", label: "Developer", pill: "pill-info" },
	{ id: "qa", label: "QA Engineer", pill: "pill-warning" },
	{ id: "reviewer", label: "Reviewer", pill: "pill-primary" },
	{ id: "architect", label: "Architect", pill: "pill-accent" },
	{ id: "devops", label: "DevOps", pill: "pill-danger" },
];

export default function CreateDelegationModal({ open, onClose }: Props) {
	const [agent, setAgent] = useState("dev");
	const [taskSummary, setTaskSummary] = useState("");
	const [submitting, setSubmitting] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const createDelegation = useKanbanStore((s) => s.createDelegation);

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault();
		if (!taskSummary.trim()) return;

		setSubmitting(true);
		setError(null);
		try {
			await createDelegation(agent, taskSummary.trim());
			onClose();
			setTaskSummary("");
		} catch (err) {
			setError(String(err));
		} finally {
			setSubmitting(false);
		}
	};

	const footer = (
		<>
			<button type="button" className="btn btn-ghost" onClick={onClose}>
				Cancel
			</button>
			<button
				type="submit"
				form="delegation-form"
				className="btn btn-primary"
				disabled={submitting || !taskSummary.trim()}
			>
				{submitting ? (
					<>
						<div className="spinner"></div>
						Creating…
					</>
				) : (
					"Create Delegation"
				)}
			</button>
		</>
	);

	return (
		<Modal
			open={open}
			onClose={onClose}
			title="New Delegation"
			subtitle="Assign a task to an AI agent"
			footer={footer}
		>
			<form id="delegation-form" onSubmit={handleSubmit}>
				<div className="form-grid">
					<div className="field span-2">
						<span className="field-label">Agent</span>
						<div className="row wrap" style={{ gap: "var(--space-2)" }}>
							{AGENTS.map((a) => (
								<button
									key={a.id}
									type="button"
									className={`pill ${agent === a.id ? a.pill : "pill-muted"}`}
									style={{
										cursor: "pointer",
										padding: "0.35rem 0.75rem",
										fontSize: "0.8rem",
									}}
									onClick={() => setAgent(a.id)}
								>
									<span className="dot"></span>
									{a.label}
								</button>
							))}
						</div>
					</div>

					<div className="field span-2">
						<label className="field-label" htmlFor="task">
							Task Summary
						</label>
						<textarea
							id="task"
							className="textarea"
							rows={4}
							value={taskSummary}
							onChange={(e) => setTaskSummary(e.target.value)}
							placeholder="Describe what this agent should do…"
							required
						/>
						<span className="field-help">
							Be specific about the goal and constraints
						</span>
					</div>
				</div>

				{error && (
					<div
						className="alert alert-danger"
						style={{ marginTop: "var(--space-3)" }}
					>
						<svg
							width="18"
							height="18"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							strokeWidth="2"
							strokeLinecap="round"
							strokeLinejoin="round"
						>
							<circle cx="12" cy="12" r="10" />
							<line x1="12" y1="8" x2="12" y2="12" />
							<line x1="12" y1="16" x2="12.01" y2="16" />
						</svg>
						{error}
					</div>
				)}
			</form>
		</Modal>
	);
}
