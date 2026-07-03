import { useEffect, useCallback, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { useKanbanStore } from "../../stores/kanbanStore";
import { useWebSocket } from "../../hooks/useWebSocket";
import type { DelegationColumn, WSMessage } from "../../api/types";
import { DelegationCard, COLUMNS } from "./DelegationCard";
import "./Kanban.css";

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
