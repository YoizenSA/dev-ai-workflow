import type {
	Session,
	Delegation,
	BoardView,
	GraphView,
	ActivityEvent,
	Mission,
	PlanMission,
	Project,
	GitInfo,
	AgentInfo,
	AgentDetail,
	AgentGraph,
	DelegationRulesResp,
	SkillInfo,
	MCPServer,
	ProviderInfo,
	OpenCodeConfig,
	ToolsResponse,
	ModelsResponse,
	AgentsResponse,
	FeatureLogsResponse,
	BrowseFSResponse,
	UserConfig,
	RoleDefaults,
	EngramObservation,
	EngramSession,
	EngramPrompt,
	EngramStats,
	EngramTimelineEvent,
	EngramContextResult,
	EngramStatus,
	EngramImportResult,
	EngramMergeResult,
	MemoryEvalRequest,
	OrchestratorProfilesResponse,
	MemoryEvalResult,
	ConsolidationRun,
	ApplySelection,
	Workflow,
	WorkflowSummary,
	WorkflowValidationResult,
	WorkflowExportPlan,
	WorkflowImportResult,
} from "./types";

const BASE = "";

async function request<T>(path: string, options?: RequestInit): Promise<T> {
	const res = await fetch(`${BASE}${path}`, {
		headers: { "Content-Type": "application/json" },
		...options,
	});
	if (!res.ok) {
		const body = await res.text().catch(() => res.statusText);
		throw new Error(`${res.status}: ${body}`);
	}
	return res.json();
}

// requestText fetches a plain-text response (e.g. mission artifacts like
// REPORT.md, architecture.md). Returns empty string on 404 so callers can
// treat "not generated yet" uniformly.
async function requestText(
	path: string,
	options?: RequestInit,
): Promise<string> {
	const res = await fetch(`${BASE}${path}`, options);
	if (!res.ok) {
		if (res.status === 404) return "";
		const body = await res.text().catch(() => res.statusText);
		throw new Error(`${res.status}: ${body}`);
	}
	return res.text();
}

// ─── Kanban API ────────────────────────────────────────────────────────────

export const kanbanApi = {
	// Sessions
	listSessions: () => request<Session[]>("/api/sessions"),
	getSession: (id: string) => request<Session>(`/api/sessions/${id}`),
	createSession: (data: { project: string; goal: string }) =>
		request<Session>("/api/sessions", {
			method: "POST",
			body: JSON.stringify(data),
		}),
	updateSession: (id: string, data: Partial<Session>) =>
		request<Session>(`/api/sessions/${id}`, {
			method: "PATCH",
			body: JSON.stringify(data),
		}),
	deleteSession: (id: string) =>
		fetch(`${BASE}/api/sessions/${id}`, { method: "DELETE" }).then((r) => {
			if (!r.ok) throw new Error(`${r.status}`);
		}),
	deleteSessionsByProject: (project: string) =>
		fetch(`${BASE}/api/sessions?project=${encodeURIComponent(project)}`, {
			method: "DELETE",
		}).then((r) => {
			if (!r.ok) throw new Error(`${r.status}`);
		}),
	updateSessionsByProject: (project: string, data: Partial<Session>) =>
		request<Session>(
			`/api/sessions?project=${encodeURIComponent(project)}`,
			{
				method: "PATCH",
				body: JSON.stringify(data),
			},
		),

	// Board — the API returns { session, columns }; the store wants the flat columns.
	getBoard: async (sessionId: string): Promise<BoardView> => {
		const data = await request<{ session: Session; columns: BoardView }>(
			`/api/sessions/${sessionId}/board`,
		);
		return data.columns;
	},
	getGraph: (sessionId: string) =>
		request<GraphView>(`/api/sessions/${sessionId}/graph`),

	// Delegations
	createDelegation: (data: {
		session_id: string;
		agent: string;
		task_summary: string;
		dependencies?: string[];
	}) =>
		request<Delegation>("/api/delegations", {
			method: "POST",
			body: JSON.stringify(data),
		}),
	getDelegation: (id: string) => request<Delegation>(`/api/delegations/${id}`),
	updateDelegation: (
		id: string,
		data: { column?: string; status?: string; blocker?: string },
	) =>
		request<Delegation>(`/api/delegations/${id}`, {
			method: "PATCH",
			body: JSON.stringify(data),
		}),

	// Activities
	createActivity: (
		delegationId: string,
		data: { type: string; content: string; options?: string[] },
	) =>
		request<ActivityEvent>(`/api/delegations/${delegationId}/activities`, {
			method: "POST",
			body: JSON.stringify(data),
		}),
	getActivities: (delegationId: string) =>
		request<ActivityEvent[]>(`/api/delegations/${delegationId}/activities`),
	resolveActivity: (
		delegationId: string,
		activityId: string,
		resolution: string,
	) =>
		request<ActivityEvent>(
			`/api/delegations/${delegationId}/activities/${activityId}`,
			{
				method: "PATCH",
				body: JSON.stringify({ resolution }),
			},
		),

	// Pending decisions
	getPendingDecisions: (sessionId: string) =>
		request<ActivityEvent[]>(`/api/sessions/${sessionId}/decisions`),
};

