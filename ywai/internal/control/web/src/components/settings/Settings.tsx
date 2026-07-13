import { useCallback, useEffect, useState } from "react";
import {
	Activity,
	Bell,
	Book,
	Check,
	Monitor,
	Package,
	Plug,
	RefreshCw,
	Server,
	Settings as SettingsIcon,
	Share2,
	Star,
	Trash2,
	User,
	Wrench,
} from "lucide-react";
import { useUrlTab } from "../../hooks/useUrlTab";
import { configApi, missionsApi } from "../../api/client";
import type {
	MCPServer,
	ModelInfo,
	ProviderInfo,
	OpenCodeConfig as OpenCodeConfigType,
} from "../../api/types";
import RoleDefaultsTab from "./RoleDefaultsTab";
import ReferencesTab from "./ReferencesTab";
import OrchestratorTab from "./OrchestratorTab";
import ProfilesTab from "./ProfilesTab";
import { NotificationsTab } from "./NotificationsTab";
import SearchSelect from "../shared/SearchSelect";
import ModelCombobox from "../missions/ModelCombobox";
import Modal from "../shared/Modal";
import "./Settings.css";

type Tab =
	| "general"
	| "roles"
	| "agents"
	| "orchestrator"
	| "skills"
	| "mcp"
	| "providers"
	| "tools"
	| "references"
	| "profiles"
	| "notifications";

const TABS: { id: Tab; label: string; icon: React.ReactNode }[] = [
	{
		id: "general",
		label: "General",
		icon: <SettingsIcon size={16} />,
	},
	// ponytail: role defaults hidden — configure per-agent instead
	// {
	// 	id: "roles",
	// 	label: "Role Defaults",
	// 	icon: <Users size={16} />,
	// },
	{
		id: "agents",
		label: "Agents",
		icon: <User size={16} />,
	},
	{
		id: "orchestrator",
		label: "Orchestrator",
		icon: <Share2 size={16} />,
	},
	{
		id: "profiles",
		label: "Profiles",
		icon: <Server size={16} />,
	},
	{
		id: "skills",
		label: "Skills",
		icon: <Star size={16} />,
	},
	{
		id: "notifications",
		label: "Notifications",
		icon: <Bell size={16} />,
	},
	{
		id: "mcp",
		label: "MCP",
		icon: <Monitor size={16} />,
	},
	{
		id: "providers",
		label: "Providers",
		icon: <Activity size={16} />,
	},
	{
		id: "tools",
		label: "Tools",
		icon: <Wrench size={16} />,
	},
	{
		id: "references",
		label: "References",
		icon: <Book size={16} />,
	},
];

const TAB_IDS = TABS.map((t) => t.id);

export default function Settings() {
	const [activeTab, setActiveTab] = useUrlTab<Tab>("general", TAB_IDS);

	return (
		<div className="settings-page">
			<header className="page-header">
				<div className="page-heading">
					<span className="page-eyebrow">Configuration</span>
					<h1 className="page-title">Settings</h1>
					<p className="page-subtitle">
						Manage OpenCode configuration, agents, skills, and integrations
					</p>
				</div>
			</header>

			<div className="tabs">
				{TABS.map((tab) => (
					<button
						key={tab.id}
						className={`tab ${activeTab === tab.id ? "active" : ""}`}
						onClick={() => setActiveTab(tab.id)}
					>
						<span className="tab-icon">{tab.icon}</span>
						{tab.label}
					</button>
				))}
			</div>

			<div className="tab-content">
				{activeTab === "general" && <GeneralTab />}
				{activeTab === "roles" && <RoleDefaultsTab />}
				{activeTab === "agents" && <AgentsTab />}
				{activeTab === "orchestrator" && <OrchestratorTab />}
				{activeTab === "profiles" && <ProfilesTab />}
				{activeTab === "skills" && <SkillsTab />}
				{activeTab === "notifications" && <NotificationsTab />}
				{activeTab === "mcp" && <MCPTab />}
				{activeTab === "providers" && <ProvidersTab />}
				{activeTab === "tools" && <ToolsTab />}
				{activeTab === "references" && <ReferencesTab />}
			</div>
		</div>
	);
}

// ─── General Tab ───────────────────────────────────────────────────────────

