import { useState, useRef, useEffect, useMemo } from "react";
import { ChevronDown, Search } from "lucide-react";

export interface WorkspaceOption {
	id: string;
	path: string;
	name: string;
}

interface Props {
	workspaces: WorkspaceOption[];
	// The selected workspace path ("" = All workspaces).
	value: string;
	onChange: (path: string) => void;
}

export default function WorkspaceSearchSelect({ workspaces, value, onChange }: Props) {
	const [open, setOpen] = useState(false);
	const [query, setQuery] = useState("");
	const [activeIndex, setActiveIndex] = useState(0);
	const rootRef = useRef<HTMLDivElement>(null);
	const inputRef = useRef<HTMLInputElement>(null);
	const listRef = useRef<HTMLDivElement>(null);

	const selected = useMemo(
		() => workspaces.find((w) => w.path === value) ?? null,
		[workspaces, value],
	);
	const displayName = value === "" ? "All workspaces" : (selected?.name ?? "All workspaces");

	const filtered = useMemo(() => {
		const q = query.trim().toLowerCase();
		if (!q) return workspaces;
		return workspaces.filter(
			(w) => w.name.toLowerCase().includes(q) || w.path.toLowerCase().includes(q),
		);
	}, [workspaces, query]);

	// Build the full pickable list (All + filtered) for keyboard indexing.
	const allItems = useMemo(
		() => [{ path: "", name: "All workspaces" }, ...filtered],
		[filtered],
	);

	useEffect(() => setActiveIndex(0), [query, open]);

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

	const pick = (path: string) => {
		onChange(path);
		setOpen(false);
		setQuery("");
	};

	const onKeyDown = (e: React.KeyboardEvent) => {
		if (e.key === "ArrowDown") {
			e.preventDefault();
			setActiveIndex((i) => Math.min(i + 1, allItems.length - 1));
		} else if (e.key === "ArrowUp") {
			e.preventDefault();
			setActiveIndex((i) => Math.max(i - 1, 0));
		} else if (e.key === "Enter") {
			e.preventDefault();
			if (allItems[activeIndex]) pick(allItems[activeIndex].path);
		} else if (e.key === "Escape") {
			e.preventDefault();
			setOpen(false);
			setQuery("");
		}
	};

	return (
		<div className={`combo-select workspace-combo ${open ? "open" : ""}`} ref={rootRef}>
			<button
				type="button"
				className="combo-trigger workspace-trigger"
				data-tip="Filter by workspace (also targets new chats)"
				aria-label="Filter by workspace"
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
								placeholder="Search workspaces…"
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
					{allItems.map((item, idx) => {
						const isAll = item.path === "";
						const isSelected = value === item.path;
						return (
							<div
								key={item.path || "__all"}
								data-idx={idx}
								className={`combo-item ${isSelected ? "selected" : ""} ${idx === activeIndex ? "active" : ""}`}
								role="option"
								aria-selected={isSelected}
								onClick={() => pick(item.path)}
							>
								<span className="combo-item-label">
									{isAll ? "All workspaces" : item.name}
								</span>
								{!isAll && (
									<span className="combo-item-desc">{item.path}</span>
								)}
							</div>
						);
					})}
					{filtered.length === 0 && (
						<div className="combo-empty">No workspaces match “{query}”.</div>
					)}
				</div>
			)}
		</div>
	);
}
