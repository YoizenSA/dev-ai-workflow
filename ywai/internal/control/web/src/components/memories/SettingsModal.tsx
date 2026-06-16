import { useState, useRef } from 'react'
import Modal from '../shared/Modal'
import { useMemoriesStore } from '../../stores/memoriesStore'

interface Props {
	open: boolean
	onClose: () => void
}

function formatBytes(n: number): string {
	if (n < 1024) return `${n} B`
	if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
	return `${(n / (1024 * 1024)).toFixed(1)} MB`
}

export default function SettingsModal({ open, onClose }: Props) {
	const { stats, exportAll, importData, mergeProjects } = useMemoriesStore()
	const [exporting, setExporting] = useState(false)
	const [importing, setImporting] = useState(false)
	const [file, setFile] = useState<File | null>(null)
	const [importResult, setImportResult] = useState<string | null>(null)
	const [importError, setImportError] = useState<string | null>(null)
	const [source, setSource] = useState('')
	const [target, setTarget] = useState('')
	const [merging, setMerging] = useState(false)
	const [mergeResult, setMergeResult] = useState<string | null>(null)
	const fileInputRef = useRef<HTMLInputElement>(null)

	const projects = stats?.projects ?? []

	const handleExport = async () => {
		setExporting(true)
		try {
			const blob = await exportAll()
			const url = URL.createObjectURL(blob)
			const a = document.createElement('a')
			a.href = url
			a.download = `engram-export-${new Date().toISOString().slice(0, 10)}.json`
			document.body.appendChild(a)
			a.click()
			document.body.removeChild(a)
			URL.revokeObjectURL(url)
		} catch (err) {
			alert(`Export failed: ${err}`)
		} finally {
			setExporting(false)
		}
	}

	const handleImport = async () => {
		if (!file) return
		setImporting(true)
		setImportResult(null)
		setImportError(null)
		try {
			const result = await importData(file)
			setImportResult(
				`Imported ${result.sessions_imported} sessions, ${result.observations_imported} observations, ${result.prompts_imported} prompts.`,
			)
			setFile(null)
			if (fileInputRef.current) fileInputRef.current.value = ''
		} catch (err) {
			setImportError(String(err))
		} finally {
			setImporting(false)
		}
	}

	const handleMerge = async () => {
		if (!source || !target || source === target) return
		if (!confirm(`Move all observations from "${source}" to "${target}"?`))
			return
		setMerging(true)
		setMergeResult(null)
		try {
			const result = await mergeProjects(source, target)
			setMergeResult(`Moved ${result.observations_updated} observations.`)
			setSource('')
			setTarget('')
		} catch (err) {
			setMergeResult(`Error: ${err}`)
		} finally {
			setMerging(false)
		}
	}

	return (
		<Modal
			open={open}
			onClose={onClose}
			title="Memory settings"
			subtitle="Export, import and merge engram data"
			width="640px"
		>
			<section className="settings-section">
				<h3 className="settings-section-title">Export</h3>
				<p className="settings-section-desc">
					Download a JSON snapshot of every session, observation and prompt.
					Useful for backups or moving between machines.
				</p>
				<div className="row">
					<button
						className="btn btn-primary"
						onClick={handleExport}
						disabled={exporting}
					>
						{exporting ? 'Exporting…' : 'Download export'}
					</button>
				</div>
			</section>

			<section className="settings-section">
				<h3 className="settings-section-title">Import</h3>
				<p className="settings-section-desc">
					Restore data from a previous export. Existing records are preserved;
					duplicates are merged by engram.
				</p>
				<input
					ref={fileInputRef}
					type="file"
					accept="application/json,.json"
					onChange={(e) => setFile(e.target.files?.[0] ?? null)}
				/>
				{file && (
					<p className="muted settings-file-info">
						{file.name} — {formatBytes(file.size)}
					</p>
				)}
				<div className="row">
					<button
						className="btn btn-primary"
						onClick={handleImport}
						disabled={!file || importing}
					>
						{importing ? 'Importing…' : 'Run import'}
					</button>
				</div>
				{importResult && (
					<div className="alert alert-success">{importResult}</div>
				)}
				{importError && (
					<div className="alert alert-danger">{importError}</div>
				)}
			</section>

			<section className="settings-section">
				<h3 className="settings-section-title">Merge projects</h3>
				<p className="settings-section-desc">
					Re-tag every observation under a source project so it lives under a
					target project. Source project is emptied after the merge.
				</p>
				<div className="settings-merge-row">
					<select
						className="input"
						value={source}
						onChange={(e) => setSource(e.target.value)}
					>
						<option value="">— Source —</option>
						{projects.map((p) => (
							<option key={p} value={p}>
								{p}
							</option>
						))}
					</select>
					<span className="muted">→</span>
					<select
						className="input"
						value={target}
						onChange={(e) => setTarget(e.target.value)}
					>
						<option value="">— Target —</option>
						{projects
							.filter((p) => p !== source)
							.map((p) => (
								<option key={p} value={p}>
									{p}
								</option>
							))}
					</select>
					<button
						className="btn btn-danger"
						onClick={handleMerge}
						disabled={!source || !target || source === target || merging}
					>
						{merging ? 'Merging…' : 'Merge'}
					</button>
				</div>
				{mergeResult && <div className="alert alert-info">{mergeResult}</div>}
			</section>
		</Modal>
	)
}
