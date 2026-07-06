import { useEffect, useMemo, useState } from "react";
import Modal from "../shared/Modal";
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

const emptyProfile = (): AdoProfile => ({
	org: "",
	project: "",
	patEnvVar: "AZURE_DEVOPS_PAT",
	repos: [],
});

// ─── .adoconfig.toml builder state ─────────────────────────────────────────

interface TomlConfig {
	strategy: "feature-chain" | "stacked";
	baseBranch: string;
	maxLength: number;
	prefix: string;
	requireWorkItem: boolean;
	defaultDraft: boolean;
	allowedTypes: string;
}

const defaultToml = (): TomlConfig => ({
	strategy: "feature-chain",
	baseBranch: "main",
	maxLength: 10,
	prefix: "feature",
	requireWorkItem: true,
	defaultDraft: true,
	allowedTypes: "feature, fix, hotfix, chore, refactor",
});

function buildToml(t: TomlConfig): string {
	const types = t.allowedTypes
		.split(",")
		.map((s) => s.trim())
		.filter(Boolean)
		.map((x) => `"${x}"`)
		.join(", ");
	return `# .adoconfig.toml — Project-level ADO conventions

[chain]
strategy = "${t.strategy}"        # "feature-chain" | "stacked"
base_branch = "${t.baseBranch}"
max_length = ${t.maxLength}
prefix = "${t.prefix}"

[branch]
allowed_types = [${types}]
slug_max_length = 40
require_wi_id = true

[pr]
require_work_item = ${t.requireWorkItem}
include_chain_context = true
review_budget = 400
default_draft = ${t.defaultDraft}

[work_item]
auto_transition = false
target_state = "In Dev"
`;
}

// Build a copy-paste shell command that writes .adoconfig.toml into the current
// repo. Heredoc keeps quoting/escaping simple across bash/zsh.
function buildTomlCommand(toml: string): string {
	return `cat > .adoconfig.toml <<'EOF'
${toml.trimEnd()}\nEOF`;
}