// ─── Missions API ──────────────────────────────────────────────────────────

export const missionsApi = {
	// Missions
	listMissions: () =>
		request<{ missions: Mission[] }>("/missions/api/missions").then(
			(r) => r.missions,
		),
	getMission: (id: string) => request<Mission>(`/missions/api/missions/${id}`),
	generatePlan: (data: {
		goal: string;
		project?: string;
		model?: string;
		agent?: string;
	}) =>
		request<{ plan: PlanMission }>("/missions/api/missions", {
			method: "POST",
			body: JSON.stringify(data),
		}),

	approvePlan: (plan: PlanMission) =>
		request<{ mission: Mission }>("/missions/api/missions/approve", {
			method: "POST",
			body: JSON.stringify({ plan }),
		}),
	runMission: (id: string) =>
		request<void>(`/missions/api/missions/${id}/run`, { method: "POST" }),
	autoMission: (data: {
		goal: string;
		project?: string;
		model?: string;
		agent?: string;
		autoApprove?: boolean;
	}) =>
		request<{ status: string; missionId: string }>(
			"/missions/api/missions/auto",
			{ method: "POST", body: JSON.stringify(data) },
		),
	pauseMission: (id: string) =>
		request<void>(`/missions/api/missions/${id}/pause`, { method: "POST" }),
	resumeMission: (id: string) =>
		request<void>(`/missions/api/missions/${id}/resume`, { method: "POST" }),
	cancelMission: (id: string) =>
		request<void>(`/missions/api/missions/${id}/cancel`, { method: "POST" }),
	deleteMission: (id: string) =>
		request<{ status: string; id: string }>(`/missions/api/missions/${id}`, {
			method: "DELETE",
		}),

	// Mission artifacts (plain text): architecture, report, services, etc.
	// Returns empty string when the artifact hasn't been generated yet.
	getMissionArtifact: (id: string, type: string) =>
		requestText(`/missions/api/missions/${id}/artifacts/${type}`),

	// Projects
	listProjects: () =>
		request<{ projects: Project[] }>("/missions/api/projects").then(
			(r) => r.projects,
		),
	createProject: (name: string, path: string) =>
		request<{ project: Project }>("/missions/api/projects", {
			method: "POST",
			body: JSON.stringify({ name, path }),
		}).then((r) => r.project),
	deleteProject: (name: string) =>
		fetch(`${BASE}/missions/api/projects/${name}`, { method: "DELETE" }).then(
			(r) => {
				if (!r.ok) throw new Error(`${r.status}`);
			},
		),

	// Git introspection for a project's repo.
	getProjectGitInfo: (name: string) =>
		request<GitInfo>(`/missions/api/projects/${name}/git-info`),
	initProjectGit: (name: string) =>
		request<{ status: string; git: GitInfo }>(
			`/missions/api/projects/${name}/init-git`,
			{ method: "POST" },
		),
	getFeatureLogs: (missionId: string, featureId: string) =>
		request<FeatureLogsResponse>(
			`/missions/api/missions/${missionId}/features/${featureId}/logs`,
		),

	// Models & Agents
	listModels: () => request<ModelsResponse>("/missions/api/opencode/models"),
	listAgents: () => request<AgentsResponse>("/missions/api/opencode/agents"),
	startOpencode: () =>
		request<{ status: string; message: string; pid?: number }>(
			"/missions/api/opencode/start",
			{ method: "POST" },
		),

	// File system browser
	browseFS: (path?: string) =>
		request<BrowseFSResponse>(
			`/missions/api/fs/browse${path ? `?path=${encodeURIComponent(path)}` : ""}`,
		),
	createFolder: (parentPath: string, name: string) =>
		request<{ path: string }>("/missions/api/fs/mkdir", {
			method: "POST",
			body: JSON.stringify({ parentPath, name }),
		}),

	// AI refinement
	refineGoal: (goal: string, context?: string, model?: string) =>
		request<{ refined: string }>("/missions/api/refine", {
			method: "POST",
			body: JSON.stringify({ goal, context, model }),
		}),
};

