import { useState, useEffect } from 'react'
import { memoriesApi } from '../../api/client'
import type { MemoryEvalResult } from '../../api/types'
import { useMemoriesStore } from '../../stores/memoriesStore'

function metricColor(value: number, good = 0.5, mid = 0.25): string {
	if (value >= good) return 'var(--tint-success)'
	if (value >= mid) return 'var(--warning)'
	return 'var(--tint-danger)'
}

export default function MemoryRecallEval() {
	const [sampleSize, setSampleSize] = useState(100)
	const [k, setK] = useState(10)
	const [project, setProject] = useState('')
	const [running, setRunning] = useState(false)
	const [result, setResult] = useState<MemoryEvalResult | null>(null)
	const [error, setError] = useState<string | null>(null)
	const { stats, fetchStats } = useMemoriesStore()

	useEffect(() => {
		if (!stats) fetchStats()
	}, [stats, fetchStats])

	const projects = stats?.projects ?? []

	const run = async () => {
		setRunning(true)
		setError(null)
		try {
			const res = await memoriesApi.runRecallEval({
				sample_size: sampleSize,
				k,
				project: project || undefined,
			})
			setResult(res)
		} catch (err) {
			setError(String(err))
		} finally {
			setRunning(false)
		}
	}

	return (
		<div className="recall-eval">
			<div className="recall-eval-form card card-pad">
				<div className="recall-eval-form-row">
					<label className="recall-field">
						<span className="recall-field-label">Sample size</span>
						<input
							className="input"
							type="number"
							min={10}
							max={500}
							value={sampleSize}
							onChange={(e) => setSampleSize(Number(e.target.value))}
						/>
					</label>
					<label className="recall-field">
						<span className="recall-field-label">Top-K</span>
						<input
							className="input"
							type="number"
							min={1}
							max={50}
							value={k}
							onChange={(e) => setK(Number(e.target.value))}
						/>
					</label>
					<label className="recall-field">
						<span className="recall-field-label">Project (optional)</span>
						<select
							className="input"
							value={project}
							onChange={(e) => setProject(e.target.value)}
						>
							<option value="">All projects</option>
							{projects.map((p) => (
								<option key={p} value={p}>
									{p}
								</option>
							))}
						</select>
					</label>
					<button
						className="btn btn-primary"
						onClick={run}
						disabled={running}
					>
						{running ? 'Running…' : '▶ Run eval'}
					</button>
				</div>
				<p className="recall-eval-explainer muted">
					For each prompt we extract content keywords, run{' '}
					<code>mem_search</code>, and check the top-K results.{' '}
					<strong>Strict relevance</strong> = result from the same{' '}
					<code>session_id</code> as the prompt; <strong>loose relevance</strong>{' '}
					= same project. Only prompts whose session has at least one
					observation are evaluated (the rest are mathematically
					unsatisfiable). Metrics reported: hit rate, precision@K, MRR.
				</p>
			</div>

			{error && <div className="alert alert-danger">Error: {error}</div>}

			{result && (
				<>
					<div className="recall-summary muted">
						{result.evaluated} evaluated of {result.total_prompts} inspected ·{' '}
						{result.skipped} skipped (short, no session, or no obs in session) ·{' '}
						{(result.duration_ms / 1000).toFixed(1)}s
						{result.project ? ` · project: ${result.project}` : ''}
					</div>

					<h3 className="recall-section-title">Strict — same session</h3>
					<div className="kpi-grid">
						<div className="kpi">
							<div className="kpi-value tnum" style={{ color: metricColor(result.hit_rate, 0.6, 0.3) }}>
								{(result.hit_rate * 100).toFixed(0)}%
							</div>
							<div className="kpi-label">Hit rate</div>
						</div>
						<div className="kpi">
							<div className="kpi-value tnum" style={{ color: metricColor(result.precision_at_k, 0.3, 0.1) }}>
								{(result.precision_at_k * 100).toFixed(1)}%
							</div>
							<div className="kpi-label">Precision@{result.k}</div>
						</div>
						<div className="kpi">
							<div className="kpi-value tnum" style={{ color: metricColor(result.mrr, 0.5, 0.25) }}>
								{result.mrr.toFixed(3)}
							</div>
							<div className="kpi-label">MRR</div>
						</div>
						<div className="kpi">
							<div className="kpi-value tnum">{result.evaluated}</div>
							<div className="kpi-label">Prompts evaluated</div>
						</div>
					</div>

					<h3 className="recall-section-title">Loose — same project</h3>
					<div className="kpi-grid">
						<div className="kpi">
							<div className="kpi-value tnum" style={{ color: metricColor(result.project_hit_rate, 0.7, 0.4) }}>
								{(result.project_hit_rate * 100).toFixed(0)}%
							</div>
							<div className="kpi-label">Project hit rate</div>
						</div>
						<div className="kpi">
							<div className="kpi-value tnum" style={{ color: metricColor(result.project_precision_at_k, 0.4, 0.15) }}>
								{(result.project_precision_at_k * 100).toFixed(1)}%
							</div>
							<div className="kpi-label">Project precision@{result.k}</div>
						</div>
					</div>

					<section className="recall-section">
						<h3 className="recall-section-title">
							Misses{' '}
							<span className="muted small">({result.misses.length})</span>
						</h3>
						<p className="muted small">
							Prompts where no top-{result.k} result came from the same
							session. The top-1 search hit is shown for inspection.
						</p>
						{result.misses.length === 0 ? (
							<div className="empty-state">
								<div className="empty-title">No misses — perfect run</div>
							</div>
						) : (
							<div className="table-wrap">
								<table className="data-table">
									<thead>
										<tr>
											<th>Prompt</th>
											<th>Session</th>
											<th>Top-1 result</th>
										</tr>
									</thead>
									<tbody>
										{result.misses.slice(0, 50).map((m) => (
											<tr key={m.prompt_id}>
												<td className="recall-prompt-cell">
													<div className="ellipsis-2">{m.content}</div>
													<span className="muted small">#{m.prompt_id}</span>
												</td>
												<td className="cell-mono cell-muted">
													{m.session_id.slice(0, 16)}…
												</td>
												<td className="cell-muted">
													{m.top_result || '—'}
												</td>
											</tr>
										))}
									</tbody>
								</table>
							</div>
						)}
					</section>

					<section className="recall-section">
						<h3 className="recall-section-title">
							Sample{' '}
							<span className="muted small">({result.samples.length})</span>
						</h3>
						<div className="table-wrap">
							<table className="data-table">
								<thead>
									<tr>
										<th>Hit</th>
										<th>Rank</th>
										<th>Precision</th>
										<th>Prompt</th>
										<th>Session</th>
									</tr>
								</thead>
								<tbody>
									{result.samples
										.slice()
										.sort((a, b) => {
											if (a.hit !== b.hit) return a.hit ? -1 : 1
											return a.hit_rank - b.hit_rank
										})
										.slice(0, 100)
										.map((s) => (
											<tr key={s.prompt_id}>
												<td>{s.hit ? '✅' : '❌'}</td>
												<td className="tnum">
													{s.hit_rank > 0 ? s.hit_rank : '—'}
												</td>
												<td
													className="tnum"
													style={{
														color: metricColor(s.precision, 0.4, 0.1),
													}}
												>
													{(s.precision * 100).toFixed(0)}%
												</td>
												<td className="recall-prompt-cell">
													<div className="ellipsis">{s.snippet}</div>
												</td>
												<td className="cell-mono cell-muted">
													{s.session_id.slice(0, 16)}…
												</td>
											</tr>
										))}
								</tbody>
							</table>
						</div>
					</section>
				</>
			)}
		</div>
	)
}
