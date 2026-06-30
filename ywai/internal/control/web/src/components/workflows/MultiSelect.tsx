import { useState, useMemo } from 'react'
import { Check, ChevronDown } from 'lucide-react'

export interface MultiSelectOption {
	value: string
	label: string
	group?: string
}

interface MultiSelectProps {
	options: MultiSelectOption[]
	/** Currently selected values. */
	selected: Set<string>
	/** Called when the selection changes. */
	onChange: (selected: Set<string>) => void
	/** Placeholder text when nothing is selected. */
	placeholder?: string
	/** Whether to show the "use defaults" hint. */
	emptyIsDefault?: boolean
	defaultLabel?: string
}

// MultiSelect is a checkbox dropdown for picking tools/delegates. Renders
// grouped checkboxes inside a collapsible panel, matching the project's
// CSS-pure glass style (no Tailwind/shadcn).
export default function MultiSelect({
	options,
	selected,
	onChange,
	placeholder = 'Select…',
	emptyIsDefault = false,
	defaultLabel,
}: MultiSelectProps) {
	const [open, setOpen] = useState(false)

	const groups = useMemo(() => {
		const map = new Map<string, MultiSelectOption[]>()
		for (const opt of options) {
			const g = opt.group ?? 'Other'
			if (!map.has(g)) map.set(g, [])
			map.get(g)!.push(opt)
		}
		return [...map.entries()]
	}, [options])

	const toggle = (value: string) => {
		const next = new Set(selected)
		if (next.has(value)) {
			next.delete(value)
		} else {
			next.add(value)
		}
		onChange(next)
	}

	const selectedLabels = options
		.filter((o) => selected.has(o.value))
		.map((o) => o.label)
	const summary =
		selectedLabels.length === 0
			? emptyIsDefault
				? (defaultLabel ?? 'Defaults')
				: placeholder
			: selectedLabels.length <= 2
				? selectedLabels.join(', ')
				: `${selectedLabels.length} selected`

	return (
		<div className="wf-multiselect">
			<button
				type="button"
				className="wf-multiselect-trigger"
				onClick={() => setOpen((v) => !v)}
			>
				<span className={`wf-multiselect-summary ${selectedLabels.length === 0 ? 'is-empty' : ''}`}>
					{summary}
				</span>
				<ChevronDown size={14} className={open ? 'is-open' : ''} />
			</button>
			{open && (
				<div className="wf-multiselect-panel">
					{groups.map(([group, opts]) => (
						<div key={group} className="wf-multiselect-group">
							<div className="wf-multiselect-group-label">{group}</div>
							{opts.map((opt) => (
								<label
									key={opt.value}
									className={`wf-multiselect-item ${selected.has(opt.value) ? 'is-checked' : ''}`}
								>
									<input
										type="checkbox"
										checked={selected.has(opt.value)}
										onChange={() => toggle(opt.value)}
									/>
									<span className="wf-multiselect-check">
										{selected.has(opt.value) && <Check size={12} />}
									</span>
									<span>{opt.label}</span>
								</label>
							))}
						</div>
					))}
				</div>
			)}
		</div>
	)
}
