import { useEffect, useMemo, useState } from "react";
import { Check } from "lucide-react";
import Modal from "../shared/Modal";
import { buildToml, buildTomlCommand, defaultToml, type TomlConfig } from "./tomlBuilder";

// ─── Types (mirror AdoConfig.tsx + the Go structs in ado_config.go) ──────

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

// A profile being edited inside the wizard (before it is named/committed).
interface DraftProfile {
	project: string;
	repos: string[];
}

interface WizardState {
	step: number;
	org: string;
	// PAT entered in step 2. Empty means "skip / reuse existing".
	pat: string;
	skipPat: boolean;
	profiles: DraftProfile[];
	defaultIndex: number;
	// Step 4 — optional .adoconfig.toml generation.
	generateToml: boolean;
	toml: TomlConfig;
}

const STEPS = [
	{ num: 1, label: "Organization" },
	{ num: 2, label: "PAT" },
	{ num: 3, label: "Profiles" },
	{ num: 4, label: "Project rules" },
	{ num: 5, label: "Review" },
];

// Slug a project name into a profile key, matching the `ado init` CLI:
// lowercase, non-alnum runs → single hyphen.
export function slugifyProject(project: string): string {
	return project
		.trim()
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, "-")
		.replace(/^-+|-+$/g, "")
		.slice(0, 50);
}

function emptyDraft(): DraftProfile {
	return { project: "", repos: [] };
}

interface Props {
	open: boolean;
	onClose: () => void;
	onApplied: (config: AdoConfig) => void;
	// Initial values seeded from the current config (so re-running the wizard
	// is an edit, not a blank slate).
	initialOrg: string;
	initialProfiles: AdoConfig["profiles"];
	initialDefault: string;
	patStatus: PatStatus | null;
	onMessage: (msg: { text: string; type: "success" | "error" }) => void;
}

