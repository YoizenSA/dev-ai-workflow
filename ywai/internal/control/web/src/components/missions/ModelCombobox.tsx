import { useState, useRef, useEffect } from "react";
import type { ModelInfo } from "../../api/types";

interface Props {
	id: string;
	label: string;
	value: string;
	models: ModelInfo[];
	onChange: (value: string) => void;
	// Model ids/names to surface under a "Recommended" group. Provided by the
	// caller (e.g. derived from the configured role defaults) — never hardcoded.
	recommended?: string[];
}

export default function ModelCombobox({ id, label, value, models, onChange, recommended = [] }: Props) {
	const [isOpen, setIsOpen] = useState(false);
	const [searchText, setSearchText] = useState("");
	const inputRef = useRef<HTMLInputElement>(null);
	const dropdownRef = useRef<HTMLDivElement>(null);

	const selectedModel = models.find((m) => m.id === value);
	const displayName = selectedModel?.name || selectedModel?.id || "Select model…";

	const filteredModels =
		searchText.trim() === ""
			? models
			: models.filter(
					(m) =>
						m.name.toLowerCase().includes(searchText.toLowerCase()) ||
						m.id.toLowerCase().includes(searchText.toLowerCase()),
				);

	const isRecommended = (m: ModelInfo) =>
		recommended.some((r) => m.id.includes(r) || m.name.includes(r));
	const recommendedModels = filteredModels.filter(isRecommended);
	const otherModels = filteredModels.filter((m) => !isRecommended(m));

	const modelsByProvider = otherModels.reduce<Record<string, ModelInfo[]>>((acc, m) => {
		const key = m.provider || "default";
		(acc[key] ||= []).push(m);
		return acc;
	}, {});
	const sortedProviders = Object.keys(modelsByProvider).sort();

	useEffect(() => {
		const handleClickOutside = (e: MouseEvent) => {
			if (
				dropdownRef.current &&
				!dropdownRef.current.contains(e.target as Node) &&
				inputRef.current &&
				!inputRef.current.contains(e.target as Node)
			) {
				setIsOpen(false);
			}
		};
		document.addEventListener("mousedown", handleClickOutside);
		return () => document.removeEventListener("mousedown", handleClickOutside);
	}, []);

	return (
		<div className="field">
			<label className="field-label" htmlFor={id}>
				{label}
			</label>
			<div style={{ position: "relative" }}>
				<input
					ref={inputRef}
					id={id}
					type="text"
					className="input"
					placeholder="Search models…"
					value={isOpen ? searchText : displayName}
					onChange={(e) => setSearchText(e.target.value)}
					onFocus={() => setIsOpen(true)}
					onKeyDown={(e) => {
						if (e.key === "Escape") setIsOpen(false);
					}}
					style={{
						paddingRight: "32px",
						cursor: "pointer",
					}}
				/>
				<div
					style={{
						position: "absolute",
						right: "8px",
						top: "50%",
						transform: "translateY(-50%)",
						pointerEvents: "none",
						fontSize: "12px",
						color: "var(--text-muted)",
					}}
				>
					{isOpen ? "▲" : "▼"}
				</div>

				{isOpen && models.length > 0 && (
					<div
						ref={dropdownRef}
						style={{
							position: "absolute",
							top: "100%",
							left: 0,
							right: 0,
							marginTop: "4px",
							backgroundColor: "var(--yz-dark-soft)",
							border: "1px solid var(--panel-border)",
							borderRadius: "6px",
							maxHeight: "400px",
							overflowY: "auto",
							zIndex: 1000,
							boxShadow: "0 4px 12px rgba(0,0,0,0.3)",
						}}
					>
						{recommendedModels.length > 0 && (
							<>
								<div
									style={{
										padding: "8px 12px",
										fontSize: "11px",
										fontWeight: 600,
										color: "var(--text-muted)",
										textTransform: "uppercase",
										borderBottom: "1px solid var(--panel-border)",
										backgroundColor: "var(--surface-hover)",
									}}
								>
									Recommended
								</div>
								{recommendedModels.map((m) => (
									<div
										key={m.id}
										onClick={() => {
											onChange(m.id);
											setIsOpen(false);
											setSearchText("");
										}}
										style={{
											padding: "8px 12px",
											cursor: "pointer",
											backgroundColor:
												value === m.id ? "var(--info-soft)" : "transparent",
											color:
												value === m.id ? "var(--info)" : "var(--text)",
											fontSize: "13px",
										}}
										onMouseEnter={(e) => {
											if (value !== m.id) {
												(e.target as HTMLElement).style.backgroundColor =
													"var(--surface-hover)";
											}
										}}
										onMouseLeave={(e) => {
											if (value !== m.id) {
												(e.target as HTMLElement).style.backgroundColor =
													"transparent";
											}
										}}
									>
										<div style={{ fontWeight: value === m.id ? 600 : 400 }}>
											{m.name || m.id}
										</div>
										<div
											style={{
												fontSize: "11px",
												color: "var(--text-muted)",
											}}
										>
											{m.provider}
										</div>
									</div>
								))}
							</>
						)}

						{sortedProviders.length > 0 && (
							<>
								{recommendedModels.length > 0 && (
									<div
										style={{
											padding: "8px 12px",
											fontSize: "11px",
											fontWeight: 600,
											color: "var(--text-muted)",
											textTransform: "uppercase",
											borderTop: "1px solid var(--panel-border)",
											borderBottom: "1px solid var(--panel-border)",
											backgroundColor: "var(--surface-hover)",
										}}
									>
										All Models
									</div>
								)}
								{sortedProviders.map((provider) => (
									<div key={provider}>
										<div
											style={{
												padding: "6px 12px",
												fontSize: "11px",
												fontWeight: 600,
												color: "var(--text-muted)",
												backgroundColor: "var(--surface-hover)",
											}}
										>
											{provider}
										</div>
										{modelsByProvider[provider]
											.slice()
											.sort((a, b) =>
												(a.name || a.id).localeCompare(b.name || b.id),
											)
											.map((m) => (
												<div
													key={m.id}
													onClick={() => {
														onChange(m.id);
														setIsOpen(false);
														setSearchText("");
													}}
													style={{
														padding: "8px 12px",
														paddingLeft: "20px",
														cursor: "pointer",
														backgroundColor:
															value === m.id ? "var(--info-soft)" : "transparent",
														color:
															value === m.id
																? "var(--info)"
																: "var(--text)",
														fontSize: "13px",
													}}
													onMouseEnter={(e) => {
														if (value !== m.id) {
															(e.target as HTMLElement).style.backgroundColor =
																"var(--surface-hover)";
														}
													}}
													onMouseLeave={(e) => {
														if (value !== m.id) {
															(e.target as HTMLElement).style.backgroundColor =
																"transparent";
														}
													}}
												>
													{m.name || m.id}
												</div>
											))}
									</div>
								))}
							</>
						)}

						{filteredModels.length === 0 && (
							<div
								style={{
									padding: "16px 12px",
									textAlign: "center",
									color: "var(--text-muted)",
									fontSize: "13px",
								}}
							>
								No models found
							</div>
						)}
					</div>
				)}
			</div>
		</div>
	);
}