function GeneralTab() {
	const [config, setConfig] = useState<OpenCodeConfigType | null>(null);
	const [visionModel, setVisionModel] = useState("");
	const [agentList, setAgentList] = useState<string[]>([]);

	const [models, setModels] = useState<ModelInfo[]>([]);
	const [visionModels, setVisionModels] = useState<ModelInfo[]>([]);
	const [visionModelsError, setVisionModelsError] = useState<string | null>(null);
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [message, setMessage] = useState<string | null>(null);
	// AGENTS.md state
	const [agentsMd, setAgentsMd] = useState<string>("");
	const [agentsMdPath, setAgentsMdPath] = useState<string>("");
	const [agentsMdLoading, setAgentsMdLoading] = useState(true);
	const [agentsMdSaving, setAgentsMdSaving] = useState(false);
	const [agentsMdMessage, setAgentsMdMessage] = useState<string | null>(null);
	// SDD cleanup state
	const [sddStatus, setSddStatus] = useState<{ agents: { name: string; count: number }[]; total: number } | null>(null);
	const [sddLoading, setSddLoading] = useState(true);
	const [sddRemoving, setSddRemoving] = useState(false);
	const [sddMessage, setSddMessage] = useState<string | null>(null);

	useEffect(() => {
		// listProviders() only returns providers declared in opencode.json (3-ish).
		// missionsApi.listModels() asks opencode CLI which knows the full runtime
		// list (github-copilot, openai, opencode, etc — 6+). Union both so the
		// datalist offers everything the user can actually pick.
		Promise.all([
			configApi.getConfig().catch(() => null),
			configApi.listAgents().catch(() => [] as { name: string }[]),
			missionsApi.listModels().catch(() => null),
			configApi.getUserConfig().catch(() => null),
			configApi.listVisionModels().catch((e: Error) => ({
				models: [] as Array<{ id: string; name: string }>,
				current: undefined as string | undefined,
				error: e?.message ?? "failed to load vision models",
			})),
		]).then(([cfg, agents, modelsRes, userCfg, visionRes]) => {
			if (cfg) {
				// opencode.json stores these keys in snake_case; the UI reads
				// camelCase. Bridge them on load so saved values reappear.
				const raw = cfg as Record<string, unknown>;
				setConfig({
					...cfg,
					smallModel: cfg.smallModel ?? (raw.small_model as string | undefined),
					defaultAgent:
						cfg.defaultAgent ?? (raw.default_agent as string | undefined),
				});
			}
			// vision_model lives in ~/.ywai/config.yaml (user config), not opencode.json
			const preferred =
				userCfg?.vision_model_override ||
				userCfg?.vision_model ||
				("current" in (visionRes ?? {}) ? visionRes?.current : undefined) ||
				"";
			setVisionModel(preferred ?? "");
			setAgentList((agents ?? []).map((a) => a.name));
			setModels(
				modelsRes
					? Object.values(modelsRes.modelsByProvider ?? {}).flat()
					: [],
			);
			const vModels = (visionRes?.models ?? []).map((m) => ({
				id: m.id,
				name: m.name || m.id,
				provider: "tokenbank",
			}));
			setVisionModels(vModels);
			setVisionModelsError(visionRes?.error ?? null);
			setLoading(false);
		});

		// Load AGENTS.md
		configApi
			.getAgentsMd()
			.then(({ path, content }) => {
				setAgentsMd(content);
				setAgentsMdPath(path);
			})
			.catch(() => {
				setAgentsMd("");
				setAgentsMdPath("");
			})
			.finally(() => setAgentsMdLoading(false));

		// Load SDD asset status (count per agent)
		configApi
			.getSddStatus()
			.then(setSddStatus)
			.catch(() => setSddStatus({ agents: [], total: 0 }))
			.finally(() => setSddLoading(false));
	}, []);

	// Helper to read the current provider / agent identifier no matter whether
	// the upstream config stores it as a flat string or as an object keyed by
	// id (the opencode.json shape uses both depending on which CLI wrote it).
	const readKey = (val: unknown): string => {
		if (typeof val === "string") return val;
		if (val && typeof val === "object") {
			const k = Object.keys(val)[0];
			return k ?? "";
		}
		return "";
	};

	const handleSave = async () => {
		if (!config) return;
		setSaving(true);
		setMessage(null);
		try {
			// Send every editable field; the backend merges into the existing
			// opencode.json instead of overwriting it, so we don't clobber
			// fields we don't render.
			const toSave: Record<string, unknown> = {};
			if (config.model) toSave.model = config.model;
			if (config.smallModel) toSave["small_model"] = config.smallModel;
			if (config.defaultAgent) toSave["default_agent"] = config.defaultAgent;
			if (config.maxTokens !== undefined) toSave.maxTokens = config.maxTokens;
			if (config.temperature !== undefined)
				toSave.temperature = config.temperature;
			await configApi.updateConfig(toSave as OpenCodeConfigType);

			// Vision model for vision-bridge plugin (TokenBank model id)
			await configApi.updateUserConfig({
				vision_model: visionModel || "",
				// Clear override so Settings is the single source of truth
				vision_model_override: "",
			});

			setMessage("Saved successfully");
		} catch (err) {
			setMessage(`Error: ${err}`);
		} finally {
			setSaving(false);
		}
	};

	const handleAgentsMdSave = async () => {
		if (!agentsMdPath) return;
		setAgentsMdSaving(true);
		setAgentsMdMessage(null);
		try {
			await configApi.saveAgentsMd(agentsMd);
			setAgentsMdMessage("AGENTS.md saved successfully");
		} catch (err) {
			setAgentsMdMessage(`Error: ${err}`);
		} finally {
			setAgentsMdSaving(false);
		}
	};

	const handleRemoveSdd = async () => {
		if (!sddStatus || sddStatus.total === 0) return;
		const ok = confirm(
			`Se van a borrar ${sddStatus.total} archivos SDD de ${sddStatus.agents.length} agente(s).\nSDD dejará de estar disponible en esos agentes. ¿Continuar?`,
		);
		if (!ok) return;
		setSddRemoving(true);
		setSddMessage(null);
		try {
			const res = await configApi.removeSdd();
			const errCount = res.errors?.length ?? 0;
			setSddMessage(
				errCount > 0
					? `Error: ${errCount} agente(s) con fallos: ${res.errors!.join(", ")}`
					: `Eliminados ${res.total} assets SDD de ${res.agents} agente(s).`,
			);
			// Refresh status
			const fresh = await configApi.getSddStatus();
			setSddStatus(fresh);
		} catch (err) {
			setSddMessage(`Error: ${err}`);
		} finally {
			setSddRemoving(false);
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

	if (!config) return <p className="muted">No config available</p>;

	return (
		<div>
			<div className="card card-pad">
				<div className="form-grid">
				<ModelCombobox
					id="cfg-model"
					label="Model"
					value={config.model ?? ""}
					models={models}
					onChange={(v) => setConfig({ ...config, model: v })}
				/>
				<ModelCombobox
					id="cfg-small-model"
					label="Small Model"
					value={config.smallModel ?? ""}
					models={models}
					onChange={(v) => setConfig({ ...config, smallModel: v })}
				/>
				<div className="field span-2">
					<label className="field-label" htmlFor="cfg-default-agent">
						Default Agent
					</label>
					<span className="field-hint">
						The default agent for new conversations
					</span>
					<SearchSelect
						id="cfg-default-agent"
						value={readKey(config.defaultAgent)}
						options={agentList}
						placeholder="e.g., orchestrator"
						onChange={(v) => setConfig({ ...config, defaultAgent: v })}
					/>
				</div>
				<div className="field">
					<label className="field-label" htmlFor="cfg-tokens">
						Max Tokens
					</label>
					<span className="field-hint">Maximum tokens for model responses</span>
					<input
						id="cfg-tokens"
						className="input"
						type="number"
						placeholder="e.g., 8192"
						value={config.maxTokens ?? ""}
						onChange={(e) =>
							setConfig({
								...config,
								maxTokens: e.target.value ? Number(e.target.value) : undefined,
							})
						}
					/>
				</div>
				<div className="field">
					<label className="field-label" htmlFor="cfg-temp">
						Temperature
					</label>
					<span className="field-hint">
						Controls randomness: 0=focused, 2=creative
					</span>
					<input
						id="cfg-temp"
						className="input"
						type="number"
						step="0.1"
						min="0"
						max="2"
						placeholder="e.g., 0.7"
						value={config.temperature ?? ""}
						onChange={(e) =>
							setConfig({
								...config,
								temperature: e.target.value
									? Number(e.target.value)
									: undefined,
							})
						}
					/>
				</div>
			</div>

			{/* ─── Vision bridge ──────────────────────────────────────── */}
			<div className="field span-2" style={{ borderTop: "1px solid var(--panel-border)", paddingTop: "var(--space-4)", marginTop: "var(--space-2)" }}>
				<span className="field-label" style={{ fontSize: "0.9rem", fontWeight: 600 }}>
					Vision bridge
				</span>
				<span className="field-hint" style={{ display: "block", marginTop: "0.25rem" }}>
					When the chat model cannot see images (e.g. DeepSeek), the vision-bridge
					plugin analyzes attached images with this TokenBank model and injects the
					text for the chat model.
				</span>
			</div>

			<div className="field span-2">
				<ModelCombobox
					id="cfg-vision-model"
					label="Vision model"
					value={visionModel}
					models={visionModels.length > 0 ? visionModels : models}
					onChange={(v) => setVisionModel(v)}
				/>
				{visionModelsError && (
					<span className="field-hint" style={{ color: "var(--danger, #c44)", display: "block", marginTop: "0.35rem" }}>
						{visionModelsError}
					</span>
				)}
				{!visionModelsError && visionModels.length === 0 && (
					<span className="field-hint" style={{ display: "block", marginTop: "0.35rem" }}>
						No vision models loaded. Configure TokenBank or type a model id.
					</span>
				)}
				<button
					type="button"
					className="btn btn-ghost"
					style={{ marginTop: "0.5rem" }}
					onClick={() => setVisionModel("")}
					disabled={!visionModel}
				>
					Use catalog default
				</button>
			</div>

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
				disabled={saving}
				aria-busy={saving || undefined}
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

			{/* ─── AGENTS.md Editor ────────────────────────────────────────── */}
			<div className="card card-pad" style={{ marginTop: "2rem" }}>
				<div className="card-header">
					<h3>AGENTS.md</h3>
					<span className="muted">{agentsMdPath || "Not found"}</span>
				</div>
				<p className="muted" style={{ marginBottom: "1rem" }}>
					Edit the AGENTS.md file from the project root. This file contains
					project-wide instructions for AI agents.
				</p>
				{agentsMdLoading ? (
					<div className="loading-inline">
						<div className="spinner"></div>
						<span>Loading AGENTS.md…</span>
					</div>
				) : agentsMdPath ? (
					<>
						<textarea
							className="input"
							rows={20}
							value={agentsMd}
							onChange={(e) => setAgentsMd(e.target.value)}
							placeholder="# AGENTS.md content..."
							style={{ fontFamily: "monospace", resize: "vertical" }}
						/>
						{agentsMdMessage && (
							<div
								className={`alert ${agentsMdMessage.startsWith("Error") ? "alert-danger" : "alert-success"}`}
							>
								{agentsMdMessage}
							</div>
						)}
						<button
							className="btn btn-primary"
							onClick={handleAgentsMdSave}
							disabled={agentsMdSaving}
							aria-busy={agentsMdSaving || undefined}
							style={{ marginTop: "0.5rem" }}
						>
							{agentsMdSaving ? (
								<>
									<div className="spinner"></div>
									Saving…
								</>
							) : (
								"Save AGENTS.md"
							)}
						</button>
					</>
				) : (
					<p className="muted">AGENTS.md not found in the project root.</p>
				)}
				</div>
			</div>

			{/* ─── SDD Cleanup ───────────────────────────────────────────── */}
			<div className="card card-pad" style={{ marginTop: "2rem" }}>
				<div className="card-header">
					<h3>Spec-Driven Development (SDD)</h3>
					<span className="muted">
						{sddLoading
							? "Cargando…"
							: sddStatus && sddStatus.total > 0
								? `${sddStatus.total} archivos en ${sddStatus.agents.length} agente(s)`
								: "Sin assets SDD"}
					</span>
				</div>
				<p className="muted" style={{ marginBottom: "1rem" }}>
					Los assets de SDD (skills, comandos y agentes) que gentle-ai sync escribió
					en los directorios de los agentes. Eliminarlos desactiva SDD en esos agentes;
					como ywai ya no ejecuta el sync, no vuelven a aparecer.
				</p>
				{sddMessage && (
					<div
						className={`alert ${sddMessage.startsWith("Error") ? "alert-danger" : "alert-success"}`}
					>
						{sddMessage}
					</div>
				)}
				<button
					type="button"
					className="btn btn-danger"
					onClick={handleRemoveSdd}
					disabled={sddRemoving || sddLoading || !sddStatus || sddStatus.total === 0}
					aria-busy={sddRemoving || undefined}
				>
					{sddRemoving ? (
						<>
							<div className="spinner"></div>
							Eliminando…
						</>
					) : (
						<>
							<Trash2 size={14} />
							Eliminar assets de SDD
						</>
					)}
				</button>
			</div>
		</div>
		);
	}

// ─── Agents Tab ────────────────────────────────────────────────────────────

function AgentsTab() {
	const [agents, setAgents] = useState<
		{ name: string; content?: string; group?: string; mode?: string }[]
	>([]);
	const [loading, setLoading] = useState(true);
	const [selected, setSelected] = useState<string | null>(null);
	const [editContent, setEditContent] = useState("");
	const [permissions, setPermissions] = useState<
		Record<string, Record<string, string>>
	>({});
	// Per-subagent task delegation maps (permission.task), keyed by agent name.
	// Only meaningful for primary agents (orchestrators).
	const [taskPerms, setTaskPerms] = useState<
		Record<string, Record<string, string>>
	>({});
	const [savingTaskPerms, setSavingTaskPerms] = useState<string | null>(null);
	// Per-agent default model (agent.<name>.model), keyed by agent name.
	const [agentModels, setAgentModels] = useState<Record<string, string>>({});
	const [models, setModels] = useState<ModelInfo[]>([]);
	const [savingModel, setSavingModel] = useState<string | null>(null);
	const [toolsData, setToolsData] = useState<{
		built_in: string[];
		mcp_tools: Record<string, { tools: string[]; enabled: boolean }>;
		plugin_tools: Record<string, string[]>;
	}>({ built_in: [], mcp_tools: {}, plugin_tools: {} });
	const [saving, setSaving] = useState(false);
	const [savingPerms, setSavingPerms] = useState<string | null>(null);
	const [message, setMessage] = useState<string | null>(null);
	const [searchQuery, setSearchQuery] = useState("");

	// Load the agent list + tools on mount. We intentionally do NOT fetch each
	// agent's full content here — doing so fired one HTTP request per agent and
	// blocked the first paint until every one resolved (the "tarda un siglo"
	// problem). Content is lazy-loaded when an agent is selected (effect below).
	useEffect(() => {
		Promise.all([
			configApi.listAgents(),
			configApi
				.listTools()
				.catch(() => ({
					built_in: [] as string[],
					all: [] as string[],
					mcp_tools: {} as Record<
						string,
						{ tools: string[]; enabled: boolean }
					>,
					plugin_tools: {} as Record<string, string[]>,
				})),
		])
			.then(([agentList, toolsRes]) => {
				const list = (agentList ?? []).map((a) => ({
					name: a.name,
					content: undefined as string | undefined,
					group: a.group,
					mode: a.mode,
				}));
				setAgents(list);
				setToolsData({
					built_in: toolsRes.built_in ?? [],
					mcp_tools: toolsRes.mcp_tools ?? {},
					plugin_tools: toolsRes.plugin_tools ?? {},
				});
				if (list.length > 0) {
					setSelected(list[0].name);
				}
				setLoading(false);
			})
			.catch(() => setLoading(false));
	}, []);

	// Lazy-load the selected agent's content the first time it's opened.
	const loadAgentContent = useCallback(
		(name: string, updateEditor: boolean) => {
			configApi
				.getAgent(name)
				.then((d) => {
					setAgents((prev) =>
						prev.map((a) =>
							a.name === name ? { ...a, content: d.content } : a,
						),
					);
					if (updateEditor) {
						setEditContent(d.content ?? "");
					}
				})
				.catch(() => {});
		},
		[],
	);

	useEffect(() => {
		if (!selected) return;
		const agent = agents.find((a) => a.name === selected);
		if (!agent) return;
		if (agent.content !== undefined) {
			setEditContent(agent.content);
			return;
		}
		loadAgentContent(selected, true);
	}, [selected, agents, loadAgentContent]);

	// Load permissions when agent selected
	useEffect(() => {
		if (!selected) return;
		if (!permissions[selected]) {
			configApi
				.getAgentPermissions(selected)
				.then((perms) => {
					setPermissions((prev) => ({ ...prev, [selected]: perms }));
				})
				.catch(() => {});
		}
	}, [selected]);

	// Load the task delegation map for primary agents when selected.
	useEffect(() => {
		if (!selected) return;
		const agent = agents.find((a) => a.name === selected);
		if (agent?.mode !== "primary") return;
		if (taskPerms[selected]) return;
		configApi
			.getAgentTaskPermissions(selected)
			.then((perms) => {
				setTaskPerms((prev) => ({ ...prev, [selected]: perms }));
			})
			.catch(() => {});
	}, [selected, agents]);

	// Load the available model catalog once for the model picker.
	useEffect(() => {
		missionsApi
			.listModels()
			.then((m) => {
				if (m?.modelsByProvider) {
					setModels(Object.values(m.modelsByProvider).flat());
				}
			})
			.catch(() => {});
	}, []);

	// Load the selected agent's default model.
	useEffect(() => {
		if (!selected) return;
		if (agentModels[selected] !== undefined) return;
		configApi
			.getAgentModel(selected)
			.then((res) => {
				setAgentModels((prev) => ({ ...prev, [selected]: res.model ?? "" }));
			})
			.catch(() => {});
	}, [selected]);

	const handleChangeModel = async (agentName: string, model: string) => {
		setSavingModel(agentName);
		try {
			await configApi.updateAgentModel(agentName, model);
			setAgentModels((prev) => ({ ...prev, [agentName]: model }));
			setMessage(model ? `Model set to ${model}` : "Model reset to default");
		} catch (err) {
			setMessage(`Error saving model: ${err}`);
		} finally {
			setSavingModel(null);
		}
	};

	const handleSelectAgent = (name: string) => {
		setSelected(name);
		const agent = agents.find((a) => a.name === name);
		setEditContent(agent?.content ?? "");
		setMessage(null);
	};

	const handleSavePrompt = async () => {
		if (!selected) return;
		setSaving(true);
		setMessage(null);
		try {
			await configApi.updateAgent(selected, editContent);
			setAgents((prev) =>
				prev.map((a) =>
					a.name === selected ? { ...a, content: editContent } : a,
				),
			);
			setMessage("Agent prompt saved successfully");
		} catch (err) {
			setMessage(`Error: ${err}`);
		} finally {
			setSaving(false);
		}
	};

	const handleCreateAgent = async () => {
		const name = prompt("New agent name:");
		if (!name || !name.trim()) return;
		const clean = name.trim().toLowerCase().replace(/\s+/g, "-");
		setSaving(true);
		try {
			await configApi.createAgent(
				clean,
				`# ${clean}\n\nYou are a helpful assistant.`,
			);
			const detail = await configApi.getAgent(clean);
			setAgents((prev) => [
				...prev,
				{ name: clean, content: detail.content, group: "custom" },
			]);
			setSelected(clean);
			setEditContent(detail.content ?? "");
			setMessage("Agent created");
		} catch (err) {
			setMessage(`Error: ${err}`);
		} finally {
			setSaving(false);
		}
	};

	const handleDeleteAgent = async () => {
		if (!selected) return;
		if (!confirm(`Delete agent "${selected}"?`)) return;
		setSaving(true);
		try {
			await configApi.deleteAgent(selected);
			setAgents((prev) => prev.filter((a) => a.name !== selected));
			const remaining = agents.filter((a) => a.name !== selected);
			if (remaining.length > 0) {
				setSelected(remaining[0].name);
				setEditContent(remaining[0].content ?? "");
			} else {
				setSelected(null);
				setEditContent("");
			}
			setMessage("Agent deleted");
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
				<div className="skel-line tag" />
			</div>
		);
	}

	// Group agents
	const grouped: Record<string, typeof agents> = {};
	for (const agent of agents) {
		const team = agent.group || "other";
		if (!grouped[team]) grouped[team] = [];
		grouped[team].push(agent);
	}
	const teamOrder = [
		"core",
		"qa-automation",
		"social-refactor",
		"experiment",
		"custom",
		"other",
	];
	const sortedTeams = Object.keys(grouped).sort((a, b) => {
		const ai = teamOrder.indexOf(a);
		const bi = teamOrder.indexOf(b);
		if (ai !== -1 && bi !== -1) return ai - bi;
		if (ai !== -1) return -1;
		if (bi !== -1) return 1;
		return a.localeCompare(b);
	});

	// Filter by search
	const filterAgents = (list: typeof agents) => {
		if (!searchQuery) return list;
		const q = searchQuery.toLowerCase();
		return list.filter((a) => a.name.toLowerCase().includes(q));
	};

	const selectedAgent = agents.find((a) => a.name === selected);
	const currentPerms = selected ? (permissions[selected] ?? {}) : {};

	// --- Permission rendering (dynamic from API) ---
	const togglePermission = async (agentName: string, tool: string) => {
		const perms = { ...(permissions[agentName] ?? {}) };
		perms[tool] = perms[tool] === "allow" ? "deny" : "allow";
		setSavingPerms(agentName);
		try {
			await configApi.updateAgentPermissions(agentName, perms);
			setPermissions((prev) => ({ ...prev, [agentName]: perms }));
			loadAgentContent(agentName, agentName === selected);
			setMessage("Permissions saved");
		} catch (err) {
			setMessage(`Error saving permission: ${err}`);
		} finally {
			setSavingPerms(null);
		}
	};

	const toggleGroupPermissions = async (
		agentName: string,
		tools: string[],
		allow: boolean,
	) => {
		const perms = { ...(permissions[agentName] ?? {}) };
		for (const tool of tools) {
			perms[tool] = allow ? "allow" : "deny";
		}
		setSavingPerms(agentName);
		try {
			await configApi.updateAgentPermissions(agentName, perms);
			setPermissions((prev) => ({ ...prev, [agentName]: perms }));
			loadAgentContent(agentName, agentName === selected);
			setMessage("Permissions saved");
		} catch (err) {
			setMessage(`Error saving permissions: ${err}`);
		} finally {
			setSavingPerms(null);
		}
	};

	// Cycle a sub-agent's delegation rule: allow → ask → deny → allow.
	const cycleTaskPermission = async (agentName: string, subAgent: string) => {
		const next: Record<string, string> = {
			allow: "ask",
			ask: "deny",
			deny: "allow",
		};
		const current = taskPerms[agentName] ?? {};
		const value = current[subAgent] ?? "deny";
		const updated = { ...current, [subAgent]: next[value] ?? "allow" };
		setSavingTaskPerms(agentName);
		try {
			await configApi.updateAgentTaskPermissions(agentName, updated);
			setTaskPerms((prev) => ({ ...prev, [agentName]: updated }));
			setMessage("Delegation saved");
		} catch (err) {
			setMessage(`Error saving delegation: ${err}`);
		} finally {
			setSavingTaskPerms(null);
		}
	};

	const pillClassForRule = (rule: string) =>
		rule === "allow"
			? "pill-success"
			: rule === "ask"
				? "pill-warning"
				: "pill-danger";

	// Sub-agent delegation editor (permission.task) — primary agents only.
	const renderTaskDelegation = () => {
		const rules = selected ? (taskPerms[selected] ?? {}) : {};
		// Candidate sub-agents = every other non-primary agent, plus the
		// catch-all "*" default that gates anything not listed explicitly.
		const candidates = agents
			.filter((a) => a.name !== selected && a.mode !== "primary")
			.map((a) => a.name);
		const rows = ["*", ...candidates];
		return (
			<div className="settings-section">
				<div className="settings-section-header">
					<h2>Sub-agent Delegation</h2>
					<span className="permissions-helper">
						Click to cycle allow → ask → deny. Last rule wins; "*" is the
						default for anything not listed.
					</span>
				</div>
				<div className="settings-section-body">
					<div
						style={{ display: "flex", flexWrap: "wrap", gap: "var(--space-2)" }}
					>
						{rows.map((sub) => {
							const rule = rules[sub] ?? (sub === "*" ? "allow" : "deny");
							return (
								<button
									key={sub}
									className={`pill ${pillClassForRule(rule)}`}
									style={{ cursor: "pointer", fontSize: "0.72rem" }}
									onClick={() => cycleTaskPermission(selected!, sub)}
									disabled={savingTaskPerms === selected}
									data-tip={`${sub}: ${rule}`}
								>
									<span className="dot"></span>
									{sub === "*" ? "* (default)" : sub} · {rule}
								</button>
							);
						})}
					</div>
				</div>
			</div>
		);
	};

	const renderPermissionGroup = (
		label: string,
		tools: string[] | null | undefined,
		disabledGroup = false,
	) => {
		if (!tools) return null;
		const perms = permissions[selected!] ?? {};
		const allowedCount = tools.filter((t) => perms[t] === "allow").length;
		const allAllowed = allowedCount === tools.length;
		return (
			<div
				style={{
					marginBottom: "var(--space-4)",
					opacity: disabledGroup ? 0.55 : 1,
					transition: "opacity 0.15s ease",
				}}
			>
				<div
					style={{
						display: "flex",
						alignItems: "center",
						justifyContent: "space-between",
						marginBottom: "var(--space-2)",
					}}
				>
					<h4
						style={{
							fontSize: "0.72rem",
							textTransform: "uppercase",
							letterSpacing: "0.08em",
							color: "var(--text-muted)",
							margin: 0,
						}}
					>
						{label}
						{disabledGroup && (
							<span
								className="pill pill-muted"
								style={{ marginLeft: "var(--space-2)", fontSize: "0.6rem" }}
							>
								disabled
							</span>
						)}
						<span style={{ marginLeft: "var(--space-2)", opacity: 0.5 }}>
							({allowedCount}/{tools.length})
						</span>
					</h4>
					<button
						className={`pill ${allAllowed ? "pill-success" : "pill-muted"}`}
						style={{
							cursor: disabledGroup ? "not-allowed" : "pointer",
							fontSize: "0.68rem",
						}}
						onClick={() =>
							!disabledGroup &&
							toggleGroupPermissions(selected!, tools, !allAllowed)
						}
						disabled={savingPerms === selected || disabledGroup}
						data-tip={
							disabledGroup
								? "MCP disabled"
								: allAllowed
									? "Deny all"
									: "Allow all"
						}
					>
						{allAllowed ? "Deny all" : "Allow all"}
					</button>
				</div>
				<div
					style={{ display: "flex", flexWrap: "wrap", gap: "var(--space-2)" }}
				>
					{tools.map((tool) => {
						const status = perms[tool] ?? "deny";
						const isActive = status === "allow";
						return (
							<button
								key={tool}
								className={`pill ${isActive ? "pill-success" : "pill-danger"}`}
								style={{
									cursor: disabledGroup ? "default" : "pointer",
									fontSize: "0.72rem",
								}}
								onClick={() =>
									!disabledGroup && togglePermission(selected!, tool)
								}
								disabled={savingPerms === selected || disabledGroup}
								data-tip={`${tool}: ${status}${disabledGroup ? " (MCP disabled)" : ""}`}
							>
								<span className="dot"></span>
								{tool}
							</button>
						);
					})}
				</div>
			</div>
		);
	};

	return (
		<div className="agents-layout">
			{/* ── Sidebar: Agent list ── */}
			<div className="agents-sidebar">
				<div className="agents-sidebar-header">
					<input
						className="input"
						placeholder="Search agents…"
						value={searchQuery}
						onChange={(e) => setSearchQuery(e.target.value)}
					/>
					<button
						className="btn btn-primary"
						onClick={handleCreateAgent}
						disabled={saving}
					>
						+ New Agent
					</button>
				</div>
				<div className="agents-list">
					{sortedTeams.map((team) => {
						const filtered = filterAgents(grouped[team]);
						if (filtered.length === 0) return null;
						return (
							<div key={team} className="tools-section">
								<div className="agents-group-header">
									{team === "other" ? "Other" : team}
									<span style={{ marginLeft: "var(--space-2)", opacity: 0.5 }}>
										({filtered.length})
									</span>
								</div>
								{filtered.map((agent) => (
									<button
										key={agent.name}
										onClick={() => handleSelectAgent(agent.name)}
										className={`agent-item ${selected === agent.name ? "active" : ""}`}
									>
										<span className="agent-item-dot" />
										{agent.name}
									</button>
								))}
							</div>
						);
					})}
				</div>
			</div>

			{/* ── Main panel: Editor + Permissions ── */}
			<div className="agents-main">
				{selectedAgent ? (
					<>
						{/* Agent header */}
						<div className="agent-detail-header">
							<div>
								<h2 className="agent-detail-title">{selectedAgent.name}</h2>
								<span className="agent-detail-subtitle">
									{selectedAgent.group || "other"} team
								</span>
							</div>
							<button
								className="btn btn-danger"
								onClick={handleDeleteAgent}
								disabled={saving}
							>
								Delete
							</button>
						</div>

						{/* System prompt editor */}
						<div className="settings-section prompt-editor-section">
							<div className="settings-section-header">
								<h2>System Prompt</h2>
								<button
									className="btn btn-primary"
									onClick={handleSavePrompt}
									disabled={saving}
									aria-busy={saving || undefined}
								>
									{saving ? (
										<>
											<div className="spinner"></div>
											Saving…
										</>
									) : (
										"Save Prompt"
									)}
								</button>
							</div>
							<div className="settings-section-body prompt-editor-body">
								<textarea
									className="input mono prompt-textarea"
									value={editContent}
									onChange={(e) => setEditContent(e.target.value)}
									placeholder="Enter system prompt…"
								/>
							</div>
						</div>

						{/* Default model */}
						<div className="settings-section">
							<div className="settings-section-header">
								<h2>Default Model</h2>
								<span className="permissions-helper">
									Override the model for this agent. Leave empty to use the
									runtime default. Lighter models suit scouts/explorers.
								</span>
							</div>
							<div className="settings-section-body">
								<ModelCombobox
									id={`agent-${selected}-model`}
									label="Model"
									value={selected ? (agentModels[selected] ?? "") : ""}
									models={models}
									onChange={(v) => selected && handleChangeModel(selected, v)}
								/>
								{savingModel === selected && (
									<span className="muted" style={{ fontSize: "0.72rem" }}>
										Saving…
									</span>
								)}
							</div>
						</div>

						{/* Permissions */}
						<div className="settings-section">
							<div className="settings-section-header">
								<h2>Permissions</h2>
								<span className="permissions-helper">
									Click to toggle allow / deny
								</span>
							</div>
							<div className="settings-section-body">
								{/* Built-in tools from API */}
								{renderPermissionGroup("Built-in Tools", toolsData.built_in)}
								{/* MCP tools grouped by MCP name */}
								{Object.entries(toolsData.mcp_tools).map(([name, group]) =>
									renderPermissionGroup(
										`MCP: ${name}`,
										group.tools,
										!group.enabled,
									),
								)}
								{/* Plugin tools grouped by plugin name */}
								{Object.entries(toolsData.plugin_tools).map(([name, tools]) =>
									renderPermissionGroup(`Plugin: ${name}`, tools),
								)}
								{Object.keys(currentPerms).length === 0 && (
									<p className="muted" style={{ padding: "var(--space-2) 0" }}>
										No permissions configured for this agent
									</p>
								)}
							</div>
						</div>

						{/* Sub-agent delegation (primary agents only) */}
						{agents.find((a) => a.name === selected)?.mode === "primary" &&
							renderTaskDelegation()}

						{/* Status message */}
						{message && (
							<div
								className={`alert ${message.startsWith("Error") ? "alert-danger" : "alert-success"}`}
							>
								{message}
							</div>
						)}
					</>
				) : (
					<div className="empty-state">
						<p>Select an agent from the list or create a new one</p>
					</div>
				)}
			</div>
		</div>
	);
}
function SkillsTab() {
	const [skills, setSkills] = useState<
		{ name: string; hasSkillMD: boolean; description: string; scope: string }[]
	>([]);
	const [loading, setLoading] = useState(true);
	const [viewingSkill, setViewingSkill] = useState<string | null>(null);
	const [editingSkill, setEditingSkill] = useState<string | null>(null);
	const [skillContent, setSkillContent] = useState<string>("");
	const [editContent, setEditContent] = useState<string>("");
	const [saving, setSaving] = useState(false);
	const [toast, setToast] = useState<string | null>(null);

	useEffect(() => {
		configApi
			.listSkills()
			.then((list) => {
				setSkills(Array.isArray(list) ? list : []);
				setLoading(false);
			})
			.catch(() => setLoading(false));
	}, []);

	const showToast = (message: string) => {
		setToast(message);
		setTimeout(() => setToast(null), 4000);
	};

	const handleView = async (name: string) => {
		try {
			const detail = await configApi.getSkill(name);
			setSkillContent(detail.content);
			setViewingSkill(name);
		} catch (err) {
			alert(`Error: ${err}`);
		}
	};

	const handleEdit = async (name: string) => {
		try {
			const detail = await configApi.getSkill(name);
			setEditContent(detail.content);
			setEditingSkill(name);
		} catch (err) {
			alert(`Error: ${err}`);
		}
	};

	const handleSave = async () => {
		if (!editingSkill) return;
		setSaving(true);
		try {
			await configApi.updateSkill(editingSkill, editContent);
			setEditingSkill(null);
			showToast(`Skill "${editingSkill}" saved successfully`);
		} catch (err) {
			alert(`Error: ${err}`);
		} finally {
			setSaving(false);
		}
	};

	const handleDelete = async (name: string) => {
		if (!confirm(`Delete skill "${name}"?`)) return;
		try {
			await configApi.deleteSkill(name);
			setSkills((prev) => prev.filter((s) => s.name !== name));
		} catch (err) {
			alert(`Error: ${err}`);
		}
	};

	if (loading) {
		return (
			<div aria-busy="true" className="skeleton skel-card" style={{ margin: 'var(--space-4)' }}>
				<div className="skel-line title" />
				<div className="skel-line desc" />
				<div className="skel-line desc sm" />
				<div className="skel-line tag" />
			</div>
		);
	}

	return (
		<div>
			{skills.length === 0 && <p className="muted">No skills found</p>}
			<div className="skills-grid">
				{skills.map((skill) => (
					<div
						key={skill.name}
						className={`skill-card${skill.hasSkillMD ? " enabled" : ""}`}
					>
						<div className="skill-card-top">
							<span className="skill-card-name">{skill.name}</span>
							{skill.hasSkillMD && (
								<span className="pill pill-accent">Enabled</span>
							)}
						</div>
						{skill.description && (
							<p className="skill-card-desc skill-card-desc-truncate">
								{skill.description}
							</p>
						)}
						<div className="skill-card-actions">
							<button
								className="btn btn-sm"
								onClick={() => handleView(skill.name)}
							>
								View
							</button>
							<button
								className="btn btn-sm"
								onClick={() => handleEdit(skill.name)}
							>
								Edit
							</button>
							<button
								className="btn btn-sm btn-danger"
								onClick={() => handleDelete(skill.name)}
							>
								Delete
							</button>
						</div>
					</div>
				))}
			</div>

			{/* View Modal */}
			{viewingSkill && (
				<Modal
					open={true}
					onClose={() => setViewingSkill(null)}
					title={`View: ${viewingSkill}`}
				>
					<div className="skill-modal-content">
						<pre className="skill-code-block">{skillContent}</pre>
					</div>
				</Modal>
			)}

			{/* Edit Modal */}
			{editingSkill && (
				<Modal
					open={true}
					onClose={() => setEditingSkill(null)}
					title={`Edit: ${editingSkill}`}
				>
					<div className="skill-modal-content">
						<textarea
							className="skill-edit-textarea"
							value={editContent}
							onChange={(e) => setEditContent(e.target.value)}
							spellCheck={false}
						/>
						<div className="skill-modal-actions">
							<button className="btn" onClick={() => setEditingSkill(null)}>
								Cancel
							</button>
							<button
								className="btn btn-primary"
								onClick={handleSave}
								disabled={saving}
							>
								{saving ? "Saving..." : "Save"}
							</button>
						</div>
					</div>
				</Modal>
			)}

			{/* Toast */}
			{toast && (
				<div
					className="alert alert-success"
					style={{
						position: "fixed",
						bottom: "var(--space-4)",
						right: "var(--space-4)",
						zIndex: 1000,
						animation: "fadeIn 0.3s ease",
					}}
				>
					<Check size={20} />
					{toast}
				</div>
			)}
		</div>
	);
}
// ─── MCP Tab ───────────────────────────────────────────────────────────────

function MCPTab() {
	const [servers, setServers] = useState<MCPServer[]>([]);
	const [loading, setLoading] = useState(true);
	const [toggling, setToggling] = useState<string | null>(null);

	useEffect(() => {
		configApi
			.listMCP()
			.then((list) => {
				// The API returns null for an empty list; guard so servers is never
				// null (servers.length would crash the whole page).
				setServers(Array.isArray(list) ? list : []);
				setLoading(false);
			})
			.catch(() => setLoading(false));
	}, []);

	const toggleEnabled = async (server: MCPServer) => {
		setToggling(server.name);
		try {
			await configApi.updateMCP(server.name, { enabled: !server.enabled });
			setServers((prev) =>
				prev.map((s) =>
					s.name === server.name ? { ...s, enabled: !s.enabled } : s,
				),
			);
		} catch (err) {
			alert(`Error: ${err}`);
		} finally {
			setToggling(null);
		}
	};

	const handleDeleteMCP = async (name: string) => {
		if (!confirm(`Delete MCP server "${name}"?`)) return;
		try {
			await configApi.deleteMCP(name);
			setServers((prev) => prev.filter((s) => s.name !== name));
		} catch (err) {
			alert(`Error: ${err}`);
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
			{servers.length === 0 && (
				<div className="empty-state">
					<div className="empty-icon">
						<Server size={24} />
					</div>
					<span className="empty-title">No MCP servers configured</span>
					<span className="empty-desc">
						Add MCP servers to extend agent capabilities
					</span>
				</div>
			)}
			{servers.map((server) => (
				<div key={server.name} className="settings-item">
					<div className="settings-item-row">
						<div className="settings-item-main">
							<div className="mcp-header-row">
								<span className="cell-strong">{server.name}</span>
								{server.config.type && (
									<span className="tag">{server.config.type}</span>
								)}
								<span
									className={`pill ${server.enabled ? "pill-success" : "pill-muted"}`}
								>
									<span className="dot"></span>
									{server.enabled ? "Enabled" : "Disabled"}
								</span>
							</div>
							<p className="mcp-command-desc">
								{server.config.command?.join(" ") ?? server.config.url ?? "—"}
							</p>
						</div>
						<div className="mcp-actions-row">
							<button
								className={`btn btn-sm ${server.enabled ? "btn-ghost" : "btn-primary"}`}
								onClick={() => toggleEnabled(server)}
								disabled={toggling === server.name}
								aria-busy={toggling === server.name || undefined}
							>
								{toggling === server.name ? (
									<div className="spinner"></div>
								) : server.enabled ? (
									"Disable"
								) : (
									"Enable"
								)}
							</button>
							<button
								className="btn btn-sm btn-danger"
								onClick={() => handleDeleteMCP(server.name)}
							>
								Delete
							</button>
						</div>
					</div>
				</div>
			))}
		</div>
	);
}

// ─── Providers Tab ─────────────────────────────────────────────────────────

function ProvidersTab() {
	const [providers, setProviders] = useState<Record<string, ProviderInfo>>({});
	const [loading, setLoading] = useState(true);
	const [editing, setEditing] = useState<string | null>(null);
	const [editForm, setEditForm] = useState({
		baseURL: "",
		apiKey: "",
		models: "",
	});

	useEffect(() => {
		configApi
			.listProviders()
			.then((list) => {
				setProviders(list ?? {});
				setLoading(false);
			})
			.catch(() => setLoading(false));
	}, []);

	const startEdit = (name: string) => {
		const p = providers[name];
		const opts = (p?.options ?? {}) as Record<string, string>;
		setEditing(name);
		setEditForm({
			baseURL: opts.baseURL ?? "",
			apiKey: opts.apiKey ?? "",
			models: JSON.stringify(p?.models ?? {}, null, 2),
		});
	};

	const handleSave = async () => {
		if (!editing) return;
		try {
			let models: Record<string, unknown> = {};
			try {
				models = JSON.parse(editForm.models);
			} catch {
				alert("Invalid models JSON");
				return;
			}

			await configApi.updateProvider(editing, {
				...providers[editing],
				options: { baseURL: editForm.baseURL, apiKey: editForm.apiKey },
				models,
			});
			const updated = await configApi.listProviders();
			setProviders(updated);
			setEditing(null);
		} catch (err) {
			alert(`Error: ${err}`);
		}
	};

	const handleDelete = async (name: string) => {
		if (!confirm(`Delete provider "${name}"?`)) return;
		try {
			await configApi.deleteProvider(name);
			setProviders((prev) => {
				const next = { ...prev };
				delete next[name];
				return next;
			});
		} catch (err) {
			alert(`Error: ${err}`);
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

	const entries = Object.entries(providers);

	return (
		<div className="card card-pad">
			{entries.length === 0 && (
				<div className="empty-state">
					<div className="empty-icon">
						<Package size={24} />
					</div>
					<span className="empty-title">No providers found</span>
					<span className="empty-desc">
						Add AI providers to configure models and API keys
					</span>
				</div>
			)}
			{entries.map(([name, provider]) => {
				const opts = (provider.options ?? {}) as Record<string, string>;
				const baseURL = opts.baseURL ?? "";
				const modelCount = Object.keys(provider.models ?? {}).length;
				const hasKey = !!opts.apiKey;

				if (editing === name) {
					return (
						<div key={name} className="settings-item">
							<div className="form-grid">
								<div className="field span-2">
									<label className="field-label">Base URL</label>
									<input
										className="input mono"
										value={editForm.baseURL}
										onChange={(e) =>
											setEditForm({ ...editForm, baseURL: e.target.value })
										}
										placeholder="https://api.example.com/v1"
									/>
								</div>
								<div className="field span-2">
									<label className="field-label">API Key</label>
									<input
										className="input mono"
										type="password"
										value={editForm.apiKey}
										onChange={(e) =>
											setEditForm({ ...editForm, apiKey: e.target.value })
										}
										placeholder="sk-…"
									/>
								</div>
								<div className="field span-2">
									<label className="field-label">Models (JSON)</label>
									<textarea
										className="textarea mono"
										rows={6}
										value={editForm.models}
										onChange={(e) =>
											setEditForm({ ...editForm, models: e.target.value })
										}
									/>
								</div>
							</div>
							<div className="provider-edit-actions">
								<button className="btn btn-primary" onClick={handleSave}>
									Save
								</button>
								<button
									className="btn btn-ghost"
									onClick={() => setEditing(null)}
								>
									Cancel
								</button>
							</div>
						</div>
					);
				}

				return (
					<div key={name} className="settings-item">
						<div className="settings-item-row">
							<div className="settings-item-main">
								<div className="provider-header-row">
									<span className="cell-strong">{provider.name ?? name}</span>
									<span className="tag">{modelCount} models</span>
									<span
										className={`pill ${hasKey ? "pill-success" : "pill-danger"}`}
									>
										<span className="dot"></span>
										{hasKey ? "Key set" : "No key"}
									</span>
								</div>
								<p className="provider-url-desc">
									{baseURL || "No base URL configured"}
								</p>
							</div>
							<div className="provider-actions-row">
								<button
									className="btn btn-sm btn-ghost"
									onClick={() => startEdit(name)}
								>
									Edit
								</button>
								<button
									className="btn btn-sm btn-danger"
									onClick={() => handleDelete(name)}
								>
									Delete
								</button>
							</div>
						</div>
					</div>
				);
			})}
		</div>
	);
}

// ─── Tools Tab ─────────────────────────────────────────────────────────────

function ToolsTab() {
	const [tools, setTools] = useState<{
		built_in: string[];
		all: string[];
		mcp_tools: Record<string, { tools: string[]; enabled: boolean }>;
		plugin_tools: Record<string, string[]>;
	} | null>(null);
	const [loading, setLoading] = useState(true);
	const [resyncing, setResyncing] = useState(false);

	const load = useCallback((refresh: boolean) => {
		if (refresh) setResyncing(true);
		configApi
			.listTools(refresh)
			.then((data) => {
				setTools(data);
				setLoading(false);
			})
			.catch(() => setLoading(false))
			.finally(() => setResyncing(false));
	}, []);

	useEffect(() => {
		load(false);
	}, [load]);

	if (loading) {
		return (
			<div aria-busy="true" className="skeleton skel-card" style={{ margin: 'var(--space-4)' }}>
				<div className="skel-line title" />
				<div className="skel-line desc" />
				<div className="skel-line desc sm" />
			</div>
		);
	}

	if (!tools) {
		return (
			<div className="card card-pad">
				<p className="muted">No tools data available</p>
			</div>
		);
	}

	const sections: { label: string; data: string[]; icon: React.ReactNode }[] = [
		{ label: "Built-in", data: tools.built_in, icon: <SettingsIcon size={16} /> },
		...Object.entries(tools.mcp_tools).map(([server, group]) => ({
			label: `MCP: ${server}${group.enabled ? "" : " (disabled)"}`,
			data: group.tools,
			icon: <Plug size={16} />,
		})),
		...Object.entries(tools.plugin_tools).map(([plugin, toolsList]) => ({
			label: `Plugin: ${plugin}`,
			data: toolsList,
			icon: <Package size={16} />,
		})),
	];

	return (
		<div className="card card-pad">
			<div className="tools-header">
				<button
					type="button"
					className="btn btn-sm"
					onClick={() => load(true)}
					disabled={resyncing}
					title="Re-scan tools (built-in, MCP servers, and plugins)"
				>
					<RefreshCw
						size={14}
						className={resyncing ? "spin" : undefined}
						aria-hidden="true"
					/>
					{resyncing ? "Resyncing…" : "Resync"}
				</button>
			</div>
			{sections.length === 0 && <p className="muted">No tools found</p>}
			{sections.map((section) => {
				const toolsList = Array.isArray(section.data) ? section.data : [];
				if (toolsList.length === 0) return null;
				return (
					<div key={section.label} className="tools-section">
						<h3 className="tools-section-title">
							<span style={{ display: "inline-flex", alignItems: "center", gap: "var(--space-1)" }}>
								{section.icon}
								{section.label}
							</span>
							<span className="tools-section-count">({toolsList.length})</span>
						</h3>
						<div className="tools-pills-container">
							{toolsList.map((tool) => (
								<span key={tool} className="pill pill-muted tools-pill">
									{tool}
								</span>
							))}
						</div>
					</div>
				);
			})}
		</div>
	);
}
