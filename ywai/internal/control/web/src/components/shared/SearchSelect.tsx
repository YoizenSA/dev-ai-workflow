import { useEffect, useRef, useState } from "react";

interface Props {
	id?: string;
	value: string;
	options: string[];
	placeholder?: string;
	allowCustom?: boolean;
	onChange: (value: string) => void;
}

/**
 * SearchSelect — input + dropdown of options.
 *
 * Unlike a native <datalist>, the dropdown always shows the full list on focus
 * and filters as the user types. Set allowCustom=true to accept arbitrary
 * strings outside the option list (the user's text becomes the value on blur).
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
				setIsOpen(false);
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
		<div style={{ position: "relative" }}>
			<input
				ref={inputRef}
				id={id}
				type="text"
				className="input mono"
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
				style={{ paddingRight: 32, cursor: "pointer" }}
			/>
			<div
				style={{
					position: "absolute",
					right: 8,
					top: "50%",
					transform: "translateY(-50%)",
					pointerEvents: "none",
					fontSize: 12,
					color: "var(--text-muted)",
				}}
			>
				{isOpen ? "▲" : "▼"}
			</div>

			{isOpen && (
				<div
					ref={dropdownRef}
					style={{
						position: "absolute",
						top: "100%",
						left: 0,
						right: 0,
						marginTop: 4,
						backgroundColor: "var(--yz-dark-soft)",
						border: "1px solid var(--panel-border)",
						borderRadius: 6,
						maxHeight: 280,
						overflowY: "auto",
						zIndex: 1000,
						boxShadow: "0 4px 12px rgba(0,0,0,0.3)",
					}}
				>
					{filtered.length === 0 ? (
						<div
							style={{
								padding: "10px 12px",
								color: "var(--text-muted)",
								fontSize: 13,
								textAlign: "center",
							}}
						>
							{allowCustom ? "Press Enter to keep your value" : "No matches"}
						</div>
					) : (
						filtered.map((opt) => (
							<div
								key={opt}
								onClick={() => commit(opt)}
								style={{
									padding: "8px 12px",
									cursor: "pointer",
									backgroundColor:
										value === opt ? "var(--info-soft)" : "transparent",
									color: value === opt ? "var(--info)" : "var(--text)",
									fontSize: 13,
									fontFamily: "var(--font-mono)",
								}}
								onMouseEnter={(e) => {
									if (value !== opt) {
										(e.target as HTMLElement).style.backgroundColor =
											"var(--surface-hover)";
									}
								}}
								onMouseLeave={(e) => {
									if (value !== opt) {
										(e.target as HTMLElement).style.backgroundColor =
											"transparent";
									}
								}}
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