export default function AdoSetupWizard({
	open,
	onClose,
	onApplied,
	initialOrg,
	initialProfiles,
	initialDefault,
	patStatus,
	onMessage,
}: Props) {
	const buildInitialState = (): WizardState => {
		const drafts: DraftProfile[] = Object.values(initialProfiles).map((p) => ({
			project: p.project,
			repos: [...(p.repos ?? [])],
		}));
		const defIdx = Math.max(
			0,
			Object.keys(initialProfiles).indexOf(initialDefault),
		);
		return {
			step: 1,
			org: initialOrg,
			pat: "",
			skipPat: !!patStatus?.hasPat,
			profiles: drafts.length > 0 ? drafts : [emptyDraft()],
			defaultIndex: defIdx,
			generateToml: false,
			toml: defaultToml(),
		};
	};

	const [state, setState] = useState<WizardState>(buildInitialState);
	const [submitting, setSubmitting] = useState(false);
	// Repo tag input per profile (by index).
	const [repoInputs, setRepoInputs] = useState<Record<number, string>>({});
	const [showEnvTutorial, setShowEnvTutorial] = useState(false);

	// Re-seed whenever the wizard is (re)opened so it reflects current config.
	useEffect(() => {
		if (open) setState(buildInitialState());
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [open]);

	const update = (patch: Partial<WizardState>) =>
		setState((s) => ({ ...s, ...patch }));

	// ─── Profile draft helpers ──────────────────────────────────────────

	const setDraft = (i: number, patch: Partial<DraftProfile>) =>
		setState((s) => {
			const profiles = s.profiles.map((p, idx) => (idx === i ? { ...p, ...patch } : p));
			return { ...s, profiles };
		});

	const addProfileDraft = () =>
		setState((s) => ({ ...s, profiles: [...s.profiles, emptyDraft()] }));

	const removeProfileDraft = (i: number) =>
		setState((s) => {
			const profiles = s.profiles.filter((_, idx) => idx !== i);
			return {
				...s,
				profiles: profiles.length ? profiles : [emptyDraft()],
				defaultIndex: Math.min(s.defaultIndex, Math.max(0, profiles.length - 1)),
			};
		});

	const addRepo = (i: number) => {
		const raw = (repoInputs[i] ?? "").trim();
		if (!raw) return;
		// Allow comma-separated paste.
		const names = raw.split(",").map((x) => x.trim()).filter(Boolean);
		const valid = names.filter((n) => /^[a-zA-Z0-9._-]+$/.test(n));
		if (valid.length !== names.length) {
			onMessage({ text: "Repo names may only contain letters, numbers, dots, hyphens, underscores", type: "error" });
		}
		setDraft(i, { repos: [...new Set([...s_profiles(i), ...valid])] });
		setRepoInputs((r) => ({ ...r, [i]: "" }));
	};
	const s_profiles = (i: number) => state.profiles[i]?.repos ?? [];

	const removeRepo = (i: number, repo: string) =>
		setDraft(i, { repos: s_profiles(i).filter((r) => r !== repo) });

	// ─── Validation per step ────────────────────────────────────────────

	const orgValid = state.org.trim().length > 0;
	// Step 2 passes if the user skipped (existing PAT) or entered a non-empty PAT.
	const patValid = state.skipPat || state.pat.trim().length > 0;
	const profilesValid =
		state.profiles.length > 0 &&
		state.profiles.every((p) => p.project.trim().length > 0) &&
		// unique slugs
		new Set(state.profiles.map((p) => slugifyProject(p.project))).size === state.profiles.length;

	const canGoNext = () => {
		switch (state.step) {
			case 1: return orgValid;
			case 2: return patValid;
			case 3: return profilesValid;
			case 4: return true;
			default: return false;
		}
	};

	const goNext = () => state.step < 5 && update({ step: state.step + 1 });
	const goBack = () => state.step > 1 && update({ step: state.step - 1 });

	// ─── Submit ─────────────────────────────────────────────────────────

	const handleApply = async () => {
		setSubmitting(true);
		try {
			// 1. Save PAT if one was entered.
			if (!state.skipPat && state.pat.trim()) {
				const res = await fetch("/api/ado/pat", {
					method: "POST",
					headers: { "Content-Type": "application/json" },
					body: JSON.stringify({ pat: state.pat.trim() }),
				});
				if (!res.ok) {
					const d = await res.json().catch(() => ({}));
					throw new Error(d.error || "Failed to save PAT");
				}
			}

			// 2. Build the config with the shared org + patEnvVar.
			const profiles: Record<string, AdoProfile> = {};
			let defaultProfile = "";
			state.profiles.forEach((p, i) => {
				const name = slugifyProject(p.project);
				const isDefault = i === state.defaultIndex;
				if (isDefault) defaultProfile = name;
				profiles[name] = {
					org: state.org.trim(),
					project: p.project.trim(),
					patEnvVar: "AZURE_DEVOPS_PAT",
					repos: p.repos,
					...(isDefault ? { default: true } : {}),
				};
			});

			const cfg: AdoConfig = { defaultProfile, profiles };
			const res = await fetch("/api/ado/config", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify(cfg),
			});
			const data = await res.json();
			if (!res.ok) throw new Error(data.error || "Failed to save config");

			onApplied(data.config);
			onMessage({ text: "Azure DevOps configuration applied", type: "success" });
			onClose();
		} catch (e) {
			onMessage({ text: (e as Error).message, type: "error" });
		} finally {
			setSubmitting(false);
		}
	};

	// ─── Render helpers ─────────────────────────────────────────────────

	const renderStepIndicator = () => (
		<div className="wizard-steps">
			{STEPS.map((s, i) => {
				const isActive = s.num === state.step;
				const isDone = s.num < state.step;
				return (
					<div key={s.num} className={`wizard-step ${isActive ? "active" : ""} ${isDone ? "done" : ""}`}>
						<div className="wizard-step-circle">
							{isDone ? <Check size={16} strokeWidth={3} /> : s.num}
						</div>
						<span className="wizard-step-label">{s.label}</span>
						{i < STEPS.length - 1 && <div className={`wizard-step-line ${isDone ? "done" : ""}`} />}
					</div>
				);
			})}
		</div>
	);

	const tomlString = useMemo(() => buildToml(state.toml), [state.toml]);
	const tomlCommand = useMemo(() => buildTomlCommand(tomlString), [tomlString]);
	const [tomlCopied, setTomlCopied] = useState(false);
	const copyToml = async () => {
		try {
			await navigator.clipboard.writeText(tomlCommand);
			setTomlCopied(true);
			setTimeout(() => setTomlCopied(false), 2000);
		} catch {
			onMessage({ text: "Clipboard not available — copy manually", type: "error" });
		}
	};

	const renderOrgStep = () => (
		<div className="wizard-step-content">
			<h3 className="wizard-step-title">Organization</h3>
			<p className="wizard-step-desc">
				Your Azure DevOps organization. Shared by all profiles — you only enter this once.
			</p>
			<div className="field">
				<label className="field-label">Organization URL or name</label>
				<input
					className="input"
					value={state.org}
					onChange={(e) => update({ org: e.target.value })}
					placeholder="myorg, or https://dev.azure.com/myorg"
					autoFocus
				/>
				<span className="field-hint">Examples: <code>myorg</code>, <code>https://dev.azure.com/myorg</code>, <code>https://myorg.visualstudio.com</code></span>
			</div>
		</div>
	);

	const renderPatStep = () => (
		<div className="wizard-step-content">
			<h3 className="wizard-step-title">Personal Access Token</h3>
			<p className="wizard-step-desc">
				Stored at <code>~/.azure-devops-cli/pat</code> (chmod 600). Never written to <code>opencode.json</code>.
			</p>

			{patStatus?.hasPat && (
				<div className="alert alert-success ado-wiz-pat-existing">
					<Check size={18} />
					<span>
						PAT already configured ({patStatus.source === "env" ? "AZURE_DEVOPS_PAT env var" : "~/.azure-devops-cli/pat"}).
					</span>
					<button type="button" className="btn btn-ghost btn-sm" onClick={() => update({ skipPat: true })}>
						{state.skipPat ? "✓ Keep existing" : "Keep existing"}
					</button>
				</div>
			)}

			<div className="field">
				<label className="field-label">
					{patStatus?.hasPat ? "Or paste a new PAT to replace it" : "Paste your PAT"}
				</label>
				<input
					className="input"
					type="password"
					value={state.pat}
					onChange={(e) => update({ pat: e.target.value, skipPat: false })}
					placeholder="Paste your PAT here"
				/>
				<span className="field-hint">Required scopes: <strong>Code</strong> (R/W), <strong>Pull Request Contribute</strong> (R/W), <strong>Work Items</strong> (Read).</span>
			</div>

			<button type="button" className="btn btn-ghost btn-sm" onClick={() => setShowEnvTutorial(!showEnvTutorial)}>
				{showEnvTutorial ? "Hide" : "Prefer an env var?"}
			</button>
			{showEnvTutorial && (
				<div className="ado-wiz-env-tutorial">
					<div className="ado-config-code-block">
						<div className="ado-config-code-label">macOS / Linux — add to <code>~/.zshrc</code></div>
						<pre><code>{`echo 'export AZURE_DEVOPS_PAT="YOUR_PAT_HERE"' >> ~/.zshrc
source ~/.zshrc`}</code></pre>
					</div>
					<div className="ado-config-code-block">
						<div className="ado-config-code-label">Windows (PowerShell)</div>
						<pre><code>{`[Environment]::SetEnvironmentVariable("AZURE_DEVOPS_PAT", "YOUR_PAT_HERE", "User")`}</code></pre>
					</div>
				</div>
			)}
		</div>
	);

	const renderProfilesStep = () => (
		<div className="wizard-step-content">
			<h3 className="wizard-step-title">Profiles</h3>
			<p className="wizard-step-desc">
				Each profile maps to one ADO project. The org (<code>{state.org || "—"}</code>) is inherited. The profile name is derived from the project.
			</p>

			{state.profiles.map((p, i) => {
				const isDefault = i === state.defaultIndex;
				const slug = slugifyProject(p.project) || "—";
				return (
					<div key={i} className={`ado-wiz-profile-card ${isDefault ? "default" : ""}`}>
						<div className="ado-wiz-profile-head">
							<label className="checkbox-row">
								<input
									type="radio"
									name="ado-default-profile"
									checked={isDefault}
									onChange={() => update({ defaultIndex: i })}
								/>
								<span className="ado-wiz-default-label">Default</span>
							</label>
							{state.profiles.length > 1 && (
								<button type="button" className="btn btn-ghost btn-xs ado-wiz-remove" onClick={() => removeProfileDraft(i)}>
									Remove
								</button>
							)}
						</div>
						<div className="field">
							<label className="field-label">Project name</label>
							<input
								className="input"
								value={p.project}
								onChange={(e) => setDraft(i, { project: e.target.value })}
								placeholder="MyProject"
							/>
							<span className="field-hint">Profile key: <code>{slug}</code></span>
						</div>
						<div className="field">
							<label className="field-label">Repos to monitor</label>
							<div className="ado-config-tag-input-wrapper">
								{p.repos.map((r) => (
									<span key={r} className="ado-config-repo-pill">
										{r}
										<button type="button" className="ado-config-repo-pill-remove" onClick={() => removeRepo(i, r)}>×</button>
									</span>
								))}
								<input
									className="ado-config-tag-input"
									value={repoInputs[i] ?? ""}
									onChange={(e) => setRepoInputs((r) => ({ ...r, [i]: e.target.value }))}
									onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addRepo(i); } }}
									placeholder="repo name + Enter (or comma-separated)"
								/>
							</div>
						</div>
					</div>
				);
			})}

			<button type="button" className="btn btn-outline btn-sm" onClick={addProfileDraft}>+ Add project</button>
		</div>
	);

	const renderTomlStep = () => (
		<div className="wizard-step-content">
			<h3 className="wizard-step-title">Project rules <span className="ado-wiz-optional">optional</span></h3>
			<p className="wizard-step-desc">
				Generate a <code>.adoconfig.toml</code> with your per-repo conventions. You'll get a command to run inside each repo.
			</p>

			<label className="checkbox-row">
				<input
					type="checkbox"
					checked={state.generateToml}
					onChange={(e) => update({ generateToml: e.target.checked })}
				/>
				Generate .adoconfig.toml
			</label>

			{state.generateToml && (
				<div className="ado-config-toml-builder">
					<div className="ado-config-toml-form">
						<div className="field">
							<label className="field-label">Chain strategy</label>
							<select
								className="input"
								value={state.toml.strategy}
								onChange={(e) => update({ toml: { ...state.toml, strategy: e.target.value as TomlConfig["strategy"] } })}
							>
								<option value="feature-chain">feature-chain</option>
								<option value="stacked">stacked</option>
							</select>
						</div>
						<div className="form-grid">
							<div className="field">
								<label className="field-label">Base branch</label>
								<input className="input" value={state.toml.baseBranch} onChange={(e) => update({ toml: { ...state.toml, baseBranch: e.target.value } })} />
							</div>
							<div className="field">
								<label className="field-label">Max chain length</label>
								<input className="input" type="number" value={state.toml.maxLength} onChange={(e) => update({ toml: { ...state.toml, maxLength: Number(e.target.value) } })} />
							</div>
							<div className="field">
								<label className="field-label">Branch prefix</label>
								<input className="input" value={state.toml.prefix} onChange={(e) => update({ toml: { ...state.toml, prefix: e.target.value } })} />
							</div>
							<div className="field">
								<label className="field-label">Allowed branch types</label>
								<input className="input" value={state.toml.allowedTypes} onChange={(e) => update({ toml: { ...state.toml, allowedTypes: e.target.value } })} />
							</div>
						</div>
						<div className="row" style={{ gap: "var(--space-4)", flexWrap: "wrap" }}>
							<label className="checkbox-row">
								<input type="checkbox" checked={state.toml.requireWorkItem} onChange={(e) => update({ toml: { ...state.toml, requireWorkItem: e.target.checked } })} />
								Require work item for PRs
							</label>
							<label className="checkbox-row">
								<input type="checkbox" checked={state.toml.defaultDraft} onChange={(e) => update({ toml: { ...state.toml, defaultDraft: e.target.checked } })} />
								PRs as draft by default
							</label>
						</div>
					</div>

					<div className="ado-config-code-block">
						<div className="ado-config-code-label">
							Run this in your repo
							<button type="button" className="btn btn-ghost btn-xs" onClick={copyToml}>
								{tomlCopied ? "✓ Copied" : "Copy"}
							</button>
						</div>
						<pre><code>{tomlCommand}</code></pre>
					</div>
				</div>
			)}
		</div>
	);

	const renderReviewStep = () => {
		const profileSummary = state.profiles.map((p, i) => ({
			name: slugifyProject(p.project),
			project: p.project,
			repos: p.repos,
			default: i === state.defaultIndex,
		}));
		return (
			<div className="wizard-step-content">
				<h3 className="wizard-step-title">Review</h3>
				<p className="wizard-step-desc">Confirm and apply. You can re-run this wizard anytime to edit.</p>

				<dl className="ado-wiz-review">
					<div><dt>Organization</dt><dd>{state.org || "—"}</dd></div>
					<div><dt>PAT</dt><dd>{state.skipPat ? "keep existing" : state.pat.trim() ? "will be saved" : "—"}</dd></div>
					<div>
						<dt>Profiles</dt>
						<dd>
							{profileSummary.map((p) => (
								<div key={p.name} className="ado-wiz-review-profile">
									<strong>{p.name}</strong>{p.default ? " (default)" : ""}
									<span className="muted"> — {p.project}: {p.repos.join(", ") || "no repos"}</span>
								</div>
							))}
						</dd>
					</div>
					<div><dt>.adoconfig.toml</dt><dd>{state.generateToml ? "command generated (copy in previous step)" : "skipped"}</dd></div>
				</dl>
			</div>
		);
	};

	const renderStepContent = () => {
		switch (state.step) {
			case 1: return renderOrgStep();
			case 2: return renderPatStep();
			case 3: return renderProfilesStep();
			case 4: return renderTomlStep();
			case 5: return renderReviewStep();
			default: return null;
		}
	};

	const stepLabel = STEPS[state.step - 1]?.label ?? "";
	const footer = (
		<>
			<button type="button" className="btn btn-ghost" onClick={onClose}>Cancel</button>
			<div className="row" style={{ gap: "var(--space-2)", marginLeft: "auto" }}>
				{state.step > 1 && (
					<button type="button" className="btn btn-ghost" onClick={goBack}>Back</button>
				)}
				{state.step < 5 ? (
					<button type="button" className="btn btn-primary" disabled={!canGoNext()} onClick={goNext}>
						Next
					</button>
				) : (
					<button type="button" className="btn btn-primary" disabled={submitting} onClick={handleApply} aria-busy={submitting || undefined}>
						{submitting ? (<><div className="spinner" /> Applying…</>) : "Apply"}
					</button>
				)}
			</div>
		</>
	);

	return (
		<Modal
			open={open}
			onClose={onClose}
			title={`Guided setup — ${stepLabel}`}
			subtitle="Configure Azure DevOps in a few steps"
			footer={footer}
			width="960px"
		>
			{renderStepIndicator()}
			{renderStepContent()}
		</Modal>
	);
}
