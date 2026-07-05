import { useState, useRef, useEffect, useMemo } from "react";
import { ChevronDown, Search } from "lucide-react";

// A single provider's models as the chat composer knows them (from
// /api/chat/providers): models arrive as bare id strings, grouped by provider.
export interface ProviderModels {
	id: string;
	name: string;
	models: string[];
}

interface Props {
	providers: ProviderModels[];
	// "providerID::modelID" — matches the wire format the chat proxy expects.
	value: string;
	onChange: (providerID: string, modelID: string) => void;
	disabled?: boolean;
}

interface FlatModel {
	providerID: string;
	providerName: string;
	modelID: string;
}

export default function ModelSearchSelect({ providers, value, onChange, disabled }: Props) {
	const [open, setOpen] = useState(false);
	const [query, setQuery] = useState("");
	const [activeIndex, setActiveIndex] = useState(0);
	const rootRef = useRef<HTMLDivElement>(null);
	const inputRef = useRef<HTMLInputElement>(null);
	const listRef = useRef<HTMLDivElement>(null);

	// Flatten providers into a single sorted list for filtering + keyboard nav.
	const flat: FlatModel[] = useMemo(
		() =>
			providers.flatMap((p) =>
				p.models.map((m) => ({ providerID: p.id, providerName: p.name, modelID: m })),
			),
		[providers],
	);

	const selected = useMemo(() => {
		if (!value) return null;
		const [providerID, modelID] = value.split("::");
		return flat.find((m) => m.providerID === providerID && m.modelID === modelID) ?? null;
	}, [value, flat]);

	const displayName = selected
		? `${selected.providerName} · ${selected.modelID}`
		: "Select model…";

	const filtered = useMemo(() => {
		const q = query.trim().toLowerCase();
		if (!q) return flat;
		return flat.filter(
			(m) =>
				m.modelID.toLowerCase().includes(q) ||
				m.providerName.toLowerCase().includes(q),
		);
	}, [flat, query]);

	// Group the filtered list by provider for display.
	const grouped = useMemo(() => {
		const map = new Map<string, FlatModel[]>();
		for (const m of filtered) {
			const arr = map.get(m.providerName) ?? [];
			arr.push(m);
			map.set(m.providerName, arr);
		}
		return [...map.entries()].sort((a, b) => a[0].localeCompare(b[0]));
	}, [filtered]);

	// Reset active index when the filtered set changes.
	useEffect(() => {
		setActiveIndex(0);
	}, [query, open]);

	// Click outside to close.
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

	// Keep the active item in view during keyboard nav.
	useEffect(() => {
		const el = listRef.current?.querySelector<HTMLElement>(`[data-idx="${activeIndex}"]`);
		el?.scrollIntoView({ block: "nearest" });
	}, [activeIndex]);

	const pick = (m: FlatModel) => {
		onChange(m.providerID, m.modelID);
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
			if (filtered[activeIndex]) pick(filtered[activeIndex]);
		} else if (e.key === "Escape") {
			e.preventDefault();
			setOpen(false);
			setQuery("");
		}
	};

	let runningIndex = -1;

	return (
		<div className={`combo-select ${open ? "open" : ""}`} ref={rootRef}>
			<button
				type="button"
				className="combo-trigger composer-select"
				data-tip="Model"
				aria-label="Model"
				disabled={disabled}
				onClick={() => {
					setOpen((o) => !o);
					if (!open) setTimeout(() => inputRef.current?.focus(), 0);
				}}
			>
				<span className="combo-trigger-label">{open ? null : displayName}
					{open && (
						<span className="combo-search">
							<Search size={14} />
							<input
								ref={inputRef}
								type="text"
								placeholder="Search models…"
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
					{grouped.length === 0 && (
						<div className="combo-empty">No models match “{query}”.</div>
					)}
					{grouped.map(([providerName, models]) => (
						<div className="combo-group" key={providerName}>
							<div className="combo-group-label">{providerName}</div>
							{models.map((m) => {
				runningIndex += 1;
				const idx = runningIndex;
				const isSelected =
					selected?.providerID === m.providerID && selected?.modelID === m.modelID;
				return (
					<div
						key={`${m.providerID}::${m.modelID}`}
						data-idx={idx}
						className={`combo-item ${isSelected ? "selected" : ""} ${idx === activeIndex ? "active" : ""}`}
						role="option"
						aria-selected={isSelected}
						onClick={() => pick(m)}
					>
						<span className="combo-item-label">{m.modelID}</span>
					</div>
				);
							})}
						</div>
					))}
				</div>
			)}
		</div>
	);
}
