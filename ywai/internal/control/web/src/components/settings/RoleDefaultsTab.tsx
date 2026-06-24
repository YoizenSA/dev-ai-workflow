import { useEffect, useMemo, useState } from "react";
import { X } from "lucide-react";
import { configApi, missionsApi } from "../../api/client";
import YdSelect from "../shared/YdSelect";
import type {
	AgentInfo,
	ModelInfo,
	RoleDefault,
	RoleDefaults,
	RoleName,
	SkillInfo,
} from "../../api/types";
import { CANONICAL_ROLES } from "../../api/types";
import ModelCombobox from "../missions/ModelCombobox";

const ROLE_LABELS: Record<RoleName, string> = {
	planning: "Planning",
	architect: "Architect",
	dev: "Dev (generic)",
	frontend: "Frontend",
	backend: "Backend",
	qa: "QA",
	reviewer: "Reviewer",
	devops: "DevOps",
};

const ROLE_HINTS: Record<RoleName, string> = {
	planning: "Used to draft and refine the mission plan",
	architect: "Upfront design — structure, patterns, interfaces, trade-offs",
	dev: "Default for features without a more specific role",
	frontend: "UI / components / browser-facing code",
	backend: "Servers / APIs / data layer",
	qa: "Tests and coverage features",
	reviewer: "Code review / audit features (no edits)",
	devops: "Infra, CI/CD, deploy",
};

export default function RoleDefaultsTab() {
	const [defaults, setDefaults] = useState<RoleDefaults>({});
	const [models, setModels] = useState<ModelInfo[]>([]);
	const [agents, setAgents] = useState<string[]>([]);
	const [skills, setSkills] = useState<SkillInfo[]>([]);
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [dirty, setDirty] = useState(false);
	const [message, setMessage] = useState<string | null>(null);

	useEffect(() => {
		Promise.all([
			configApi.getUserConfig().catch(() => null),
			missionsApi.listModels().catch(() => null),
			// Use the config agents source (reads opencode.json + the ywai agents
			// dir) — the same one other screens use. The missions opencode endpoint
			// returns empty when the opencode HTTP server isn't running (e.g. on
			// Windows), which left this dropdown blank.
			configApi.listAgents().catch(() => [] as AgentInfo[]),
			configApi.listSkills().catch(() => [] as SkillInfo[]),
		])
			.then(([cfg, m, a, s]) => {
				setDefaults(cfg?.role_defaults ?? {});
				if (m) {
					const allModels = Object.values(m.modelsByProvider).flat();
					setModels(allModels);
				}
				setAgents((Array.isArray(a) ? a : []).map((x) => x.name));
				setSkills(Array.isArray(s) ? s : []);
				setLoading(false);
			})
			.catch(() => setLoading(false));
	}, []);

	const skillNames = useMemo(() => skills.map((s) => s.name), [skills]);

	const updateRole = (role: RoleName, patch: Partial<RoleDefault>) => {
		setDefaults((prev) => ({
			...prev,
			[role]: { ...(prev[role] ?? {}), ...patch },
		}));
		setDirty(true);
		setMessage(null);
	};

	const handleSave = async () => {
		setSaving(true);
		setMessage(null);
		try {
			await configApi.updateUserConfig({ role_defaults: defaults });
			setMessage("Role defaults saved");
			setDirty(false);
		} catch (err) {
			setMessage(`Error: ${err}`);
		} finally {
			setSaving(false);
		}
	};

	if (loading) {
		return (
			<div aria-busy="true" className="skeleton skel-card" style={{ margin: 'var(--space-4)' }}>
				<div className="skel-line title" />
				<div className="skel-line desc" />
				<div className="skel-line desc sm" />
			</div>
		);
	}

	return (
		<div className="card card-pad">
			<p className="muted" style={{ marginBottom: "var(--space-4)" }}>
				The mission planner classifies each feature by role. At execution time
				the worker uses the role's default model and tries the fallbacks if the
				primary fails. Agents and skills come from your OpenCode config.
			</p>

			{CANONICAL_ROLES.map((role) => {
				const rd = defaults[role] ?? {};
				return (
					<div
						key={role}
						className="settings-item"
						style={{ marginBottom: "var(--space-4)" }}
					>
						<div
							style={{
								display: "flex",
								alignItems: "baseline",
								justifyContent: "space-between",
								marginBottom: "var(--space-2)",
							}}
						>
							<h3 style={{ margin: 0 }}>{ROLE_LABELS[role]}</h3>
							<span className="muted" style={{ fontSize: "0.78rem" }}>
								{ROLE_HINTS[role]}
							</span>
						</div>

						<div className="form-grid">
							<div className="field">
								<label className="field-label" htmlFor={`role-${role}-agent`}>
									Agent
								</label>
								<YdSelect
									options={agents.map((name) => ({ value: name, label: name }))}
									value={rd.agent ?? ""}
									onChange={(v) => updateRole(role, { agent: v })}
									placeholder="— pick agent —"
								/>
							</div>

							<div className="field">
								<ModelCombobox
									id={`role-${role}-model`}
									label="Primary model"
									value={rd.model ?? ""}
									models={models}
									onChange={(v) => updateRole(role, { model: v })}
								/>
							</div>

							<div className="field span-2">
								<label className="field-label">Fallback models (ordered)</label>
								<FallbackChips
									value={rd.fallbacks ?? []}
									models={models}
									onChange={(v) => updateRole(role, { fallbacks: v })}
								/>
								<span className="field-hint">
									Tried in order if the primary model returns a retriable error.
									Max 2 — capped at 3 attempts including primary.
								</span>
							</div>

							<div className="field span-2">
								<label className="field-label">Skills</label>
								<SkillPicker
									value={rd.skills ?? []}
									available={skillNames}
									onChange={(v) => updateRole(role, { skills: v })}
								/>
							</div>
						</div>
					</div>
				);
			})}

			{message && (
				<div
					className={`alert ${message.startsWith("Error") ? "alert-danger" : "alert-success"}`}
				>
					{message}
				</div>
			)}

			<button
				className="btn btn-primary"
				onClick={handleSave}
				disabled={saving || !dirty}
				aria-busy={saving || undefined}
				style={{ marginTop: "var(--space-3)" }}
			>
				{saving ? (
					<>
						<div className="spinner"></div>
						Saving…
					</>
				) : (
					"Save Changes"
				)}
			</button>
		</div>
	);
}