export default function AdoConfig() {
	const [config, setConfig] = useState<AdoConfig | null>(null);
	const [cliStatus, setCliStatus] = useState<CliStatus | null>(null);
	const [patStatus, setPatStatus] = useState<PatStatus | null>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [message, setMessage] = useState<{ text: string; type: "success" | "error" } | null>(null);

	// Profile modal state
	const [showModal, setShowModal] = useState(false);
	const [editingName, setEditingName] = useState<string | null>(null);
	const [formName, setFormName] = useState("");
	const [formProfile, setFormProfile] = useState<AdoProfile>(emptyProfile());
	const [repoInput, setRepoInput] = useState("");

	// PAT modal state
	const [showPatModal, setShowPatModal] = useState(false);
	const [patValue, setPatValue] = useState("");
	const [showTutorial, setShowTutorial] = useState(false);

	// CLI update state ("idle" | "updating" | "error")
	const [cliUpdateStatus, setCliUpdateStatus] = useState<"idle" | "updating" | "error">("idle");

	// TOML builder state
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

	// Auto-dismiss messages after 3s
	useEffect(() => {
		if (!message) return;
		const id = setTimeout(() => setMessage(null), 3000);
		return () => clearTimeout(id);
	}, [message]);

	// ─── Profile handlers ───────────────────────────────────────────────

	const openAddModal = () => {
		setEditingName(null);
		setFormName("");
		setFormProfile(emptyProfile());
		setRepoInput("");
		setShowModal(true);
	};

	const openEditModal = (name: string) => {
		const p = config?.profiles[name];
		if (!p) return;
		setEditingName(name);
		setFormName(name);
		setFormProfile({ ...p, repos: [...(p.repos ?? [])] });
		setRepoInput("");
		setShowModal(true);
	};

	const closeModal = () => setShowModal(false);

	const addRepo = () => {
		const r = repoInput.trim();
		if (!r) return;
		if (!formProfile.repos.includes(r)) {
			setFormProfile({ ...formProfile, repos: [...formProfile.repos, r] });
		}
		setRepoInput("");
	};

	const removeRepo = (r: string) => {
		setFormProfile({ ...formProfile, repos: formProfile.repos.filter((x) => x !== r) });
	};

	const handleRepoKeyDown = (e: React.KeyboardEvent) => {
		if (e.key === "Enter") {
			e.preventDefault();
			addRepo();
		}
	};

	const handleSaveProfile = async () => {
		if (!formName.trim() || !formProfile.org.trim() || !formProfile.project.trim()) {
			setMessage({ text: "Name, org and project are required", type: "error" });
			return;
		}
		try {
			const res = await fetch("/api/ado/profile", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({ name: formName.trim(), profile: formProfile }),
			});
			const data = await res.json();
			if (!res.ok) throw new Error(data.error || "Failed to save profile");
			setConfig(data.config);
			setMessage({ text: `Profile "${formName.trim()}" saved`, type: "success" });
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

	// ─── PAT handlers ───────────────────────────────────────────────────

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

	// ─── TOML builder ───────────────────────────────────────────────────

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

	const profileNames = config ? Object.keys(config.profiles) : [];

	// ─── CLI update ─────────────────────────────────────────────────────

	// Runs `npm i -g @cioffinahuel/opencode-ado` server-side. The server stays
	// up (unlike ywai self-update), so we just refresh cliStatus from the
	// response — no /health polling or page reload needed.
	const handleUpdateCli = async () => {
		setCliUpdateStatus("updating");
		setMessage(null);
		try {
			const res = await fetch("/api/ado/cli-update", { method: "POST" });
			const data: CliStatus = await res.json();
			if (!res.ok) throw new Error(data.error || "Failed to update ado CLI");
			setCliStatus(data);
			setCliUpdateStatus("idle");
			if (data.updateAvailable) {
				setMessage({ text: "ado CLI updated, but a newer version is still available", type: "error" });
			} else {
				setMessage({ text: `ado CLI updated to v${data.version}`, type: "success" });
			}
		} catch (e) {
			setCliUpdateStatus("error");
			setMessage({ text: (e as Error).message, type: "error" });
		}
	};

	// ─── Render ─────────────────────────────────────────────────────────

	if (loading) {
		return (
			<div className="ado-config" aria-busy="true">
				{/* Header skeleton */}
				<div className="ado-skel-header">
					<span className="skeleton ado-skel-title" />
					<span className="skeleton ado-skel-badge" />
				</div>
				{/* PAT bar skeleton */}
				<div className="skeleton ado-skel-pat">
					<span className="skeleton ado-skel-pat-line" />
				</div>
				{/* Profiles section header skeleton */}
				<div className="ado-skel-section-header">
					<span className="skeleton ado-skel-section-title" />
					<span className="skeleton ado-skel-add-btn" />
				</div>
				{/* Profile cards skeleton (3 cards in a grid) */}
				<div className="ado-config-profiles">
					{[0, 1, 2].map((i) => (
						<div key={i} className="skeleton ado-skel-card">
							<span className="skeleton ado-skel-line w60" />
							<span className="skeleton ado-skel-line w85" />
							<span className="skeleton ado-skel-line w40" />
						</div>
					))}
				</div>
			</div>
		);
	}

	if (error) {
		return <div className="ado-config"><p className="ado-error">{error}</p></div>;
	}

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
									<button
										type="button"
										className="ado-config-cli-update-btn"
										onClick={handleUpdateCli}
										disabled={cliUpdateStatus === "updating"}
									>
										{cliUpdateStatus === "updating" ? "Updating…" : "Update"}
									</button>
								</>
							) : (
								`ado v${cliStatus.version}`
							)}
						</span>
					) : (
						<span className="ado-config-cli-badge missing">
							CLI not installed — run ywai install
						</span>
					)}
				</div>
			</div>

			{message && (
				<div className={`ado-config-message ${message.type}`}>{message.text}</div>
			)}

			{/* PAT status */}
			<div className="ado-config-pat">
				<div className="ado-config-pat-info">
					<span className={`ado-config-pat-dot ${patStatus?.hasPat ? "ok" : "missing"}`} />
					<div>
						<strong>PAT</strong>
						{patStatus?.hasPat ? (
							<span className="ado-config-pat-source">
								{" "}✓ available ({patStatus.source === "env" ? "AZURE_DEVOPS_PAT env var" : "~/.azure-devops-cli/pat"})
							</span>
						) : (
							<span className="ado-config-pat-source missing">{" "}✗ not configured</span>
						)}
					</div>
				</div>
				<div className="ado-config-pat-actions">
					<button className="ado-config-btn" onClick={() => setShowPatModal(true)}>Set PAT</button>
					<button className="ado-config-btn" onClick={() => setShowTutorial(!showTutorial)}>
						{showTutorial ? "Hide" : "Env var"} tutorial
					</button>
				</div>
			</div>

			{/* Env var tutorial */}
			{showTutorial && (
				<div className="ado-config-tutorial">
					<h3>Set <code>AZURE_DEVOPS_PAT</code> as an environment variable</h3>
					<p>The <code>ado</code> CLI reads the PAT in this order: direct profile value → <code>AZURE_DEVOPS_PAT</code> env var → <code>~/.azure-devops-cli/pat</code> file.</p>
					<p className="ado-config-tutorial-hint">Required PAT scopes: <strong>Code</strong> (R/W), <strong>Pull Request Contribute</strong> (R/W), <strong>Work Items</strong> (Read).</p>

					<div className="ado-config-code-block">
						<div className="ado-config-code-label">macOS / Linux (zsh, bash) — add to <code>~/.zshrc</code> or <code>~/.bashrc</code></div>
						<pre><code>{`echo 'export AZURE_DEVOPS_PAT="YOUR_PAT_HERE"' >> ~/.zshrc
source ~/.zshrc`}</code></pre>
					</div>

					<div className="ado-config-code-block">
						<div className="ado-config-code-label">Windows (PowerShell) — current user, persistent</div>
						<pre><code>{`[Environment]::SetEnvironmentVariable("AZURE_DEVOPS_PAT", "YOUR_PAT_HERE", "User")
# Restart your terminal`}</code></pre>
					</div>

					<div className="ado-config-code-block">
						<div className="ado-config-code-label">Verify it works</div>
						<pre><code>{`echo $AZURE_DEVOPS_PAT   # should print your PAT
ado profile            # uses it to reach Azure DevOps`}</code></pre>
					</div>
				</div>
			)}

			{/* Default profile selector */}
			{profileNames.length > 0 && (
				<div className="ado-config-default">
					<label className="ado-config-default-label" htmlFor="ado-default">Default profile</label>
					<select
						id="ado-default"
						className="ado-config-default-select"
						value={config?.defaultProfile ?? ""}
						onChange={(e) => handleDefaultChange(e.target.value)}
					>
						{profileNames.map((n) => <option key={n} value={n}>{n}</option>)}
					</select>
				</div>
			)}

			{/* Profiles */}
			<div className="ado-config-section-header">
				<h2 className="ado-config-section-title">Profiles</h2>
				<button className="ado-config-add-btn" onClick={openAddModal}>+ Add Profile</button>
			</div>

			{profileNames.length === 0 ? (
				<div className="ado-config-empty">
					<h3>No profiles yet</h3>
					<p>An ADO profile maps to one project. Add your org, project and the repos you want to monitor.</p>
				</div>
			) : (
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
									<p><strong>Org:</strong> {p.org}</p>
									<p><strong>Project:</strong> {p.project}</p>
									{p.patEnvVar && <p><strong>PAT var:</strong> <code>{p.patEnvVar}</code></p>}
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
			)}

			{/* ─── .adoconfig.toml builder ─────────────────────────────── */}
			<div className="ado-config-section-header">
				<h2 className="ado-config-section-title">Project rules (.adoconfig.toml)</h2>
				<button className="ado-config-add-btn" onClick={() => setShowTomlBuilder(!showTomlBuilder)}>
					{showTomlBuilder ? "Hide" : "Build"}
				</button>
			</div>
			<p className="ado-config-section-desc">
				Per-repo conventions (chain strategy, branch types, PR rules). Generates a command you run inside a repo to write <code>.adoconfig.toml</code>.
			</p>

			{showTomlBuilder && (
				<div className="ado-config-toml-builder">
					<div className="ado-config-toml-form">
						<div className="ado-config-form-group">
							<label className="ado-config-form-label">Chain strategy</label>
							<select
								className="ado-config-form-input"
								value={toml.strategy}
								onChange={(e) => setToml({ ...toml, strategy: e.target.value as TomlConfig["strategy"] })}
							>
								<option value="feature-chain">feature-chain</option>
								<option value="stacked">stacked</option>
							</select>
						</div>
						<div className="ado-config-form-group">
							<label className="ado-config-form-label">Base branch</label>
							<input className="ado-config-form-input" value={toml.baseBranch} onChange={(e) => setToml({ ...toml, baseBranch: e.target.value })} />
						</div>
						<div className="ado-config-form-group">
							<label className="ado-config-form-label">Max chain length</label>
							<input className="ado-config-form-input" type="number" value={toml.maxLength} onChange={(e) => setToml({ ...toml, maxLength: Number(e.target.value) })} />
						</div>
						<div className="ado-config-form-group">
							<label className="ado-config-form-label">Branch prefix</label>
							<input className="ado-config-form-input" value={toml.prefix} onChange={(e) => setToml({ ...toml, prefix: e.target.value })} />
						</div>
						<div className="ado-config-form-group">
							<label className="ado-config-form-label">Allowed branch types (comma-separated)</label>
							<input className="ado-config-form-input" value={toml.allowedTypes} onChange={(e) => setToml({ ...toml, allowedTypes: e.target.value })} />
						</div>
						<div className="ado-config-form-group ado-config-form-row">
							<label>
								<input type="checkbox" checked={toml.requireWorkItem} onChange={(e) => setToml({ ...toml, requireWorkItem: e.target.checked })} />
								Require work item for PRs
							</label>
							<label>
								<input type="checkbox" checked={toml.defaultDraft} onChange={(e) => setToml({ ...toml, defaultDraft: e.target.checked })} />
								Create PRs as draft by default
							</label>
						</div>
					</div>

					<div className="ado-config-code-block">
						<div className="ado-config-code-label">
							Run this in your repo (writes <code>.adoconfig.toml</code>)
							<button className="ado-config-copy-btn" onClick={copyTomlCommand}>
								{tomlCopied ? "✓ Copied" : "Copy"}
							</button>
						</div>
						<pre><code>{tomlCommand}</code></pre>
					</div>
				</div>
			)}

			{/* Profile modal */}
			<Modal
				open={showModal}
				onClose={closeModal}
				title={editingName ? `Edit profile "${editingName}"` : "Add profile"}
				subtitle="An ADO profile maps to one Azure DevOps project."
				width="540px"
				footer={
					<>
						<button className="ado-config-btn" onClick={closeModal}>Cancel</button>
						<button className="ado-config-btn primary" onClick={handleSaveProfile}>Save</button>
					</>
				}
			>
				<div className="ado-config-form-group">
					<label className="ado-config-form-label">Profile name</label>
					<input
						className="ado-config-form-input"
						value={formName}
						disabled={!!editingName}
						onChange={(e) => setFormName(e.target.value)}
						placeholder="e.g. myorg-web"
					/>
					{editingName && <span className="ado-config-form-hint">Profile name cannot be changed.</span>}
				</div>
				<div className="ado-config-form-group">
					<label className="ado-config-form-label">Organization</label>
					<input
						className="ado-config-form-input"
						value={formProfile.org}
						onChange={(e) => setFormProfile({ ...formProfile, org: e.target.value })}
						placeholder="myorg, or https://dev.azure.com/myorg"
					/>
				</div>
				<div className="ado-config-form-group">
					<label className="ado-config-form-label">Project</label>
					<input
						className="ado-config-form-input"
						value={formProfile.project}
						onChange={(e) => setFormProfile({ ...formProfile, project: e.target.value })}
						placeholder="MyProject"
					/>
				</div>
				<div className="ado-config-form-group">
					<label className="ado-config-form-label">PAT env var</label>
					<input
						className="ado-config-form-input"
						value={formProfile.patEnvVar}
						onChange={(e) => setFormProfile({ ...formProfile, patEnvVar: e.target.value })}
					/>
					<span className="ado-config-form-hint">Name of the env var holding your PAT. Defaults to AZURE_DEVOPS_PAT.</span>
				</div>
				<div className="ado-config-form-group">
					<label className="ado-config-form-label">Repos to monitor</label>
					<div className="ado-config-tag-input-wrapper">
						{formProfile.repos.map((r) => (
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
							placeholder="Type a repo name and press Enter"
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
						<button className="ado-config-btn" onClick={() => setShowPatModal(false)}>Cancel</button>
						<button className="ado-config-btn primary" onClick={handleSavePat}>Save</button>
					</>
				}
			>
				<div className="ado-config-form-group">
					<label className="ado-config-form-label">Personal Access Token</label>
					<input
						className="ado-config-form-input"
						type="password"
						value={patValue}
						onChange={(e) => setPatValue(e.target.value)}
						placeholder="Paste your PAT here"
					/>
					<span className="ado-config-form-hint">
						Alternatively set the <code>AZURE_DEVOPS_PAT</code> env var (see tutorial).
					</span>
				</div>
			</Modal>
		</div>
	);
}
