import { useState, useEffect } from 'react'
import { Plug } from 'lucide-react'
import Modal from '../shared/Modal'
import { workflowApi } from '../../api/client'

type Preview = { toSync: string[]; existing: string[]; missing: string[] }
type Result = { added: string[]; skipped: string[]; errors?: string[] }

// McpSyncModal previews and applies an opencode → claude-code MCP sync for the
// servers referenced by the workflow's MCP nodes. The preview lists servers to
// add, already-present (skipped) servers, and servers referenced but missing
// from opencode.json. Append-only: existing entries are never overwritten.
export default function McpSyncModal({
	open,
	workflowName,
	onClose,
}: {
	open: boolean
	workflowName: string
	onClose: () => void
}) {
	const [preview, setPreview] = useState<Preview | null>(null)
	const [result, setResult] = useState<Result | null>(null)
	const [loading, setLoading] = useState(false)
	const [error, setError] = useState('')

	useEffect(() => {
		if (!open || !workflowName) return
		setPreview(null)
		setResult(null)
		setError('')
		setLoading(true)
		workflowApi
			.mcpSyncPreview(workflowName)
			.then((p) => setPreview(p))
			.catch((e) => setError(e instanceof Error ? e.message : String(e)))
			.finally(() => setLoading(false))
	}, [open, workflowName])

	const apply = async () => {
		setLoading(true)
		setError('')
		try {
			const res = await workflowApi.mcpSyncApply(workflowName)
			setResult(res)
		} catch (e) {
			setError(e instanceof Error ? e.message : String(e))
		} finally {
			setLoading(false)
		}
	}

	const nothingToDo = Boolean(
		preview && preview.toSync.length === 0 && preview.missing.length === 0,
	)

	return (
		<Modal open={open} onClose={onClose} title="Sync MCP servers → Claude Code" width="540px">
			<div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
				<p style={{ fontSize: 13, color: 'var(--text-muted)', margin: 0 }}>
					Replicates the MCP servers referenced by this workflow's nodes from{' '}
					<code>opencode.json</code> into <code>~/.claude.json</code> (append-only).
				</p>

				{loading && !preview && <div className="empty">Reading configs…</div>}
				{error && <div className="wf-run-stderr">{error}</div>}

				{preview && !result && (
					<>
						<SyncGroup title="Will be added" servers={preview.toSync} tone="add" />
						<SyncGroup title="Already present (skipped)" servers={preview.existing} tone="skip" />
						<SyncGroup
							title="Referenced but not in opencode.json"
							servers={preview.missing}
							tone="missing"
						/>
						{nothingToDo && (
							<div className="empty">Nothing to sync — all servers are already present.</div>
						)}
					</>
				)}

				{result && (
					<>
						<SyncGroup title="Added" servers={result.added} tone="add" />
						<SyncGroup title="Skipped" servers={result.skipped} tone="skip" />
						{result.errors && result.errors.length > 0 && (
							<SyncGroup title="Errors" servers={result.errors} tone="missing" />
						)}
						{result.added.length === 0 && result.errors?.length === 0 && (
							<div className="empty">No changes — servers were already present.</div>
						)}
					</>
				)}
			</div>

			<div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 12 }}>
				<button className="btn" onClick={onClose}>{result ? 'Close' : 'Cancel'}</button>
				{!result && (
					<button
						className="btn btn-primary"
						onClick={apply}
						disabled={loading || !preview || nothingToDo}
					>
						<Plug size={14} /> {loading ? 'Syncing…' : 'Sync'}
					</button>
				)}
			</div>
		</Modal>
	)
}

function SyncGroup({
	title,
	servers,
	tone,
}: {
	title: string
	servers: string[]
	tone: 'add' | 'skip' | 'missing'
}) {
	if (servers.length === 0) return null
	return (
		<div className="field">
			<label className="field-label">
				{title} <span className="wf-sync-count">({servers.length})</span>
			</label>
			<ul className={`wf-sync-list wf-sync-${tone}`}>
				{servers.map((s) => (
					<li key={s} className="mono">{s}</li>
				))}
			</ul>
		</div>
	)
}