// ─── Config API ────────────────────────────────────────────────────────────

export const configApi = {
	// Version check
	getVersion: () => request<{ current: string; latest: string | null; updateAvailable: boolean; error?: string }>("/api/version"),
	// Trigger a detached `ywai update`. The server relaunches itself, so the
	// caller should poll health and reload once it comes back.
	triggerUpdate: () => request<{ started: boolean; pid?: number; error?: string }>("/api/update", { method: "POST" }),

	// OpenCode general config
	getConfig: () => request<OpenCodeConfig>("/api/config/opencode"),
	updateConfig: (data: Partial<OpenCodeConfig>) =>
		request<void>("/api/config/opencode", {
			method: "PUT",
			body: JSON.stringify(data),
		}),

		// Agents
		listAgents: () => request<AgentInfo[]>("/api/config/agents"),
		getAgentGraph: () => request<AgentGraph>("/api/config/agents/graph"),
	getAgent: (name: string) =>
		request<AgentDetail>(`/api/config/agents/${name}`),
	getAgentPermissions: (name: string) =>
		request<Record<string, string>>(`/api/config/agents/${name}/permissions`),
	updateAgentPermissions: (name: string, perms: Record<string, string>) =>
		request<void>(`/api/config/agents/${name}/permissions`, {
			method: "PUT",
			body: JSON.stringify(perms),
		}),
	getAgentModel: (name: string) =>
		request<{ model: string }>(`/api/config/agents/${name}/model`),
	updateAgentModel: (name: string, model: string) =>
		request<void>(`/api/config/agents/${name}/model`, {
			method: "PUT",
			body: JSON.stringify({ model }),
		}),
	getAgentTaskPermissions: (name: string) =>
		request<Record<string, string>>(
			`/api/config/agents/${name}/task-permissions`,
		),
	updateAgentTaskPermissions: (name: string, perms: Record<string, string>) =>
		request<void>(`/api/config/agents/${name}/task-permissions`, {
			method: "PUT",
			body: JSON.stringify(perms),
		}),
	getDelegationRules: (name: string) =>
		request<DelegationRulesResp>(
			`/api/config/agents/${name}/delegation-rules`,
		),
	updateDelegationRules: (
		name: string,
		data: { rules: DelegationRulesResp["rules"]; triggers: DelegationRulesResp["triggers"] },
	) =>
		request<void>(`/api/config/agents/${name}/delegation-rules`, {
			method: "PUT",
			body: JSON.stringify(data),
		}),

	createAgent: (name: string, content: string) =>
		request<void>("/api/config/agents", {
			method: "POST",
			body: JSON.stringify({ name, content }),
		}),

	updateAgent: (name: string, content: string) =>
		request<void>(`/api/config/agents/${name}`, {
			method: "PUT",
			body: JSON.stringify({ content }),
		}),

	deleteAgent: (name: string) =>
		fetch(`${BASE}/api/config/agents/${name}`, { method: "DELETE" }).then(
			(r) => {
				if (!r.ok) throw new Error(`${r.status}`);
			},
		),

	// Skills
	listSkills: () => request<SkillInfo[]>("/api/config/skills"),
	getSkill: (name: string) =>
		request<{ name: string; content: string }>(`/api/config/skills/${name}`),
	updateSkill: (name: string, content: string) =>
		request<void>(`/api/config/skills/${name}`, {
			method: "PUT",
			body: JSON.stringify({ content }),
		}),
	deleteSkill: (name: string) =>
		fetch(`${BASE}/api/config/skills/${name}`, { method: "DELETE" }).then(
			(r) => {
				if (!r.ok) throw new Error(`${r.status}`);
			},
		),

	// MCP Servers
	listMCP: () => request<MCPServer[]>("/api/config/mcp"),
	updateMCP: (name: string, data: Partial<MCPServer>) =>
		request<void>(`/api/config/mcp/${name}`, {
			method: "PUT",
			body: JSON.stringify(data),
		}),
	deleteMCP: (name: string) =>
		fetch(`${BASE}/api/config/mcp/${name}`, { method: "DELETE" }).then((r) => {
			if (!r.ok) throw new Error(`${r.status}`);
		}),

	// Providers
	listProviders: () =>
		request<Record<string, ProviderInfo>>("/api/config/providers"),
	updateProvider: (name: string, data: Partial<ProviderInfo>) =>
		request<void>(`/api/config/providers/${name}`, {
			method: "PUT",
			body: JSON.stringify(data),
		}),
	deleteProvider: (name: string) =>
		fetch(`${BASE}/api/config/providers/${name}`, { method: "DELETE" }).then(
			(r) => {
				if (!r.ok) throw new Error(`${r.status}`);
			},
		),

	// Tools. Pass refresh=true to bypass the server cache and force a fresh
	// rediscovery (used by the resync button after adding a plugin/MCP).
	listTools: (refresh = false) =>
		request<ToolsResponse>(`/api/config/tools${refresh ? "?refresh=1" : ""}`),

	// User config (role defaults)
	getUserConfig: () => request<UserConfig>("/api/config/user"),
	updateUserConfig: (data: Partial<UserConfig>) =>
		request<{ status: string }>("/api/config/user", {
			method: "PUT",
			body: JSON.stringify(data),
		}),
	getRoleDefaults: () =>
		request<RoleDefaults>("/api/config/user/role-defaults"),
	getOrchestratorProfiles: () =>
		request<OrchestratorProfilesResponse>("/api/config/user/orchestrator-profiles"),
	setActiveOrchestratorProfile: (name: string) =>
		request<{ status: string }>("/api/config/user/orchestrator-profiles/active", { method: "PUT", body: JSON.stringify({ name }) }),
	resyncOrchestratorProfiles: () =>
		request<OrchestratorProfilesResponse>("/api/config/user/orchestrator-profiles/resync", { method: "POST" }),
	updateOrchestratorProfile: (
		name: string,
		profile: { display_name?: string; description?: string; agents: Record<string, { model: string }> },
	) =>
		request<OrchestratorProfilesResponse & { agents_applied: number }>(
			`/api/config/user/orchestrator-profiles/${encodeURIComponent(name)}`,
			{ method: "PUT", body: JSON.stringify(profile) },
		),

	// Native directory picker
	browseDirectory: () =>
		request<{ path: string }>("/api/browse-directory", { method: "POST" }),

	// AGENTS.md
	getAgentsMd: () =>
		request<{ path: string; content: string }>("/api/agents-md"),
	saveAgentsMd: (content: string) =>
		request<{ status: string; path: string }>("/api/agents-md", {
			method: "PUT",
			body: JSON.stringify({ content }),
		}),
};

