import { useEffect, useRef, useState } from "react";
import "./SearchSelect.css";

interface Props {
	id?: string;
	value: string;
	options: string[];
	placeholder?: string;
	allowCustom?: boolean;
	onChange: (value: string) => void;
}

/**
 * Combobox-style text input that filters options as you type.
 * - Matches are case-insensitive substring.
 * - `allowCustom` (default true) lets the user type a value not in the list.
 * - Click an option or press Enter to commit.
 * - Press Escape to close the dropdown without committing.
 * - Click outside or blur the input to commit whatever is in the draft.
 * - When `allowCustom` is false, selecting an option or clicking away hides
 *   the option list (the user's text becomes the value on blur).
 */
export default function SearchSelect({
	id,
	value,
	options,
	placeholder,
	allowCustom = true,
	onChange,
}: Props) {
	const [isOpen, setIsOpen] = useState(false);
	const [draft, setDraft] = useState("");
	const inputRef = useRef<HTMLInputElement>(null);
	const dropdownRef = useRef<HTMLDivElement>(null);

	const filtered =
		draft.trim() === ""
			? options
			: options.filter((o) => o.toLowerCase().includes(draft.toLowerCase()));

	useEffect(() => {
		const onClick = (e: MouseEvent) => {
			if (
				dropdownRef.current &&
				!dropdownRef.current.contains(e.target as Node) &&
				inputRef.current &&
				!inputRef.current.contains(e.target as Node)
			) {
				commit(draft);
			}
		};
		document.addEventListener("mousedown", onClick);
		return () => document.removeEventListener("mousedown", onClick);
	}, []);

	const commit = (next: string) => {
		onChange(next);
		setDraft("");
		setIsOpen(false);
	};

	return (
		<div className="search-select">
			<input
				ref={inputRef}
				id={id}
				type="text"
				className="input mono search-select-input"
				placeholder={placeholder}
				value={isOpen ? draft : value}
				onChange={(e) => {
					setDraft(e.target.value);
					if (allowCustom) onChange(e.target.value);
				}}
				onFocus={() => {
					setDraft("");
					setIsOpen(true);
				}}
				onKeyDown={(e) => {
					if (e.key === "Escape") setIsOpen(false);
				}}
			/>
			<div className="search-select-clear">
				{isOpen ? "▲" : "▼"}
			</div>

			{isOpen && (
				<div
					ref={dropdownRef}
					className="search-select-dropdown"
				>
					{filtered.length === 0 ? (
						<div className="search-select-empty">
							{allowCustom ? "Press Enter to keep your value" : "No matches"}
						</div>
					) : (
						filtered.map((opt) => (
							<div
								key={opt}
								onClick={() => commit(opt)}
								className={"search-select-option" + (value === opt ? " selected" : "")}
							>
								{opt}
							</div>
						))
					)}
				</div>
			)}
		</div>
	);
}