// ─── Helper components ────────────────────────────────────────────────────

interface FallbackChipsProps {
	value: string[];
	models: ModelInfo[];
	onChange: (next: string[]) => void;
}

function FallbackChips({ value, models, onChange }: FallbackChipsProps) {
	const [drafting, setDrafting] = useState(false);
	const [draftValue, setDraftValue] = useState("");

	const remove = (idx: number) => {
		const next = value.slice();
		next.splice(idx, 1);
		onChange(next);
	};

	const moveUp = (idx: number) => {
		if (idx === 0) return;
		const next = value.slice();
		[next[idx - 1], next[idx]] = [next[idx], next[idx - 1]];
		onChange(next);
	};

	const moveDown = (idx: number) => {
		if (idx === value.length - 1) return;
		const next = value.slice();
		[next[idx], next[idx + 1]] = [next[idx + 1], next[idx]];
		onChange(next);
	};

	const addDraft = () => {
		if (!draftValue.trim()) {
			setDrafting(false);
			return;
		}
		onChange([...value, draftValue.trim()]);
		setDraftValue("");
		setDrafting(false);
	};

	return (
		<div style={{ display: "flex", flexDirection: "column", gap: "var(--space-2)" }}>
			{value.map((m, idx) => (
				<div
					key={`${m}-${idx}`}
					style={{
						display: "flex",
						alignItems: "center",
						gap: "var(--space-2)",
						padding: "6px 10px",
						background: "var(--surface-soft)",
						borderRadius: 6,
						fontSize: "0.85rem",
					}}
				>
					<span style={{ flex: 1, fontFamily: "var(--font-mono)" }}>{m}</span>
					<button
						type="button"
						className="btn btn-sm btn-ghost"
						onClick={() => moveUp(idx)}
						disabled={idx === 0}
						aria-label="Move up"
					>
						↑
					</button>
					<button
						type="button"
						className="btn btn-sm btn-ghost"
						onClick={() => moveDown(idx)}
						disabled={idx === value.length - 1}
						aria-label="Move down"
					>
						↓
					</button>
					<button
						type="button"
						className="btn btn-sm btn-danger"
						onClick={() => remove(idx)}
						aria-label="Remove"
					>
						<X size={16} />
					</button>
				</div>
			))}

			{drafting ? (
				<div style={{ display: "flex", gap: "var(--space-2)" }}>
					<input
						className="input"
						list="all-models-list"
						value={draftValue}
						onChange={(e) => setDraftValue(e.target.value)}
						placeholder="provider/model-id"
						autoFocus
					/>
					<datalist id="all-models-list">
						{models.map((m) => (
							<option key={m.id} value={m.id}>
								{m.name || m.id}
							</option>
						))}
					</datalist>
					<button
						type="button"
						className="btn btn-sm btn-primary"
						onClick={addDraft}
					>
						Add
					</button>
					<button
						type="button"
						className="btn btn-sm btn-ghost"
						onClick={() => {
							setDrafting(false);
							setDraftValue("");
						}}
					>
						Cancel
					</button>
				</div>
			) : (
				<button
					type="button"
					className="btn btn-sm"
					onClick={() => setDrafting(true)}
					disabled={value.length >= 2}
				>
					+ Add fallback
				</button>
			)}
		</div>
	);
}

interface SkillPickerProps {
	value: string[];
	available: string[];
	onChange: (next: string[]) => void;
}

function SkillPicker({ value, available, onChange }: SkillPickerProps) {
	const toggle = (name: string) => {
		if (value.includes(name)) {
			onChange(value.filter((v) => v !== name));
		} else {
			onChange([...value, name]);
		}
	};
	return (
		<div style={{ display: "flex", flexWrap: "wrap", gap: "var(--space-2)" }}>
			{available.length === 0 && (
				<span className="muted" style={{ fontSize: "0.85rem" }}>
					No skills available
				</span>
			)}
			{available.map((name) => {
				const active = value.includes(name);
				return (
					<button
						type="button"
						key={name}
						className={`pill ${active ? "pill-success" : "pill-muted"}`}
						style={{ cursor: "pointer" }}
						onClick={() => toggle(name)}
					>
						<span className="dot" />
						{name}
					</button>
				);
			})}
		</div>
	);
}
