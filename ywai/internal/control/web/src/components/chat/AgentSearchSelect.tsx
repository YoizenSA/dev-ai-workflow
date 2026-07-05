import { useState, useRef, useEffect, useMemo } from "react";
import { ChevronDown, Search } from "lucide-react";

export interface AgentOption {
	name: string;
	description?: string;
}

interface Props {
	agents: AgentOption[];
	value: string;
	onChange: (name: string) => void;
	disabled?: boolean;
}

export default function AgentSearchSelect({ agents, value, onChange, disabled }: Props) {
	const [open, setOpen] = useState(false);
	const [query, setQuery] = useState("");
	const [activeIndex, setActiveIndex] = useState(0);
	const rootRef = useRef<HTMLDivElement>(null);
	const inputRef = useRef<HTMLInputElement>(null);
	const listRef = useRef<HTMLDivElement>(null);

	const selected = useMemo(
		() => agents.find((a) => a.name === value) ?? null,
		[agents, value],
	);
	const displayName = selected ? `@${selected.name}` : "Default agent";

	const filtered = useMemo(() => {
		const q = query.trim().toLowerCase();
		if (!q) return agents;
		return agents.filter(
			(a) =>
				a.name.toLowerCase().includes(q) ||
				(a.description ?? "").toLowerCase().includes(q),
		);
	}, [agents, query]);

	useEffect(() => {
		setActiveIndex(0);
	}, [query, open]);

	useEffect(() => {
		if (!open) return;
		const handler = (e: MouseEvent) => {
			if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
				setOpen(false);
				setQuery("");
			}
		};
		document.addEventListener("mousedown", handler);
		return () => document.removeEventListener("mousedown", handler);
	}, [open]);

	useEffect(() => {
		const el = listRef.current?.querySelector<HTMLElement>(`[data-idx="${activeIndex}"]`);
		el?.scrollIntoView({ block: "nearest" });
	}, [activeIndex]);

	const pick = (name: string) => {
		onChange(name);
		setOpen(false);
		setQuery("");
	};

	const onKeyDown = (e: React.KeyboardEvent) => {
		if (e.key === "ArrowDown") {
			e.preventDefault();
			setActiveIndex((i) => Math.min(i + 1, filtered.length - 1));
		} else if (e.key === "ArrowUp") {
			e.preventDefault();
			setActiveIndex((i) => Math.max(i - 1, 0));
		} else if (e.key === "Enter") {
			e.preventDefault();
			if (filtered[activeIndex]) pick(filtered[activeIndex].name);
		} else if (e.key === "Escape") {
			e.preventDefault();
			setOpen(false);
			setQuery("");
		}
	};

	return (
		<div className={`combo-select ${open ? "open" : ""}`} ref={rootRef}>
			<button
				type="button"
				className="combo-trigger composer-select"
				data-tip="Agent"
				aria-label="Agent"
				disabled={disabled}
				onClick={() => {
					setOpen((o) => !o);
					if (!open) setTimeout(() => inputRef.current?.focus(), 0);
				}}
			>
				<span className="combo-trigger-label">
					{open ? null : displayName}
					{open && (
						<span className="combo-search">
							<Search size={14} />
							<input
								ref={inputRef}
								type="text"
								placeholder="Search agents…"
								value={query}
								onChange={(e) => setQuery(e.target.value)}
								onKeyDown={onKeyDown}
								onClick={(e) => e.stopPropagation()}
							/>
						</span>
					)}
				</span>
				<ChevronDown size={14} className="combo-chevron" />
			</button>

			{open && (
				<div className="combo-dropdown" ref={listRef} role="listbox">
					<div
						className={`combo-item ${value === "" ? "selected" : ""} ${0 === activeIndex ? "active" : ""}`}
						data-idx={0}
						role="option"
						aria-selected={value === ""}
						onClick={() => pick("")}
					>
						<span className="combo-item-label">Default agent</span>
					</div>
					{filtered.length > 0 && <div className="combo-group-label">Agents</div>}
					{filtered.map((a, i) => {
						const idx = i + 1;
						return (
							<div
								key={a.name}
								data-idx={idx}
								className={`combo-item ${value === a.name ? "selected" : ""} ${idx === activeIndex ? "active" : ""}`}
								role="option"
								aria-selected={value === a.name}
								onClick={() => pick(a.name)}
							>
								<span className="combo-item-label">@{a.name}</span>
								{a.description && (
									<span className="combo-item-desc">{a.description}</span>
								)}
							</div>
						);
					})}
					{filtered.length === 0 && (
						<div className="combo-empty">No agents match “{query}”.</div>
					)}
				</div>
			)}
		</div>
	);
}
