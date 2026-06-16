import { useState } from 'react'
import Modal from '../shared/Modal'
import SearchSelect from '../shared/SearchSelect'
import { useMemoriesStore } from '../../stores/memoriesStore'

interface Props {
	open: boolean
	onClose: () => void
}

const TYPES = ['save', 'observation', 'summary', 'topic']
const SCOPES = ['project', 'personal', 'global']

export default function CaptureMemoryModal({ open, onClose }: Props) {
	const [type, setType] = useState('save')
	const [content, setContent] = useState('')
	const [scope, setScope] = useState('project')
	const saveMemory = useMemoriesStore((s) => s.saveMemory)

	const submit = async () => {
		if (!content.trim()) return
		await saveMemory({ type, content, scope })
		setContent('')
		setScope('project')
		onClose()
	}

	return (
		<Modal
			open={open}
			onClose={onClose}
			title="Capture Memory"
			width="560px"
			footer={
				<>
					<button className="btn btn-ghost" onClick={onClose}>
						Cancel
					</button>
					<button
						className="btn btn-primary"
						onClick={submit}
						disabled={!content.trim()}
					>
						Save
					</button>
				</>
			}
		>
			<div className="field">
				<label className="field-label">Type</label>
				<SearchSelect value={type} options={TYPES} onChange={setType} />
			</div>
			<div className="field">
				<label className="field-label">Content</label>
				<textarea
					className="textarea"
					rows={5}
					value={content}
					onChange={(e) => setContent(e.target.value)}
				/>
			</div>
			<div className="field">
				<label className="field-label">Scope</label>
				<SearchSelect
					value={scope}
					options={SCOPES}
					onChange={setScope}
				/>
			</div>
		</Modal>
	)
}
