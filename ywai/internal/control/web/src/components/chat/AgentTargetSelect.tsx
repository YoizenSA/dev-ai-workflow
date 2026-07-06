import { useState, useRef, useEffect } from "react";
import { ChevronDown } from "lucide-react";

export interface AgentTargetOption {
	value: string;
	label: string;
	icon: string;
}

interface Props {
	value: string;
	onChange: (value: string) => void;
	connectionStatus: { pi: boolean; opencode: boolean };
	options?: AgentTargetOption[];
	disabled?: boolean;
}

const DEFAULT_OPTIONS: AgentTargetOption[] = [
	{ value: "opencode", label: "OpenCode", icon: "🔷" },
	{ value: "pi", label: "PI.dev", icon: "🟣" },
];

/**
 * Combo selector for the chat agent target (OpenCode / PI.dev).
 * Reuses the `.combo-select` design-system classes so it matches the
 * AgentSearchSelect / ModelSearchSelect look. Each option shows a
 * connection-status dot (green/red) driven by `connectionStatus`.
 *
 * No search — there are only two fixed options.
 */
export default function AgentTargetSelect({
	value,
	onChange,
	connectionStatus,
	options = DEFAULT_OPTIONS,
	disabled,
}: Props) {
	const [open, setOpen] = useState(false);
	const [activeIndex, setActiveIndex] = useState(0);
	const rootRef = useRef<HTMLDivElement>(null);
	const listRef = useRef<HTMLDivElement>(null);

	const selected = options.find((o) => o.value === value) ?? options[0];
	const isConnected = (v: string) =>
		v === "pi" ? connectionStatus.pi : connectionStatus.opencode;

	useEffect(() => {
		setActiveIndex(Math.max(0, options.findIndex((o) => o.value === value)));
	}, [value, options]);

	useEffect(() => {
		if (!open) return;
		const handler = (e: MouseEvent) => {
			if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
				setOpen(false);
			}
		};
		document.addEventListener("mousedown", handler);
		return () => document.removeEventListener("mousedown", handler);
	}, [open]);

	useEffect(() => {
		const el = listRef.current?.querySelector<HTMLElement>(
			`[data-idx="${activeIndex}"]`,
		);
		el?.scrollIntoView({ block: "nearest" });
	}, [activeIndex]);

	const pick = (v: string) => {
		onChange(v);
		setOpen(false);
	};

	const onKeyDown = (e: React.KeyboardEvent) => {
		if (e.key === "ArrowDown") {
			e.preventDefault();
			setOpen(true);
			setActiveIndex((i) => Math.min(i + 1, options.length - 1));
		} else if (e.key === "ArrowUp") {
			e.preventDefault();
			setOpen(true);
			setActiveIndex((i) => Math.max(i - 1, 0));
		} else if (e.key === "Enter") {
			e.preventDefault();
			if (options[activeIndex]) pick(options[activeIndex].value);
		} else if (e.key === "Escape") {
			e.preventDefault();
			setOpen(false);
		}
	};

	const dotColor = isConnected(selected.value);

	return (
		<div
			className={`combo-select${open ? " open" : ""}`}
			ref={rootRef}
			onKeyDown={onKeyDown}
			tabIndex={0}
		>
			<button
				type="button"
				className="combo-trigger composer-select agent-target-trigger"
				aria-label="Agent target"
				title="Select AI agent"
				disabled={disabled}
				onClick={() => setOpen((o) => !o)}
			>
				<span
					className="connection-dot"
					style={{ background: dotColor ? "#4caf50" : "#f44336" }}
					title={
						dotColor ? `${selected.label} connected` : `${selected.label} disconnected`
					}
				/>
				<span className="combo-trigger-label">
					{selected.icon} {selected.label}
				</span>
				<ChevronDown size={14} className="combo-chevron" />
			</button>

			{open && (
				<div className="combo-dropdown" ref={listRef} role="listbox">
					<div className="combo-group-label">Backend</div>
					{options.map((o, i) => {
						const connected = isConnected(o.value);
						return (
							<div
								key={o.value}
								data-idx={i}
								className={`combo-item ${value === o.value ? "selected" : ""} ${i === activeIndex ? "active" : ""}`}
								role="option"
								aria-selected={value === o.value}
								onClick={() => pick(o.value)}
							>
								<span className="combo-item-label agent-target-item">
									<span
										className="connection-dot"
										style={{ background: connected ? "#4caf50" : "#f44336" }}
										title={connected ? `${o.label} connected` : `${o.label} disconnected`}
									/>
									{o.icon} {o.label}
								</span>
							</div>
						);
					})}
				</div>
			)}
		</div>
	);
}
