import { useEffect, useState } from "react";
import { configApi } from "../../api/client";
import type { OpenCodeConfig, Reference } from "../../api/types";

// ─── Types ──────────────────────────────────────────────────────────────────

interface ReferenceEntry {
	alias: string;
	ref: Reference;
}

type ReferenceType = "local" | "git";

interface ReferenceForm {
	alias: string;
	type: ReferenceType;
	path: string;
	repository: string;
	branch: string;
	description: string;
	hidden: boolean;
}

const EMPTY_FORM: ReferenceForm = {
	alias: "",
	type: "local",
	path: "",
	repository: "",
	branch: "",
	description: "",
	hidden: false,
};

const DESCRIPTION_MAX_RECOMMENDED = 200;

// ─── Validation ─────────────────────────────────────────────────────────────

function validateAlias(alias: string, existing: string[], editing?: string): string | null {
	if (!alias.trim()) return "Alias is required";
	if (/[\/\s,`]/.test(alias)) return "Alias cannot contain /, whitespace, backticks, or commas";
	if (existing.includes(alias) && alias !== editing) return "Alias already exists";
	return null;
}

function validateForm(form: ReferenceForm, existing: string[], editing?: string): string | null {
	const aliasErr = validateAlias(form.alias, existing, editing);
	if (aliasErr) return aliasErr;

	if (form.type === "local" && !form.path.trim()) return "Path is required for local references";
	if (form.type === "git" && !form.repository.trim()) return "Repository is required for git references";

	return null;
}

// ─── Helpers ────────────────────────────────────────────────────────────────

function isLocalRef(ref: Reference): boolean {
	return !!ref.path;
}

function normalizeForm(form: ReferenceForm): Reference {
	if (form.type === "local") {
		return {
			path: form.path.trim(),
			description: form.description.trim() || undefined,
			hidden: form.hidden || undefined,
		};
	}
	return {
		repository: form.repository.trim(),
		branch: form.branch.trim() || undefined,
		description: form.description.trim() || undefined,
		hidden: form.hidden || undefined,
	};
}

function formFromEntry(entry: ReferenceEntry): ReferenceForm {
	const { alias, ref } = entry;
	if (isLocalRef(ref)) {
		return {
			alias,
			type: "local",
			path: ref.path ?? "",
			repository: "",
			branch: "",
			description: ref.description ?? "",
			hidden: ref.hidden ?? false,
		};
	}
	return {
		alias,
		type: "git",
		path: "",
		repository: ref.repository ?? "",
		branch: ref.branch ?? "",
		description: ref.description ?? "",
		hidden: ref.hidden ?? false,
	};
}

// ─── Config parsing ─────────────────────────────────────────────────────────

function parseReferences(raw?: Record<string, Reference | string>): ReferenceEntry[] {
	if (!raw) return [];
	return Object.entries(raw).map(([alias, ref]) => ({
		alias,
		ref: typeof ref === "string" ? { path: ref } : ref,
	}));
}

function toConfigReferences(entries: ReferenceEntry[]): Record<string, Reference> {
	const result: Record<string, Reference> = {};
	for (const { alias, ref } of entries) {
		result[alias] = ref;
	}
	return result;
}

// ─── Component ──────────────────────────────────────────────────────────────

export default function ReferencesTab() {
	const [references, setReferences] = useState<ReferenceEntry[]>([]);
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);

	// Form state
	const [showForm, setShowForm] = useState(false);
	const [editing, setEditing] = useState<string | null>(null);
	const [form, setForm] = useState<ReferenceForm>({ ...EMPTY_FORM });
	const [formError, setFormError] = useState<string | null>(null);

	// Load references from config
	useEffect(() => {
		configApi
			.getConfig()
			.then((config) => {
				setReferences(parseReferences(config.references));
				setLoading(false);
			})
			.catch(() => setLoading(false));
	}, []);

	// ─── Directory picker ─────────────────────────────────────────────────
	const handleBrowse = async () => {
		try {
			const res = await fetch("/api/browse-directory", { method: "POST" });
			if (res.status === 204) {
				// User cancelled or dialog not available
				return;
			}
			if (!res.ok) {
				const text = await res.text();
				console.warn("Browse failed:", text);
				return;
			}
			const data = await res.json();
			if (data.path) {
				setForm((f) => ({ ...f, path: data.path }));
			}
		} catch (err) {
			console.warn("Browse error:", err);
		}
	};

	// ─── Save to config ─────────────────────────────────────────────────

	const saveToConfig = async (entries: ReferenceEntry[]) => {
		setSaving(true);
		setError(null);
		try {
			const config: OpenCodeConfig = await configApi.getConfig();
			const updated = {
				...config,
				references: toConfigReferences(entries),
			};
			await configApi.updateConfig(updated);
		} catch (err) {
			setError(`Failed to save: ${err}`);
		} finally {
			setSaving(false);
		}
	};

	// ─── Add / Edit ─────────────────────────────────────────────────────

	const openAdd = () => {
		setEditing(null);
		setForm({ ...EMPTY_FORM });
		setFormError(null);
		setShowForm(true);
	};

	const openEdit = (entry: ReferenceEntry) => {
		setEditing(entry.alias);
		setForm(formFromEntry(entry));
		setFormError(null);
		setShowForm(true);
	};

	const closeForm = () => {
		setShowForm(false);
		setEditing(null);
		setForm({ ...EMPTY_FORM });
		setFormError(null);
	};

	const handleSave = async () => {
		const existingAliases = references.map((r) => r.alias);
		const err = validateForm(form, existingAliases, editing ?? undefined);
		if (err) {
			setFormError(err);
			return;
		}

		const normalized = normalizeForm(form);
		let next: ReferenceEntry[];

		if (editing) {
			// Replace, remove old entry
			next = references.filter((r) => r.alias !== editing);
			next.push({ alias: form.alias.trim(), ref: normalized });
		} else {
			next = [...references, { alias: form.alias.trim(), ref: normalized }];
		}

		setReferences(next);
		closeForm();
		await saveToConfig(next);
	};

	// ─── Delete ─────────────────────────────────────────────────────────

	const handleDelete = async (alias: string) => {
		if (!confirm(`Delete reference "${alias}"?`)) return;
		const next = references.filter((r) => r.alias !== alias);
		setReferences(next);
		await saveToConfig(next);
	};

	// ─── Loading ────────────────────────────────────────────────────────

	if (loading) {
		return (
			<div className="card card-pad">
				<div className="spinner" />
			</div>
		);
	}

	// ─── Render ─────────────────────────────────────────────────────────

	return (
		<div className="card card-pad">
			{error && (
				<div className="alert alert-danger" style={{ marginBottom: "var(--space-3)" }}>
					{error}
				</div>
			)}

			{saving && (
				<div className="alert" style={{ marginBottom: "var(--space-3)" }}>
					<div className="spinner" style={{ display: "inline-block", marginRight: "var(--space-2)" }} />
					Saving…
				</div>
			)}

			{/* Header row */}
			<div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "var(--space-4)" }}>
				<button className="btn btn-primary btn-sm" onClick={openAdd}>
					+ Add Reference
				</button>
			</div>

			{/* Empty state */}
			{references.length === 0 && !showForm && (
				<div className="empty-state">
					<div className="empty-icon">
						<svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
							<path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
							<path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" />
						</svg>
					</div>
					<span className="empty-title">No references configured</span>
					<span className="empty-desc">
						Add references to give agents access to local directories or git repositories
					</span>
				</div>
			)}

			{/* Reference list */}
			{references.length > 0 && (
				<div className="mcp-grid">
					{references.map((entry) => {
						const isLocal = isLocalRef(entry.ref);
						const hasDescription = !!entry.ref.description?.trim();
						return (
							<div key={entry.alias} className="mcp-card">
								<div className="mcp-card-header">
									<div style={{ display: "flex", alignItems: "center", gap: "var(--space-2)" }}>
										<span style={{ fontSize: "1.1rem" }} title={isLocal ? "Local directory" : "Git repository"}>
											{isLocal ? "📁" : "🔗"}
										</span>
										<h3>{entry.alias}</h3>
									</div>
									<div style={{ display: "flex", gap: "var(--space-1)", alignItems: "center" }}>
										<span className="pill pill-muted">
											{isLocal ? "local" : "git"}
										</span>
										{entry.ref.hidden && (
											<span className="pill" style={{ background: "var(--warning-surface, #fef3cd)", color: "var(--warning-text, #856404)", borderColor: "var(--warning-border, #ffc107)" }}>
												Hidden
											</span>
										)}
									</div>
								</div>
								<p className="mcp-command-desc" style={{ fontFamily: "var(--font-mono, monospace)", fontSize: "0.82rem" }}>
									{isLocal ? entry.ref.path : entry.ref.repository}
									{!isLocal && entry.ref.branch && (
										<span className="pill pill-muted" style={{ marginLeft: "var(--space-1)", fontSize: "0.75rem", verticalAlign: "middle" }}>
											@{entry.ref.branch}
										</span>
									)}
								</p>
								{hasDescription ? (
									<p style={{ margin: "var(--space-1) 0 0", fontSize: "0.85rem", color: "var(--text-muted)" }}>
										{entry.ref.description}
									</p>
								) : (
									<p style={{ margin: "var(--space-1) 0 0", fontSize: "0.8rem", color: "var(--warning-text, #856404)", fontStyle: "italic" }}>
										⚠ No description — agents won't see this
									</p>
								)}
								<div className="mcp-actions-row" style={{ marginTop: "var(--space-3)" }}>
									<button
										className="btn btn-ghost btn-sm"
										onClick={() => openEdit(entry)}
									>
										Edit
									</button>
									<button
										className="btn btn-danger btn-sm"
										onClick={() => handleDelete(entry.alias)}
									>
										Delete
									</button>
								</div>
							</div>
						);
					})}
				</div>
			)}

			{/* Add / Edit form */}
			{showForm && (
				<div style={{ marginTop: references.length > 0 ? "var(--space-4)" : 0 }}>
					<div className="card card-pad" style={{ background: "var(--surface-soft)" }}>
						<h3 style={{ margin: "0 0 var(--space-4)" }}>
							{editing ? "Edit" : "Add"} Reference
						</h3>

						{formError && (
							<div className="alert alert-danger" style={{ marginBottom: "var(--space-3)" }}>
								{formError}
							</div>
						)}

						{/* ── Identity ── */}
						<div style={{ marginBottom: "var(--space-4)" }}>
							<h4 style={{ margin: "0 0 var(--space-2)", fontSize: "0.8rem", fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--text-muted)" }}>
								Identity
							</h4>
							<div className="settings-item">
								<div className="settings-item-header">
									<label htmlFor="ref-alias">Alias *</label>
								</div>
								<input
									id="ref-alias"
									type="text"
									className="input"
									value={form.alias}
									onChange={(e) => setForm((f) => ({ ...f, alias: e.target.value }))}
									placeholder="my-reference"
									disabled={!!editing}
								/>
								<span className="settings-item-desc">Short name used as @reference in chat. No spaces or slashes.</span>
							</div>
						</div>

						{/* ── Source ── */}
						<div style={{ marginBottom: "var(--space-4)" }}>
							<h4 style={{ margin: "0 0 var(--space-2)", fontSize: "0.8rem", fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--text-muted)" }}>
								Source
							</h4>
							<div className="settings-item">
								<div className="settings-item-header">
									<label htmlFor="ref-type">Type *</label>
								</div>
								<select
									id="ref-type"
									className="input"
									value={form.type}
									onChange={(e) => setForm((f) => ({ ...f, type: e.target.value as ReferenceType }))}
									disabled={!!editing}
								>
									<option value="local">Local directory</option>
									<option value="git">Git repository</option>
								</select>
							</div>

							{form.type === "local" ? (
								<div className="settings-item">
									<div className="settings-item-header">
										<label htmlFor="ref-path">Path *</label>
									</div>
									<div style={{ display: "flex", gap: "var(--space-2)", alignItems: "center" }}>
										<input
											id="ref-path"
											type="text"
											className="input mono"
											value={form.path}
											onChange={(e) => setForm((f) => ({ ...f, path: e.target.value }))}
											placeholder="/absolute/path or ~/relative/path"
											style={{ flex: 1 }}
										/>
									<button
										type="button"
										className="btn btn-ghost btn-sm"
										onClick={handleBrowse}
										title="Browse for directory"
									>
											Browse…
										</button>
									</div>
									<span className="settings-item-desc">Absolute, relative to config file, or ~/path</span>
								</div>
							) : (
								<>
									<div className="settings-item">
										<div className="settings-item-header">
											<label htmlFor="ref-repository">Repository *</label>
										</div>
										<input
											id="ref-repository"
											type="text"
											className="input mono"
											value={form.repository}
											onChange={(e) => setForm((f) => ({ ...f, repository: e.target.value }))}
											placeholder="owner/repo or https://github.com/owner/repo"
										/>
										<span className="settings-item-desc">GitHub shorthand (owner/repo) or full Git URL</span>
									</div>
									<div className="settings-item">
										<div className="settings-item-header">
											<label htmlFor="ref-branch">Branch</label>
											<span className="settings-item-desc">Optional</span>
										</div>
										<input
											id="ref-branch"
											type="text"
											className="input mono"
											value={form.branch}
											onChange={(e) => setForm((f) => ({ ...f, branch: e.target.value }))}
											placeholder="main"
										/>
										<span className="settings-item-desc">Defaults to the repository's default branch</span>
									</div>
								</>
							)}
						</div>

						{/* ── Agent Context ── */}
						<div style={{ marginBottom: "var(--space-4)" }}>
							<h4 style={{ margin: "0 0 var(--space-2)", fontSize: "0.8rem", fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--text-muted)" }}>
								Agent Context
							</h4>
							<div className="settings-item">
								<div className="settings-item-header">
									<label htmlFor="ref-description">Description</label>
									<span className="settings-item-desc">
										{form.description.length}/{DESCRIPTION_MAX_RECOMMENDED}
									</span>
								</div>
								<textarea
									id="ref-description"
									className="input"
									value={form.description}
									onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
									placeholder={`"Use when implementing UI components or working with design tokens"`}
									rows={3}
									style={{
										resize: "vertical",
										minHeight: "4.5rem",
										color: form.description.length > DESCRIPTION_MAX_RECOMMENDED ? "var(--danger, #dc3545)" : undefined,
									}}
								/>
								<span className="settings-item-desc" style={{ lineHeight: 1.5 }}>
									Descriptions are included in agent context. Write <strong>when</strong> to use this reference, not <strong>what</strong> it contains.{" "}
									<br />
									<span style={{ color: "var(--text-muted)" }}>
										Good: <em>"Use for authentication flows and JWT handling"</em> — Bad: <em>"Auth library"</em>
									</span>
								</span>
							</div>

							<div className="settings-item">
								<div className="settings-item-header">
									<label htmlFor="ref-hidden" style={{ display: "flex", alignItems: "center", gap: "var(--space-2)" }}>
										<input
											id="ref-hidden"
											type="checkbox"
											checked={form.hidden}
											onChange={(e) => setForm((f) => ({ ...f, hidden: e.target.checked }))}
										/>
										Hidden
									</label>
								</div>
								<span className="settings-item-desc">Hidden references are available but not shown in the default list</span>
							</div>
						</div>

						<div style={{ display: "flex", gap: "var(--space-2)", marginTop: "var(--space-3)" }}>
							<button className="btn btn-primary" onClick={handleSave}>
								{editing ? "Update" : "Add"} Reference
							</button>
							<button className="btn btn-ghost" onClick={closeForm}>
								Cancel
							</button>
						</div>
					</div>
				</div>
			)}
		</div>
	);
}
