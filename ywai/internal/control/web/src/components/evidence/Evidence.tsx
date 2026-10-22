import { useEffect, useState } from "react";
import {
	evidenceApi,
	type EvidenceRunDetail,
	type EvidenceRunSummary,
} from "../../api/client";
import "./Evidence.css";

// ponytail: no project_dir picker in the UI — the server falls back to its
// working directory. Add a picker when multi-project matters.
export default function Evidence() {
	const [runs, setRuns] = useState<EvidenceRunSummary[] | null>(null);
	const [error, setError] = useState<string | null>(null);
	const [detail, setDetail] = useState<EvidenceRunDetail | null>(null);
	const [detailError, setDetailError] = useState<string | null>(null);
	const [loading, setLoading] = useState(false);

	useEffect(() => {
		evidenceApi
			.list()
			.then(setRuns)
			.catch((err) => {
				setRuns([]);
				setError(err instanceof Error ? err.message : String(err));
			});
	}, []);

	const openRun = (runId: string) => {
		setLoading(true);
		setDetailError(null);
		evidenceApi
			.get(runId)
			.then(setDetail)
			.catch((err) => {
				setDetail(null);
				setDetailError(err instanceof Error ? err.message : String(err));
			})
			.finally(() => setLoading(false));
	};

	return (
		<div className="evidence">
			<h2>Evidence</h2>
			<p className="evidence-hint">QA run evidence from the project's .evidence/ folder.</p>
			{error && (
				<p className="evidence-error" role="alert">
					{error}
				</p>
			)}
			{runs === null ? (
				<p className="evidence-muted">Loading…</p>
			) : runs.length === 0 ? (
				<p className="evidence-empty">No evidence runs found</p>
			) : (
				<div className="evidence-layout">
					<ul className="evidence-runs">
						{runs.map((run) => (
							<li key={run.runId}>
								<button
									type="button"
									className={`evidence-run${detail?.runId === run.runId ? " is-active" : ""}`}
									onClick={() => openRun(run.runId)}
								>
									<span className="evidence-run-id">{run.runId}</span>
									<span className="evidence-run-meta">
										{run.files} {run.files === 1 ? "file" : "files"} ·{" "}
										{new Date(run.modified).toLocaleString()}
									</span>
								</button>
							</li>
						))}
					</ul>
					<section className="evidence-detail">
						{loading && <p className="evidence-muted">Loading run…</p>}
						{detailError && (
							<p className="evidence-error" role="alert">
								{detailError}
							</p>
						)}
						{!loading && !detailError && !detail && (
							<p className="evidence-muted">Pick a run to see its files and report.</p>
						)}
						{detail && (
							<>
								<h3 className="evidence-detail-title">{detail.runId}</h3>
								<ul className="evidence-files">
									{detail.files.map((f) => (
										<li key={f.name}>
											{f.name} <span className="evidence-size">({f.size} B)</span>
										</li>
									))}
								</ul>
								<pre className="evidence-report">{detail.report || "(no report.md)"}</pre>
							</>
						)}
					</section>
				</div>
			)}
		</div>
	);
}