// ─── Memories API ───────────────────────────────────────────────────────────

export const memoriesApi = {
	// Engram status
	status: () => request<EngramStatus>("/missions/api/engram/status"),

	// Observations
	listObservations: (limit = 50) =>
		request<{ observations: EngramObservation[] }>(
			`/missions/api/engram/observations?limit=${limit}`,
		).then((r) => r.observations ?? []),
	getObservation: (id: string) =>
		request<EngramObservation>(`/missions/api/engram/observations/${id}`),
	updateObservation: (
		id: string,
		data: {
			content?: string;
			title?: string;
			type?: string;
			scope?: string;
			project?: string;
			topic_key?: string;
		},
	) =>
		request<EngramObservation>(`/missions/api/engram/observations/${id}`, {
			method: "PATCH",
			body: JSON.stringify(data),
		}),
	deleteObservation: (id: string) =>
		fetch(`/missions/api/engram/observations/${id}`, {
			method: "DELETE",
		}).then((r) => {
			if (!r.ok) throw new Error(`${r.status}`);
		}),
	save: (data: {
		type: string;
		content: string;
		title?: string;
		scope?: string;
		project?: string;
	}) =>
		request<EngramObservation>("/missions/api/engram/save", {
			method: "POST",
			body: JSON.stringify(data),
		}),

	// Search / stats / sessions / timeline / context
	search: (q: string, limit = 50, type?: string) =>
		request<{ observations: EngramObservation[] }>(
			`/missions/api/engram/search?q=${encodeURIComponent(q)}&limit=${limit}${
				type ? `&type=${encodeURIComponent(type)}` : ""
			}`,
		).then((r) => r.observations ?? []),
	stats: () => request<EngramStats>("/missions/api/engram/stats"),
	listSessions: (limit = 50) =>
		request<{ sessions: EngramSession[] }>(
			`/missions/api/engram/sessions?limit=${limit}`,
		).then((r) => r.sessions ?? []),
	deleteSession: (id: string) =>
		fetch(`/missions/api/engram/sessions/${id}`, { method: "DELETE" }).then(
			(r) => {
				if (!r.ok) throw new Error(`${r.status}`);
			},
		),
	listPrompts: (limit = 100) =>
		request<{ prompts: EngramPrompt[] }>(
			`/missions/api/engram/prompts?limit=${limit}`,
		).then((r) => r.prompts ?? []),
	deletePrompt: (id: string) =>
		fetch(`/missions/api/engram/prompts/${id}`, { method: "DELETE" }).then(
			(r) => {
				if (!r.ok) throw new Error(`${r.status}`);
			},
		),
	exportAll: async (): Promise<Blob> => {
		const res = await fetch("/missions/api/engram/export");
		if (!res.ok) throw new Error(`${res.status}`);
		return res.blob();
	},
	importData: async (file: File): Promise<EngramImportResult> => {
		const res = await fetch("/missions/api/engram/import", {
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: file,
		});
		if (!res.ok) throw new Error(`${res.status}: ${await res.text()}`);
		return res.json();
	},
	mergeProjects: (source: string, target: string) =>
		request<EngramMergeResult>("/missions/api/engram/projects/merge", {
			method: "POST",
			body: JSON.stringify({ source, target }),
		}),

	runRecallEval: (req: MemoryEvalRequest = {}) =>
		request<MemoryEvalResult>("/missions/api/engram/memory-evals", {
			method: "POST",
			body: JSON.stringify(req),
		}),
	timeline: (observationId?: string, limit = 50) =>
		request<{ events: EngramTimelineEvent[] }>(
			`/missions/api/engram/timeline?limit=${limit}${observationId ? `&observation_id=${encodeURIComponent(observationId)}` : ""}`,
		).then((r) => r.events ?? []),
	context: (q?: string, limit = 100) =>
		request<EngramContextResult>(
			`/missions/api/engram/context?limit=${limit}${
				q ? `&q=${encodeURIComponent(q)}` : ""
			}`,
		),
	saveContext: (text: string) =>
		request<EngramContextResult>("/missions/api/engram/context", {
			method: "PUT",
			body: JSON.stringify({ context: text }),
		}),

	// Consolidation
	startConsolidation: (data: {
		model: string;
		agent: string;
		topic_key?: string;
		project?: string;
	}) =>
		request<{ run_id: string; status: string }>(
			"/missions/api/engram/consolidations",
			{ method: "POST", body: JSON.stringify(data) },
		),
	getConsolidation: (id: string) =>
		request<ConsolidationRun>(`/missions/api/engram/consolidations/${id}`),
	applyConsolidation: (id: string, sel: ApplySelection) =>
		request<{ status: string }>(
			`/missions/api/engram/consolidations/${id}/apply`,
			{ method: "POST", body: JSON.stringify(sel) },
		),
	discardConsolidation: (id: string) =>
		request<{ status: string }>(
			`/missions/api/engram/consolidations/${id}/discard`,
			{ method: "POST" },
		),
};

