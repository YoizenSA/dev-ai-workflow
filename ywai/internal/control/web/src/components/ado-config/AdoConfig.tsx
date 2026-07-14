import { useEffect, useMemo, useState } from "react";
import Modal from "../shared/Modal";
import AdoSetupWizard from "./AdoSetupWizard";
import { buildToml, buildTomlCommand, defaultToml, type TomlConfig } from "./tomlBuilder";
import "./AdoConfig.css";

// ─── Types (mirror the Go structs in internal/control/ado_config.go) ───────

interface AdoProfile {
	org: string;
	project: string;
	patEnvVar: string;
	repos: string[];
	default?: boolean;
}

interface AdoConfig {
	defaultProfile: string;
	profiles: Record<string, AdoProfile>;
}

interface PatStatus {
	hasPat: boolean;
	source: "env" | "file" | "none";
}

interface CliStatus {
	installed: boolean;
	version: string;
	latest: string | null;
	updateAvailable: boolean;
	error?: string;
}

export default function AdoConfig() {
	const [config, setConfig] = useState<AdoConfig | null>(null);
	const [cliStatus, setCliStatus] = useState<CliStatus | null>(null);
	const [patStatus, setPatStatus] = useState<PatStatus | null>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [message, setMessage] = useState<{ text: string; type: "success" | "error" } | null>(null);

	// Wizard
	const [showWizard, setShowWizard] = useState(false);

	// Profile add/edit modal (management view)
	const [showModal, setShowModal] = useState(false);
	const [editingName, setEditingName] = useState<string | null>(null);
	const [formName, setFormName] = useState("");
	const [formProject, setFormProject] = useState("");
	const [formRepos, setFormRepos] = useState<string[]>([]);
	const [repoInput, setRepoInput] = useState("");

	// PAT modal (management view)
	const [showPatModal, setShowPatModal] = useState(false);
	const [patValue, setPatValue] = useState("");

	// CLI update
	const [cliUpdateStatus, setCliUpdateStatus] = useState<"idle" | "updating" | "error">("idle");

	// TOML builder (collapsible)
	const [toml, setToml] = useState<TomlConfig>(defaultToml());
	const [tomlCopied, setTomlCopied] = useState(false);
	const [showTomlBuilder, setShowTomlBuilder] = useState(false);

	useEffect(() => {
		Promise.all([
			fetch("/api/ado/config").then((r) => r.json()),
			fetch("/api/ado/cli-status").then((r) => r.json()),
			fetch("/api/ado/pat-status").then((r) => r.json()),
		])
			.then(([cfg, cli, pat]) => {
				setConfig(cfg);
				setCliStatus(cli);
				setPatStatus(pat);
				setLoading(false);
			})
			.catch(() => { setError("Failed to load ADO config"); setLoading(false); });
	}, []);

	useEffect(() => {
		if (!message) return;
		const id = setTimeout(() => setMessage(null), 3000);
		return () => clearTimeout(id);
	}, [message]);

	// ─── Derived: the org is global (taken from the default profile) ──────
	const profileNames = config ? Object.keys(config.profiles) : [];
	const defaultProfileName = config?.defaultProfile ?? "";
	const globalOrg = config && defaultProfileName
		? (config.profiles[defaultProfileName]?.org ?? "")
		: (profileNames.length > 0 ? config!.profiles[profileNames[0]].org : "");
	// Detect legacy heterogeneous orgs (so we can warn).
	const orgMismatch = useMemo(() => {
		if (!config) return false;
		const orgs = new Set(Object.values(config.profiles).map((p) => p.org));
		return orgs.size > 1;
	}, [config]);

	// ─── Profile handlers (management modal: project + repos only) ────────

	const openAddModal = () => {
		setEditingName(null);
		setFormName("");
		setFormProject("");
		setFormRepos([]);
		setRepoInput("");
		setShowModal(true);
	};

	const openEditModal = (name: string) => {
		const p = config?.profiles[name];
		if (!p) return;
		setEditingName(name);
		setFormName(name);
		setFormProject(p.project);
		setFormRepos([...(p.repos ?? [])]);
		setRepoInput("");
		setShowModal(true);
	};

	const closeModal = () => setShowModal(false);

	const addRepo = () => {
		const raw = repoInput.trim();
		if (!raw) return;
		const names = raw.split(",").map((x) => x.trim()).filter(Boolean);
		const valid = names.filter((n) => /^[a-zA-Z0-9._-]+$/.test(n));
		if (valid.length !== names.length) {
			setMessage({ text: "Repo names may only contain letters, numbers, dots, hyphens, underscores", type: "error" });
		}
		setFormRepos([...new Set([...formRepos, ...valid])]);
		setRepoInput("");
	};

	const removeRepo = (r: string) => setFormRepos(formRepos.filter((x) => x !== r));

	const handleRepoKeyDown = (e: React.KeyboardEvent) => {
		if (e.key === "Enter") { e.preventDefault(); addRepo(); }
	};

	const handleSaveProfile = async () => {
		if (!formProject.trim()) {
			setMessage({ text: "Project is required", type: "error" });
			return;
		}
		const fallback = formName.trim() || formProject.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
		const name = (editingName ?? fallback).slice(0, 50);
		if (!name) {
			setMessage({ text: "Could not derive a profile name", type: "error" });
			return;
		}
		try {
			const res = await fetch("/api/ado/profile", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					name,
					profile: { org: globalOrg, project: formProject.trim(), patEnvVar: "AZURE_DEVOPS_PAT", repos: formRepos },
				}),
			});
			const data = await res.json();
			if (!res.ok) throw new Error(data.error || "Failed to save profile");
			setConfig(data.config);
			setMessage({ text: `Profile "${name}" saved`, type: "success" });
			closeModal();
		} catch (e) {
			setMessage({ text: (e as Error).message, type: "error" });
		}
	};

	const handleDeleteProfile = async (name: string) => {
		if (!confirm(`Delete profile "${name}"?`)) return;
		try {
			const res = await fetch("/api/ado/profile", {
				method: "DELETE",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({ name }),
			});
			const data = await res.json();
			if (!res.ok) throw new Error(data.error || "Failed to delete profile");
			setConfig(data.config);
			setMessage({ text: `Profile "${name}" deleted`, type: "success" });
		} catch (e) {
			setMessage({ text: (e as Error).message, type: "error" });
		}
	};

	const handleDefaultChange = async (name: string) => {
		if (!config) return;
		const updated = { ...config, defaultProfile: name };
		try {
			const res = await fetch("/api/ado/config", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify(updated),
			});
			const data = await res.json();
			if (!res.ok) throw new Error(data.error || "Failed to set default");
			setConfig(data.config);
		} catch (e) {
			setMessage({ text: (e as Error).message, type: "error" });
		}
	};

	// ─── PAT handler ──────────────────────────────────────────────────────

	const handleSavePat = async () => {
		if (!patValue.trim()) {
			setMessage({ text: "PAT is required", type: "error" });
			return;
		}
		try {
			const res = await fetch("/api/ado/pat", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({ pat: patValue.trim() }),
			});
			const data = await res.json();
			if (!res.ok) throw new Error(data.error || "Failed to save PAT");
			setPatStatus({ hasPat: true, source: "file" });
			setMessage({ text: "PAT saved to ~/.azure-devops-cli/pat (chmod 600)", type: "success" });
			setPatValue("");
			setShowPatModal(false);
		} catch (e) {
			setMessage({ text: (e as Error).message, type: "error" });
		}
	};

	// ─── TOML builder ─────────────────────────────────────────────────────

	const tomlString = useMemo(() => buildToml(toml), [toml]);
	const tomlCommand = useMemo(() => buildTomlCommand(tomlString), [tomlString]);
	const copyTomlCommand = async () => {
		try {
			await navigator.clipboard.writeText(tomlCommand);
			setTomlCopied(true);
			setTimeout(() => setTomlCopied(false), 2000);
		} catch {
			setMessage({ text: "Clipboard not available — copy manually", type: "error" });
		}
	};

	// ─── CLI update ───────────────────────────────────────────────────────

	const handleUpdateCli = async () => {
		setCliUpdateStatus("updating");
		setMessage(null);
		try {
			const res = await fetch("/api/ado/cli-update", { method: "POST" });
			const data: CliStatus = await res.json();
			if (!res.ok) throw new Error(data.error || "Failed to update ado CLI");
			setCliStatus(data);
			setCliUpdateStatus("idle");
			setMessage(data.updateAvailable
				? { text: "ado CLI updated, but a newer version is still available", type: "error" }
				: { text: `ado CLI updated to v${data.version}`, type: "success" });
		} catch (e) {
			setCliUpdateStatus("error");
			setMessage({ text: (e as Error).message, type: "error" });
		}
	};

	// ─── Render ───────────────────────────────────────────────────────────

	if (loading) {
		return (
			<div className="ado-config" aria-busy="true">
				<div className="ado-skel-header">
					<span className="skeleton ado-skel-title" />
					<span className="skeleton ado-skel-badge" />
				</div>
				<div className="skeleton ado-skel-pat"><span className="skeleton ado-skel-pat-line" /></div>
			</div>
		);
	}

	if (error) {
		return <div className="ado-config"><p className="ado-error">{error}</p></div>;
	}

	const hasProfiles = profileNames.length > 0;

	return (
		<div className="ado-config">
			{/* Header */}
			<div className="ado-config-header">
				<div className="ado-config-title-section">
					<h1 className="ado-config-title">Azure DevOps</h1>
					{cliStatus?.installed ? (
						<span className={`ado-config-cli-badge ${cliStatus.updateAvailable ? "outdated" : "ok"}`}>
							{cliStatus.updateAvailable && cliStatus.latest ? (
								<>
									{`ado v${cliStatus.version} → ${cliStatus.latest}`}
									<button type="button" className="ado-config-cli-update-btn" onClick={handleUpdateCli} disabled={cliUpdateStatus === "updating"}>
										{cliUpdateStatus === "updating" ? "Updating…" : "Update"}
									</button>
								</>
							) : (
								`ado v${cliStatus.version}`
							)}
						</span>
					) : (
						<span className="ado-config-cli-badge missing">CLI not installed — run ywai install</span>
					)}
				</div>
				<button className="btn btn-primary" onClick={() => setShowWizard(true)}>
					🪄 {hasProfiles ? "Guided setup" : "Get started"}
				</button>
			</div>

			{message && <div className={`ado-config-message ${message.type}`}>{message.text}</div>}

			{/* Status row — compact checks */}
			<div className="ado-config-status">
				<div className={`ado-config-check ${cliStatus?.installed ? "ok" : "missing"}`}>
					<span className="ado-config-check-dot" />
					CLI {cliStatus?.installed ? `v${cliStatus.version}` : "missing"}
				</div>
				<button className={`ado-config-check ${patStatus?.hasPat ? "ok" : "missing"}`} onClick={() => setShowPatModal(true)} title="Set PAT">
					<span className="ado-config-check-dot" />
					PAT {patStatus?.hasPat ? `(${patStatus.source === "env" ? "env" : "file"})` : "missing"}
				</button>
				<div className={`ado-config-check ${hasProfiles ? "ok" : "missing"}`}>
					<span className="ado-config-check-dot" />
					{hasProfiles ? `${profileNames.length} profile${profileNames.length === 1 ? "" : "s"}` : "no profiles"}
				</div>
			</div>

			{/* Org shown once, globally */}
			{globalOrg && (
				<div className="ado-config-org">
					<span className="muted">Organization:</span> <strong>{globalOrg}</strong>
					{orgMismatch && (
						<span className="pill pill-warning ado-config-org-warn" title="Some profiles use a different org">
							mixed orgs
						</span>
					)}
				</div>
			)}

			{/* Profiles */}
			<div className="ado-config-section-header">
				<h2 className="ado-config-section-title">Profiles</h2>
				{hasProfiles && <button className="btn btn-outline btn-sm" onClick={openAddModal}>+ Add profile</button>}
			</div>

			{!hasProfiles ? (
				<div className="ado-config-empty">
					<h3>No profiles yet</h3>
					<p>Run the guided setup to configure your org, PAT and first project — it only takes a minute.</p>
					<button className="btn btn-primary" onClick={() => setShowWizard(true)}>🪄 Start guided setup</button>
				</div>
			) : (
				<>
					<div className="ado-config-default">
						<label className="ado-config-default-label" htmlFor="ado-default">Default profile</label>
						<select
							id="ado-default"
							className="input ado-config-default-select"
							value={config?.defaultProfile ?? ""}
							onChange={(e) => handleDefaultChange(e.target.value)}
						>
							{profileNames.map((n) => <option key={n} value={n}>{n}</option>)}
						</select>
					</div>
					<div className="ado-config-profiles">
						{profileNames.map((name) => {
							const p = config!.profiles[name];
							return (
								<div key={name} className={`ado-config-card ${p.default ? "default" : ""}`}>
									<div className="ado-config-card-header">
										<span className="ado-config-card-name">{name}</span>
										{p.default && <span className="ado-config-card-default-badge">Default</span>}
										<div className="ado-config-card-actions">
											<button className="ado-config-card-btn" onClick={() => openEditModal(name)}>Edit</button>
											<button className="ado-config-card-btn danger" onClick={() => handleDeleteProfile(name)}>Delete</button>
										</div>
									</div>
									<div className="ado-config-card-info">
										<p><strong>Project:</strong> {p.project}</p>
										{p.repos && p.repos.length > 0 && (
											<div className="ado-config-card-repos">
												<strong>Repos:</strong>
												{p.repos.map((r) => <span key={r} className="ado-config-repo-pill">{r}</span>)}
											</div>
										)}
									</div>
								</div>
							);
						})}
					</div>
				</>
			)}

			{/* .adoconfig.toml builder (collapsible) */}
			<div className="ado-config-section-header">
				<h2 className="ado-config-section-title">Project rules (.adoconfig.toml)</h2>
				<button className="btn btn-outline btn-sm" onClick={() => setShowTomlBuilder(!showTomlBuilder)}>
					{showTomlBuilder ? "Hide" : "Build"}
				</button>
			</div>
			<p className="ado-config-section-desc">
				Per-repo conventions (chain strategy, branch types, PR rules). Generates a command you run inside a repo to write <code>.adoconfig.toml</code>.
			</p>

			{showTomlBuilder && (
				<div className="ado-config-toml-builder">
					<div className="ado-config-toml-form">
						<div className="field">
							<label className="field-label">Chain strategy</label>
							<select className="input" value={toml.strategy} onChange={(e) => setToml({ ...toml, strategy: e.target.value as TomlConfig["strategy"] })}>
								<option value="feature-chain">feature-chain</option>
								<option value="stacked">stacked</option>
							</select>
						</div>
						<div className="form-grid">
							<div className="field">
								<label className="field-label">Base branch</label>
								<input className="input" value={toml.baseBranch} onChange={(e) => setToml({ ...toml, baseBranch: e.target.value })} />
							</div>
							<div className="field">
								<label className="field-label">Max chain length</label>
								<input className="input" type="number" value={toml.maxLength} onChange={(e) => setToml({ ...toml, maxLength: Number(e.target.value) })} />
							</div>
							<div className="field">
								<label className="field-label">Branch prefix</label>
								<input className="input" value={toml.prefix} onChange={(e) => setToml({ ...toml, prefix: e.target.value })} />
							</div>
							<div className="field">
								<label className="field-label">Allowed branch types</label>
								<input className="input" value={toml.allowedTypes} onChange={(e) => setToml({ ...toml, allowedTypes: e.target.value })} />
							</div>
						</div>
						<div className="row" style={{ gap: "var(--space-4)", flexWrap: "wrap" }}>
							<label className="checkbox-row">
								<input type="checkbox" checked={toml.requireWorkItem} onChange={(e) => setToml({ ...toml, requireWorkItem: e.target.checked })} />
								Require work item for PRs
							</label>
							<label className="checkbox-row">
								<input type="checkbox" checked={toml.defaultDraft} onChange={(e) => setToml({ ...toml, defaultDraft: e.target.checked })} />
								PRs as draft by default
							</label>
						</div>
					</div>

					<div className="ado-config-code-block">
						<div className="ado-config-code-label">
							Run this in your repo
							<button className="btn btn-ghost btn-xs" onClick={copyTomlCommand}>
								{tomlCopied ? "✓ Copied" : "Copy"}
							</button>
						</div>
						<pre><code>{tomlCommand}</code></pre>
					</div>
				</div>
			)}

			{/* Wizard */}
			<AdoSetupWizard
				open={showWizard}
				onClose={() => setShowWizard(false)}
				onApplied={(cfg) => { setConfig(cfg); setPatStatus({ hasPat: true, source: "file" }); }}
				initialOrg={globalOrg}
				initialProfiles={config?.profiles ?? {}}
				initialDefault={config?.defaultProfile ?? ""}
				patStatus={patStatus}
				onMessage={setMessage}
			/>

			{/* Profile add/edit modal (project + repos only) */}
			<Modal
				open={showModal}
				onClose={closeModal}
				title={editingName ? `Edit profile "${editingName}"` : "Add profile"}
				subtitle={`Org inherited: ${globalOrg || "(configure org in guided setup)"}`}
				width="540px"
				footer={
					<>
						<button className="btn btn-ghost" onClick={closeModal}>Cancel</button>
						<button className="btn btn-primary" onClick={handleSaveProfile}>Save</button>
					</>
				}
			>
				<div className="field">
					<label className="field-label">Profile name</label>
					<input
						className="input"
						value={formName}
						disabled={!!editingName}
						onChange={(e) => setFormName(e.target.value)}
						placeholder="auto-derived from project if blank"
					/>
					<span className="field-hint">Left blank, it's derived from the project name (slug).</span>
				</div>
				<div className="field">
					<label className="field-label">Project</label>
					<input className="input" value={formProject} onChange={(e) => setFormProject(e.target.value)} placeholder="MyProject" />
				</div>
				<div className="field">
					<label className="field-label">Repos to monitor</label>
					<div className="ado-config-tag-input-wrapper">
						{formRepos.map((r) => (
							<span key={r} className="ado-config-repo-pill">
								{r}
								<button className="ado-config-repo-pill-remove" onClick={() => removeRepo(r)}>×</button>
							</span>
						))}
						<input
							className="ado-config-tag-input"
							value={repoInput}
							onChange={(e) => setRepoInput(e.target.value)}
							onKeyDown={handleRepoKeyDown}
							placeholder="repo name + Enter (or comma-separated)"
						/>
					</div>
				</div>
			</Modal>

			{/* PAT modal */}
			<Modal
				open={showPatModal}
				onClose={() => setShowPatModal(false)}
				title="Set Azure DevOps PAT"
				subtitle="Stored at ~/.azure-devops-cli/pat (chmod 600). Never written to opencode.json."
				width="480px"
				footer={
					<>
						<button className="btn btn-ghost" onClick={() => setShowPatModal(false)}>Cancel</button>
						<button className="btn btn-primary" onClick={handleSavePat}>Save</button>
					</>
				}
			>
				<div className="field">
					<label className="field-label">Personal Access Token</label>
					<input className="input" type="password" value={patValue} onChange={(e) => setPatValue(e.target.value)} placeholder="Paste your PAT here" />
					<span className="field-hint">Alternatively set the <code>AZURE_DEVOPS_PAT</code> env var.</span>
				</div>
			</Modal>
		</div>
	);
}
