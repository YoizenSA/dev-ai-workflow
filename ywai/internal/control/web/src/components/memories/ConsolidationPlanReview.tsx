import { useState } from 'react'
import { useMemoriesStore } from '../../stores/memoriesStore'
import type { ConsolidationPlan } from '../../api/types'

interface Props {
	plan: ConsolidationPlan
	onDone: () => void
}

export default function ConsolidationPlanReview({ plan, onDone }: Props) {
	const apply = useMemoriesStore((s) => s.applyConsolidation)
	const discard = useMemoriesStore((s) => s.discardConsolidation)
	const run = useMemoriesStore((s) => s.consolidation)

	// Default: all checked.
	const [uSel, setUSel] = useState<Record<number, boolean>>(
		Object.fromEntries((plan.updates ?? []).map((_, i) => [i, true])),
	)
	const [dSel, setDSel] = useState<Record<number, boolean>>(
		Object.fromEntries((plan.deletes ?? []).map((_, i) => [i, true])),
	)
	const [sSel, setSSel] = useState<Record<number, boolean>>(
		Object.fromEntries((plan.new_summaries ?? []).map((_, i) => [i, true])),
	)

	const count =
		Object.values(uSel).filter(Boolean).length +
		Object.values(dSel).filter(Boolean).length +
		Object.values(sSel).filter(Boolean).length

	const doApply = async () => {
		if (!run) return
		await apply(run.id, {
			updates: (plan.updates ?? []).filter((_, i) => uSel[i]),
			deletes: (plan.deletes ?? []).filter((_, i) => dSel[i]),
			new_summaries: (plan.new_summaries ?? []).filter((_, i) => sSel[i]),
		})
		onDone()
	}

	const doDiscard = async () => {
		if (!run) return
		await discard(run.id)
		onDone()
	}

	return (
		<div className="consolidation-review">
			{plan.digest && (
				<div className="alert alert-info">
					<strong>Digest:</strong> {plan.digest}
				</div>
			)}

			{(plan.updates?.length ?? 0) > 0 && (
				<Section title={`Update (${plan.updates!.length})`}>
					{plan.updates!.map((u, i) => (
						<label key={i} className="review-item">
							<input
								type="checkbox"
								checked={!!uSel[i]}
								onChange={(e) =>
									setUSel({ ...uSel, [i]: e.target.checked })
								}
							/>
							<div>
								<div className="muted">
									{u.observation_id} — {u.reason}
								</div>
								{u.new_content && <div>{u.new_content}</div>}
							</div>
						</label>
					))}
				</Section>
			)}

			{(plan.deletes?.length ?? 0) > 0 && (
				<Section title={`Delete (${plan.deletes!.length})`}>
					{plan.deletes!.map((d, i) => (
						<label key={i} className="review-item">
							<input
								type="checkbox"
								checked={!!dSel[i]}
								onChange={(e) =>
									setDSel({ ...dSel, [i]: e.target.checked })
								}
							/>
							<div>
								<div className="muted">{d.observation_id}</div>
								<div>{d.reason}</div>
							</div>
						</label>
					))}
				</Section>
			)}

			{(plan.new_summaries?.length ?? 0) > 0 && (
				<Section title={`New Summaries (${plan.new_summaries!.length})`}>
					{plan.new_summaries!.map((s, i) => (
						<label key={i} className="review-item">
							<input
								type="checkbox"
								checked={!!sSel[i]}
								onChange={(e) =>
									setSSel({ ...sSel, [i]: e.target.checked })
								}
							/>
							<div>
								<span className="pill pill-accent">{s.type}</span>{' '}
								{s.content}
							</div>
						</label>
					))}
				</Section>
			)}

			<div
				className="row"
				style={{
					justifyContent: 'flex-end',
					gap: 'var(--space-2)',
					marginTop: 'var(--space-4)',
				}}
			>
				<button className="btn btn-danger" onClick={doDiscard}>
					Discard
				</button>
				<button className="btn btn-primary" onClick={doApply}>
					Apply {count} changes
				</button>
			</div>
		</div>
	)
}

function Section({
	title,
	children,
}: {
	title: string
	children: React.ReactNode
}) {
	return (
		<div className="review-section">
			<div className="section-title">{title}</div>
			{children}
		</div>
	)
}
