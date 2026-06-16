import { useEffect, useState } from "react";
import { configApi, missionsApi } from "../../api/client";
import type {
	MCPServer,
	ProviderInfo,
	OpenCodeConfig as OpenCodeConfigType,
} from "../../api/types";
import RoleDefaultsTab from "./RoleDefaultsTab";
import SearchSelect from "../shared/SearchSelect";
import "./Settings.css";

type Tab =
	| "general"
	| "roles"
	| "agents"
	| "skills"
	| "mcp"
	| "providers"
	| "tools";

const TABS: { id: Tab; label: string; icon: React.ReactNode }[] = [
	{
		id: "general",
		label: "General",
		icon: (
			<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
				<circle cx="12" cy="12" r="3" />
				<path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
			</svg>
		),
	},
	{
		id: "roles",
		label: "Role Defaults",
		icon: (
			<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
				<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
				<circle cx="9" cy="7" r="4" />
				<path d="M22 21v-2a4 4 0 0 0-3-3.87" />
				<path d="M16 3.13a4 4 0 0 1 0 7.75" />
			</svg>
		),
	},
	{
		id: "agents",
		label: "Agents",
		icon: (
			<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
				<path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
				<circle cx="12" cy="7" r="4" />
			</svg>
		),
	},
	{
		id: "skills",
		label: "Skills",
		icon: (
			<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
				<polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2" />
			</svg>
		),
	},
	{
		id: "mcp",
		label: "MCP",
		icon: (
			<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
				<rect x="2" y="3" width="20" height="14" rx="2" ry="2" />
				<line x1="8" y1="21" x2="16" y2="21" />
				<line x1="12" y1="17" x2="12" y2="21" />
			</svg>
		),
	},
	{
		id: "providers",
		label: "Providers",
		icon: (
			<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
				<path d="M22 12h-4l-3 9L9 3l-3 9H2" />
			</svg>
		),
	},
	{
		id: "tools",
		label: "Tools",
		icon: (
			<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
				<path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" />
			</svg>
		),
	},
];

export default function Settings() {
	const [activeTab, setActiveTab] = useState<Tab>("general");

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
				{activeTab === "skills" && <SkillsTab />}
				{activeTab === "mcp" && <MCPTab />}
				{activeTab === "providers" && <ProvidersTab />}
				{activeTab === "tools" && <ToolsTab />}
			</div>
		</div>
	);
}

// ─── General Tab ───────────────────────────────────────────────────────────

