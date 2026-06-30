import { useState, useEffect, useMemo } from 'react'
import { Settings2 } from 'lucide-react'
import Modal from '../shared/Modal'
import YdSelect from '../shared/YdSelect'
import { useOpencodeModels } from './NodeDetail'
import type { WorkflowSlashCommandOptions } from '../../api/types'

// SlashCommandOptionsModal edits the exported slash command's frontmatter
// (allowed-tools, model, context, argument-hint, disable-model-invocation).
// Hooks are intentionally omitted from the UI for now (the backend already
// renders them when present).
export default function SlashCommandOptionsModal({
	open,
	options,
	onClose,
	onSave,
}: {
	open: boolean
	options: WorkflowSlashCommandOptions | undefined
	onClose: () => void
	onSave: (opts: WorkflowSlashCommandOptions | null) => void
}) {
	const [allowedTools, setAllowedTools] = useState('')
	const [model, setModel] = useState('default')
	const [context, setContext] = useState('default')
	const [argumentHint, setArgumentHint] = useState('')
	const [disableModelInvocation, setDisableModelInvocation] = useState(false)

	// Real models from opencode, plus the `default` (unset) and `inherit`
	// special values. Mirrors how the Run and subAgent model selectors work.
	const opencodeModels = useOpencodeModels()
	const modelOptions = useMemo(() => {
		const opts = [
			{ value: 'default', label: 'default (unset)' },
			{ value: 'inherit', label: 'inherit' },
		]
		for (const m of opencodeModels) {
			opts.push({ value: m.id, label: `${m.provider}/${m.name}` })
		}
		// Fall back to the classic aliases when the model list is empty (still
		// loading or opencode unavailable), so the field is always usable.
		if (opencodeModels.length === 0) {
			opts.push(
				{ value: 'sonnet', label: 'sonnet' },
				{ value: 'opus', label: 'opus' },
				{ value: 'haiku', label: 'haiku' },
			)
		}
		// Ensure the currently-saved value is always selectable even if it isn't
		// in the live list (e.g. a custom provider/id).
		if (model && model !== 'default' && model !== 'inherit' && !opts.some((o) => o.value === model)) {
			opts.push({ value: model, label: model })
		}
		return opts
	}, [opencodeModels, model])

	// Load the workflow's current options whenever the modal opens.
	useEffect(() => {
		if (!open) return
		setAllowedTools(options?.allowedTools ?? '')
		setModel(options?.model ?? 'default')
		setContext(options?.context ?? 'default')
		setArgumentHint(options?.argumentHint ?? '')
		setDisableModelInvocation(options?.disableModelInvocation ?? false)
	}, [open, options])

	const handleSave = () => {
		// If nothing is set, clear the options entirely (no frontmatter emitted).
		const hasAny =
			allowedTools.trim() ||
			(model !== 'default' && model) ||
			(context !== 'default' && context) ||
			argumentHint.trim() ||
			disableModelInvocation
		if (!hasAny) {
			onSave(null)
			return
		}
		onSave({
			allowedTools: allowedTools.trim() || undefined,
			model: model === 'default' ? undefined : (model as WorkflowSlashCommandOptions['model']),
			context: context === 'default' ? undefined : (context as WorkflowSlashCommandOptions['context']),
			argumentHint: argumentHint.trim() || undefined,
			disableModelInvocation: disableModelInvocation || undefined,
		})
	}

	return (
		<Modal open={open} onClose={onClose} title="Slash command options" width="520px">
			<div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
				<p style={{ fontSize: 13, color: 'var(--text-muted)', margin: 0 }}>
					These control the exported <code>/&lt;workflow&gt;</code> command's frontmatter.
					Leave fields empty to omit them from the export.
				</p>
				<div className="field">
					<label className="field-label">Allowed tools</label>
					<input
						className="input mono"
						value={allowedTools}
						onChange={(e) => setAllowedTools(e.target.value)}
						placeholder="comma-separated, e.g. read,webfetch,bash"
					/>
					<span className="field-help">Restricts which tools the command may use.</span>
				</div>
				<div className="row">
					<div className="field">
						<label className="field-label">Model</label>
						<YdSelect
							options={modelOptions}
							value={model}
							onChange={setModel}
							ariaLabel="Model"
						/>
					</div>
					<div className="field">
						<label className="field-label">Context</label>
						<YdSelect
							options={[
								{ value: 'default', label: 'default' },
								{ value: 'fork', label: 'fork (new session)' },
							]}
							value={context}
							onChange={setContext}
							ariaLabel="Context"
						/>
					</div>
				</div>
				<div className="field">
					<label className="field-label">Argument hint</label>
					<input
						className="input"
						value={argumentHint}
						onChange={(e) => setArgumentHint(e.target.value)}
						placeholder='e.g. [topic] | [search]'
					/>
				</div>
				<label className="wf-check-row">
					<input
						type="checkbox"
						checked={disableModelInvocation}
						onChange={(e) => setDisableModelInvocation(e.target.checked)}
					/>
					<span>Disable model invocation (script/command-only)</span>
				</label>
			</div>
			<div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 12 }}>
				<button className="btn" onClick={onClose}>Cancel</button>
				<button className="btn btn-primary" onClick={handleSave}>
					<Settings2 size={14} /> Save
				</button>
			</div>
		</Modal>
	)
}