// ─── Workflow Studio API ───────────────────────────────────────────────────

export const workflowApi = {
	// List + create.
	list: () =>
		request<{ workflows: WorkflowSummary[] }>("/api/workflows"),
	get: (name: string) => request<Workflow>(`/api/workflows/${name}`),
	create: (wf: Workflow) =>
		request<Workflow>("/api/workflows", {
			method: "POST",
			body: JSON.stringify(wf),
		}),
	save: (name: string, wf: Workflow) =>
		request<Workflow>(`/api/workflows/${name}`, {
			method: "PUT",
			body: JSON.stringify(wf),
		}),
	delete: (name: string) =>
		fetch(`${BASE}/api/workflows/${name}`, { method: "DELETE" }).then((r) => {
			if (!r.ok) throw new Error(`${r.status}`);
		}),

	// Import cc-wf-studio JSON. Accepts raw JSON or {json, name}.
	import: (raw: unknown, name?: string) =>
		request<WorkflowImportResult>("/api/workflows/import", {
			method: "POST",
			body: JSON.stringify(name !== undefined ? { json: raw, name } : raw),
		}),

	// Validate the stored workflow.
	validate: (name: string) =>
		request<WorkflowValidationResult>(
			`/api/workflows/${name}/validate`,
			{ method: "POST" },
		),

	// Export. Dry-run (preview the file plan) by default; pass apply:true to
	// actually write the opencode artifacts to ~/.config/opencode.
	export: (name: string, apply = false, target = "opencode") => {
		const params = new URLSearchParams();
		if (apply) params.set("apply", "true");
		if (target && target !== "opencode") params.set("target", target);
		const qs = params.toString();
		return request<WorkflowExportPlan>(
			`/api/workflows/${name}/export${qs ? `?${qs}` : ""}`,
			{ method: "POST" },
		);
	},

	// Edit with AI. Applies a natural-language instruction via the opencode CLI
	// and returns the proposed workflow (not saved) plus its validation.
	aiEdit: (name: string, instruction: string, model?: string) =>
		request<{ workflow: Workflow; validation: WorkflowValidationResult }>(
			`/api/workflows/${name}/ai-edit`,
			{ method: "POST", body: JSON.stringify({ instruction, model }) },
		),

	// Read-only catalogs the node editors populate from.
	listSkills: () =>
		request<{ name: string; description: string }[]>("/api/workflows-meta/skills"),
	// Real MCP servers configured in opencode.json (not the static catalog).
	listMcpServers: () =>
		request<{ id: string; enabled: boolean }[]>("/api/workflows-meta/mcps"),
	// Static catalog (used to suggest known tools for a server, best-effort).
	listMcps: () => request<McpCatalogItem[]>("/api/mcp/catalog"),
};

// McpCatalogItem mirrors the control server's MCP catalog entry (subset used by
// the MCP node editor).
export interface McpCatalogItem {
	id: string;
	name: string;
	tools: string[];
	installed: boolean;
	enabled: boolean;
}