function GeneralTab() {
	const [config, setConfig] = useState<OpenCodeConfigType | null>(null);
	const [agentList, setAgentList] = useState<string[]>([]);
	const [providerList, setProviderList] = useState<string[]>([]);
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [message, setMessage] = useState<string | null>(null);

	useEffect(() => {
		// listProviders() only returns providers declared in opencode.json (3-ish).
		// missionsApi.listModels() asks opencode CLI which knows the full runtime
		// list (github-copilot, openai, opencode, etc — 6+). Union both so the
		// datalist offers everything the user can actually pick.
		Promise.all([
			configApi.getConfig().catch(() => null),
			configApi.listAgents().catch(() => [] as { name: string }[]),
			configApi.listProviders().catch(() => ({})),
			missionsApi.listModels().catch(() => null),
		]).then(([cfg, agents, providers, modelsRes]) => {
			if (cfg) setConfig(cfg);
			setAgentList((agents ?? []).map((a) => a.name));
			const declared = Object.keys(providers ?? {});
			const runtime = modelsRes
				? Object.keys(modelsRes.modelsByProvider ?? {})
				: [];
			const union = Array.from(new Set([...declared, ...runtime])).sort((a, b) =>
				a.localeCompare(b),
			);
			setProviderList(union);
			setLoading(false);
		});
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
			if (config.provider !== undefined) toSave.provider = config.provider;
			if (config.model) toSave.model = config.model;
			if (config.agent !== undefined) toSave.agent = config.agent;
			if (config.maxTokens !== undefined) toSave.maxTokens = config.maxTokens;
			if (config.temperature !== undefined)
				toSave.temperature = config.temperature;
			await configApi.updateConfig(toSave as OpenCodeConfigType);
			setMessage("Saved successfully");
		} catch (err) {
			setMessage(`Error: ${err}`);
		} finally {
			setSaving(false);
		}
	};

	if (loading) {
		return (
			<div className="loading-inline">
				<div className="spinner"></div>
				<span>Loading config…</span>
			</div>
		);
	}

	if (!config) return <p className="muted">No config available</p>;

	return (
		<div className="card card-pad">
			<div className="form-grid">
				<div className="field">
					<label className="field-label" htmlFor="cfg-provider">
						Provider
					</label>
					<span className="field-hint">The LLM provider for this configuration</span>
					<SearchSelect
						id="cfg-provider"
						value={readKey(config.provider)}
						options={providerList}
						placeholder="e.g., openai"
						onChange={(v) => setConfig({ ...config, provider: v })}
					/>
				</div>
				<div className="field">
					<label className="field-label" htmlFor="cfg-model">
						Model
					</label>
					<span className="field-hint">The model identifier to use</span>
					<input
						id="cfg-model"
						className="input"
						value={config.model ?? ""}
						onChange={(e) => setConfig({ ...config, model: e.target.value })}
						placeholder="e.g., gpt-4o"
					/>
				</div>
				<div className="field span-2">
					<label className="field-label" htmlFor="cfg-agent">
						Default Agent
					</label>
					<span className="field-hint">The default agent for new conversations</span>
					<SearchSelect
						id="cfg-agent"
						value={readKey(config.agent)}
						options={agentList}
						placeholder="e.g., orchestrator"
						onChange={(v) => setConfig({ ...config, agent: v })}
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
					<span className="field-hint">Controls randomness: 0=focused, 2=creative</span>
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

// ─── Agents Tab ────────────────────────────────────────────────────────────

function AgentsTab() {
	const [agents, setAgents] = useState<
		{ name: string; content?: string; group?: string }[]
	>([]);
	const [loading, setLoading] = useState(true);
	const [selected, setSelected] = useState<string | null>(null);
	const [editContent, setEditContent] = useState("");
	const [permissions, setPermissions] = useState<
		Record<string, Record<string, string>>
	>({});
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
			configApi.listTools().catch(() => ({ built_in: [] as string[], all: [] as string[], mcp_tools: {} as Record<string, { tools: string[]; enabled: boolean }>, plugin_tools: {} as Record<string, string[]> })),
		])
			.then(([agentList, toolsRes]) => {
				const list = agentList.map((a) => ({
					name: a.name,
					content: undefined as string | undefined,
					group: a.group,
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
	useEffect(() => {
		if (!selected) return;
		const agent = agents.find((a) => a.name === selected);
		if (!agent) return;
		if (agent.content !== undefined) {
			setEditContent(agent.content);
			return;
		}
		configApi
			.getAgent(selected)
			.then((d) => {
				setAgents((prev) =>
					prev.map((a) =>
						a.name === selected ? { ...a, content: d.content } : a,
					),
				);
				setEditContent(d.content ?? "");
			})
			.catch(() => {});
	}, [selected, agents]);

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
			await configApi.createAgent(clean, `# ${clean}\n\nYou are a helpful assistant.`);
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
			<div className="loading-inline">
				<div className="spinner"></div>
				<span>Loading agents…</span>
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
	const teamOrder = ["core", "social-refactor", "experiment", "custom", "other"];
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
	const currentPerms = selected ? permissions[selected] ?? {} : {};

	// --- Permission rendering (dynamic from API) ---
	const togglePermission = async (agentName: string, tool: string) => {
		const perms = { ...(permissions[agentName] ?? {}) };
		perms[tool] = perms[tool] === "allow" ? "deny" : "allow";
		setSavingPerms(agentName);
		try {
			await configApi.updateAgentPermissions(agentName, perms);
			setPermissions((prev) => ({ ...prev, [agentName]: perms }));
			setMessage(null);
		} catch (err) {
			setMessage(`Error saving permission: ${err}`);
		} finally {
			setSavingPerms(null);
		}
	};

	const toggleGroupPermissions = async (agentName: string, tools: string[], allow: boolean) => {
		const perms = { ...(permissions[agentName] ?? {}) };
		for (const tool of tools) {
			perms[tool] = allow ? "allow" : "deny";
		}
		setSavingPerms(agentName);
		try {
			await configApi.updateAgentPermissions(agentName, perms);
			setPermissions((prev) => ({ ...prev, [agentName]: perms }));
			setMessage(null);
		} catch (err) {
			setMessage(`Error saving permissions: ${err}`);
		} finally {
			setSavingPerms(null);
		}
	};

	const renderPermissionGroup = (label: string, tools: string[] | null | undefined, disabledGroup = false) => {
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
						style={{ cursor: disabledGroup ? "not-allowed" : "pointer", fontSize: "0.68rem" }}
						onClick={() => !disabledGroup && toggleGroupPermissions(selected!, tools, !allAllowed)}
						disabled={savingPerms === selected || disabledGroup}
						data-tip={disabledGroup ? "MCP disabled" : allAllowed ? "Deny all" : "Allow all"}
					>
						{allAllowed ? "Deny all" : "Allow all"}
					</button>
				</div>
				<div style={{ display: "flex", flexWrap: "wrap", gap: "var(--space-2)" }}>
					{tools.map((tool) => {
						const status = perms[tool] ?? "deny";
						const isActive = status === "allow";
						return (
							<button
								key={tool}
								className={`pill ${isActive ? "pill-success" : "pill-danger"}`}
								style={{ cursor: disabledGroup ? "default" : "pointer", fontSize: "0.72rem" }}
								onClick={() => !disabledGroup && togglePermission(selected!, tool)}
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
	};;

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
								<h2 className="agent-detail-title">
									{selectedAgent.name}
								</h2>
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
									renderPermissionGroup(`MCP: ${name}`, group.tools, !group.enabled),
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
		{ name: string; hasSkillMD: boolean; description: string }[]
	>([]);
	const [loading, setLoading] = useState(true);
	const [expanded, setExpanded] = useState<string | null>(null);
	const [skillContent, setSkillContent] = useState<Record<string, string>>({});
	const [editing, setEditing] = useState<string | null>(null);
	const [editContent, setEditContent] = useState('');
	const [savingSkill, setSavingSkill] = useState(false);

	useEffect(() => {
		configApi
			.listSkills()
			.then((list) => {
				setSkills(list);
				setLoading(false);
			})
			.catch(() => setLoading(false));
	}, []);

	const toggleSkill = async (name: string) => {
		if (expanded === name) {
			setExpanded(null);
			setEditing(null);
			return;
		}
		if (!skillContent[name]) {
			try {
				const detail = await configApi.getSkill(name);
				setSkillContent((prev) => ({ ...prev, [name]: detail.content }));
			} catch { /* ignore */ }
		}
		setExpanded(name);
		setEditing(null);
	};

	const startEditSkill = (name: string) => {
		setEditing(name);
		setEditContent(skillContent[name] ?? '');
	};

	const handleSaveSkill = async (name: string) => {
		setSavingSkill(true);
		try {
			await configApi.updateSkill(name, editContent);
			setSkillContent((prev) => ({ ...prev, [name]: editContent }));
			setEditing(null);
		} catch (err) {
			alert(`Error: ${err}`);
		} finally {
			setSavingSkill(false);
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
			<div className="loading-inline">
				<div className="spinner"></div>
				<span>Loading skills…</span>
			</div>
		);
	}

	return (
		<div>
			{skills.length === 0 && <p className="muted">No skills found</p>}
			<div className="skills-grid">
				{skills.map((skill) => {
					const isOpen = expanded === skill.name;
					return (
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
								<p className="skill-card-desc">{skill.description}</p>
							)}
							<div className="skill-card-actions">
								<button
									className="btn btn-sm"
									onClick={() => toggleSkill(skill.name)}
								>
									{isOpen ? "Close" : "View"}
								</button>
								<button
									className="btn btn-sm"
									onClick={() => startEditSkill(skill.name)}
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
							{isOpen && editing === skill.name ? (
								<div className="skill-card-content">
									<textarea
										className="input mono skill-edit-textarea"
										rows={12}
										value={editContent}
										onChange={(e) => setEditContent(e.target.value)}
									/>
									<div className="skill-edit-actions">
										<button className="btn btn-primary" onClick={() => handleSaveSkill(skill.name)} disabled={savingSkill}>
											{savingSkill ? 'Saving...' : 'Save'}
										</button>
										<button className="btn" onClick={() => setEditing(null)}>Cancel</button>
									</div>
								</div>
							) : isOpen && skillContent[skill.name] ? (
								<div className="skill-card-content">
									<pre className="settings-code">
										{skillContent[skill.name].slice(0, 2000)}
										{skillContent[skill.name].length > 2000 ? "…" : ""}
									</pre>
								</div>
							) : null}
						</div>
					);
				})}
			</div>
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
				setServers(list);
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
			<div className="loading-inline">
				<div className="spinner"></div>
				<span>Loading MCP servers…</span>
			</div>
		);
	}

	return (
		<div className="card card-pad">
			{servers.length === 0 && (
				<div className="empty-state">
					<div className="empty-icon">
						<svg
							width="26"
							height="26"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							strokeWidth="2"
							strokeLinecap="round"
							strokeLinejoin="round"
						>
							<rect x="2" y="2" width="20" height="8" rx="2" ry="2" />
							<rect x="2" y="14" width="20" height="8" rx="2" ry="2" />
							<line x1="6" y1="6" x2="6.01" y2="6" />
							<line x1="6" y1="18" x2="6.01" y2="18" />
						</svg>
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
				setProviders(list);
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
			<div className="loading-inline">
				<div className="spinner"></div>
				<span>Loading providers…</span>
			</div>
		);
	}

	const entries = Object.entries(providers);

	return (
		<div className="card card-pad">
			{entries.length === 0 && (
				<div className="empty-state">
					<div className="empty-icon">
						<svg
							width="26"
							height="26"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							strokeWidth="2"
							strokeLinecap="round"
							strokeLinejoin="round"
						>
							<path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" />
						</svg>
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

	useEffect(() => {
		configApi
			.listTools()
			.then((data) => {
				setTools(data);
				setLoading(false);
			})
			.catch(() => setLoading(false));
	}, []);

	if (loading) {
		return (
			<div className="loading-inline">
				<div className="spinner"></div>
				<span>Loading tools…</span>
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

	const sections: { label: string; data: string[]; icon: string }[] = [
		{ label: "Built-in", data: tools.built_in, icon: "⚙" },
		...Object.entries(tools.mcp_tools).map(([server, group]) => ({
			label: `MCP: ${server}${group.enabled ? "" : " (disabled)"}`,
			data: group.tools,
			icon: "🔌",
		})),
		...Object.entries(tools.plugin_tools).map(([plugin, toolsList]) => ({
			label: `Plugin: ${plugin}`,
			data: toolsList,
			icon: "📦",
		})),
	];

	return (
		<div className="card card-pad">
			{sections.length === 0 && <p className="muted">No tools found</p>}
			{sections.map((section) => {
				const toolsList = Array.isArray(section.data) ? section.data : [];
				if (toolsList.length === 0) return null;
				return (
					<div key={section.label} className="tools-section">
						<h3 className="tools-section-title">
							{section.icon} {section.label}
							<span className="tools-section-count">
								({toolsList.length})
							</span>
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
