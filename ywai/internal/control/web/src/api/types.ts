// ─── Kanban Types ──────────────────────────────────────────────────────────

export interface Session {
  id: string
  project: string
  goal: string
  status: 'active' | 'closed'
  created_at: string
}

export type DelegationStatus =
  | 'pending'
  | 'running'
  | 'review'
  | 'changes'
  | 'blocked'
  | 'done'

export type DelegationColumn =
  | 'backlog'
  | 'ready'
  | 'in_progress'
  | 'review'
  | 'done'

export interface Delegation {
  id: string
  session_id: string
  agent: 'dev' | 'qa' | 'reviewer' | 'architect' | 'devops'
  task_summary: string
  status: DelegationStatus
  column: DelegationColumn
  dependencies: string[]
  created_at: string
  started_at?: string | null
  completed_at?: string | null
  handoff?: string
  handoff_preview?: string
  blocker?: string
  pending_action?: boolean
  latest_activity?: string
}

export type ActivityType = 'progress' | 'decision' | 'question' | 'blocked'

export interface ActivityEvent {
  id: string
  delegation_id: string
  type: ActivityType
  content: string
  options?: string[]
  resolution?: string
  created_at: string
  resolved_at?: string | null
}

export interface BoardUpdate {
  type: string
  payload: unknown
}

export interface BoardView {
  backlog: Delegation[]
  ready: Delegation[]
  in_progress: Delegation[]
  review: Delegation[]
  done: Delegation[]
}

export interface GraphNode {
  id: string
  label: string
  status: string
  agent: string
  task_summary: string
  column: string
  handoff_preview?: string
  pending_action?: boolean
}

export interface GraphEdge {
  from: string
  to: string
}

export interface GraphView {
  session: Session
  nodes: GraphNode[]
  edges: GraphEdge[]
}

// ─── Missions Types ────────────────────────────────────────────────────────

export type MissionStatus =
  | 'pending'
  | 'planning'
  | 'active'
  | 'paused'
  | 'completed'
  | 'failed'
  | 'cancelled'
  | 'validating'

export type FeatureStatus =
  | 'pending'
  | 'in_progress'
  | 'completed'
  | 'failed'
  | 'cancelled'

export interface Feature {
  id: string
  description: string
  status: FeatureStatus
  skillName?: string
  milestone?: string
  preconditions?: string[]
  expectedBehavior?: string[]
  fulfills?: string[]
  workerSessionIds?: string[]
}

export interface Milestone {
  name: string
  description: string
}

export interface PlanMilestone {
  name: string
  description: string
}

export interface PlanFeature {
  id: string
  description: string
  skillName: string
  milestone: string
  preconditions?: string[]
  expectedBehavior?: string[]
  fulfills?: string[]
}

export interface PlanMission {
  name: string
  description: string
  project?: string
  milestones: PlanMilestone[]
  features: PlanFeature[]
  model?: string
  agent?: string
}

export interface Mission {
  id: string
  name: string
  project?: string
  status: MissionStatus
  createdAt: string
  updatedAt: string
  completedAt?: string | null
  features?: Feature[] // undefined in list view, present in detail
  milestones?: Milestone[] // undefined in list view
  featureCount?: number
  milestoneCount?: number
  model?: string
  agent?: string
}

export interface Project {
  name: string
  path: string
  branch?: string
}

export interface GitInfo {
  isGitRepo: boolean
  currentBranch?: string
  branches?: string[]
}

export interface FSEntry {
  name: string
  isDir: boolean
  size?: number
  modTime?: string
}

export interface BrowseFSResponse {
  path: string
  entries: FSEntry[]
}

export interface ModelInfo {
  id: string
  name: string
  provider: string
}

export interface ModelsResponse {
  modelsByProvider: Record<string, ModelInfo[]>
  default: string
}

export interface AgentsResponse {
  agents: string[]
}

export interface FeatureLogLine {
  missionId: string
  featureId: string
  line: string
  timestamp: number
}

export interface FeatureLogsResponse {
  missionId: string
  featureId: string
  content: string
}

// ─── WebSocket Messages ────────────────────────────────────────────────────

export type WSMessage<T = unknown> = {
  type: string
  payload: T
}

// ─── Config Types ──────────────────────────────────────────────────────────

export interface AgentInfo {
  name: string
  size: number
  mode?: string
  permission?: Record<string, string>
  group?: string
}

export interface AgentDetail {
  name: string
  content: string
}

// ─── Agent Delegation Graph ────────────────────────────────────────────────
// Static capability graph derived from each agent's permission.task map.
// Nodes are agents (or ghost targets referenced by an edge but not defined);
// edges run source -> target for task keys whose value is allow/ask. The "*"
// catch-all is surfaced as node attributes, not an edge.

export interface AgentGraphNode {
  id: string
  name: string
  mode?: string
  model?: string
  group?: string
  hasWildcard?: boolean
  wildcardValue?: string
  ghost?: boolean
}

export interface AgentGraphEdge {
  id: string
  source: string
  target: string
  value: 'allow' | 'ask'
}

export interface AgentGraph {
  nodes: AgentGraphNode[]
  edges: AgentGraphEdge[]
}

// ─── Delegation Rules (prompt-body section editor) ─────────────────────────
// The "Delegation Rules" markdown table + "Mandatory Delegation Triggers"
// list that live in an orchestrator agent's prompt body. Edited as structured
// data via GET/PUT /api/config/agents/{name}/delegation-rules.

export interface DelegationRule {
  action: string
  inline: 'Yes' | 'No'
  delegate: string
}

export interface DelegationTrigger {
  name: string
  description: string
}

export interface DelegationRulesResp {
  rules: DelegationRule[]
  triggers: DelegationTrigger[]
  hasRules: boolean
}

export interface MCPToolGroup {
  tools: string[]
  enabled: boolean
}

export interface ToolsResponse {
  built_in: string[]
  all: string[]
  mcp_tools: Record<string, MCPToolGroup>
  plugin_tools: Record<string, string[]>
}

export interface SkillInfo {
  name: string
  hasSkillMD: boolean
  description: string
}

export interface MCPServer {
  name: string
  config: {
    command?: string[]
    url?: string
    enabled: boolean
    type?: 'local' | 'remote'
  }
  enabled: boolean
}

export interface ProviderInfo {
  name: string
  models: Record<string, unknown> // model name -> model config
  npm?: string
  options?: Record<string, unknown>
}

export interface Reference {
  path?: string           // For local directories
  repository?: string     // For git repos (owner/repo or URL)
  branch?: string         // Optional branch for git repos
  description?: string    // Description for agent context
  hidden?: boolean        // Hide from autocomplete
}

export interface OpenCodeConfig {
  provider?: string
  model?: string
  smallModel?: string
  defaultAgent?: string
  maxTokens?: number
  temperature?: number
  references?: Record<string, Reference | string>
}

// ─── User Config (Role Defaults) ──────────────────────────────────────────

export type RoleName =
  | 'planning'
  | 'architect'
  | 'dev'
  | 'frontend'
  | 'backend'
  | 'qa'
  | 'reviewer'
  | 'devops'

export const CANONICAL_ROLES: RoleName[] = [
  'planning',
  'architect',
  'dev',
  'frontend',
  'backend',
  'qa',
  'reviewer',
  'devops',
]

export interface RoleDefault {
  agent?: string
  model?: string
  fallbacks?: string[]
  skills?: string[]
}

export type RoleDefaults = Partial<Record<RoleName, RoleDefault>>

export interface UserConfig {
  default_preset?: string
  default_sdd_mode?: string
  default_persona?: string
  default_scope?: string
  default_tui?: boolean
  default_mcp?: boolean
  agents?: string[]
  colored_output?: boolean
  log_level?: string
  custom_agents_dir?: string
  custom_skills_dir?: string
  tokenbank_url?: string
  tokenbank_api_key?: string
  server?: {
    port?: number
    background?: boolean
    mcp?: boolean
    autostart?: boolean
  }
  role_defaults?: RoleDefaults
}

// ─── Memories Types ──────────────────────────────────────────────────────────

export interface EngramObservation {
  id: number
  sync_id?: string
  session_id?: string
  type?: string
  title?: string
  content?: string
  project?: string
  scope?: string
  topic_key?: string
  revision_count?: number
  duplicate_count?: number
  last_seen_at?: string
  created_at?: string
  updated_at?: string
}

export interface EngramSession {
  id: string
  project?: string
  directory?: string
  started_at?: string
  observation_count: number
}

export interface EngramPrompt {
  id: number
  sync_id?: string
  session_id?: string
  content?: string
  project?: string
  created_at?: string
}

export interface EngramStats {
  total_sessions: number
  total_observations: number
  total_prompts: number
  projects?: string[]
}

export interface EngramImportResult {
  sessions_imported: number
  observations_imported: number
  prompts_imported: number
}

export interface EngramMergeResult {
  source: string
  target: string
  observations_updated: number
}

export interface MemoryFilters {
  query: string
  project: string
  type: string
  scope: string
  limit: 20 | 50 | 100
}

// ─── Memory Recall Eval ──────────────────────────────────────────────────

export interface MemoryEvalRequest {
  sample_size?: number
  k?: number
  project?: string
  min_len?: number
}

export interface MemoryEvalSample {
  prompt_id: number
  session_id: string
  hit: boolean
  hit_rank: number
  precision: number
  snippet: string
}

export interface MemoryEvalMiss {
  prompt_id: number
  content: string
  session_id: string
  project?: string
  top_result?: string
}

export interface MemoryEvalResult {
  started_at: string
  duration_ms: number
  k: number
  project?: string
  total_prompts: number
  evaluable: number
  evaluated: number
  skipped: number
  hit_rate: number
  precision_at_k: number
  mrr: number
  project_hit_rate: number
  project_precision_at_k: number
  samples: MemoryEvalSample[]
  misses: MemoryEvalMiss[]
}

export interface EngramTimelineEvent {
  id: string
  type?: string
  content?: string
  createdAt?: string
}

export interface EngramContextResult {
  context?: string
}

export interface EngramStatus {
  connected: boolean
  source?: string
  version?: string
  error?: string
}

// Consolidation
export type ConsolidationStatus =
  | 'running'
  | 'awaiting_review'
  | 'applying'
  | 'applied'
  | 'discarded'
  | 'failed'

export interface PlanUpdate {
  observation_id: string
  reason: string
  new_content?: string
  new_importance?: number
}
export interface PlanDelete {
  observation_id: string
  reason: string
}
export interface PlanSummary {
  type: string
  content: string
  scope: string
}
export interface ConsolidationPlan {
  updates?: PlanUpdate[]
  deletes?: PlanDelete[]
  new_summaries?: PlanSummary[]
  digest?: string
}

export interface ConsolidationScope {
  topic_key?: string
  project?: string
}

export interface ConsolidationRun {
  id: string
  model: string
  agent: string
  status: ConsolidationStatus
  scope?: ConsolidationScope
  plan?: ConsolidationPlan
  digest?: string
  sessionID?: string
  error?: string
  startedAt?: string
  updatedAt?: string
}

export interface ApplySelection {
  updates: PlanUpdate[]
  deletes: PlanDelete[]
  new_summaries: PlanSummary[]
}

export interface OrchestratorModelMapping {
  agent?: string;
  model: string;
}

export interface OrchestratorProfile {
  name: string;
  display_name?: string;
  description?: string;
  is_seed?: boolean;
  // Keyed by agent name (dev, qa, architect, qa-analyst, migration-planner, …).
  agents?: Record<string, OrchestratorModelMapping>;
}

export interface OrchestratorProfilesResponse {
  profiles: Record<string, OrchestratorProfile>;
  active: string;
}

// ─── Workflow Studio Types ─────────────────────────────────────────────────
// Mirrors internal/workflows/model.go. The JSON shape is stable so workflow
// JSON round-trips on import/export.

export type WorkflowNodeType =
	| 'start'
	| 'end'
	| 'prompt'
	| 'subAgent'
	| 'askUserQuestion'
	| 'ifElse'
	| 'switch'
	| 'branch' // legacy alias of switch
	| 'skill'
	| 'mcp'
	| 'subAgentFlow'
	| 'codex'
	| 'group';

export interface WorkflowPosition {
	x: number;
	y: number;
}

export interface WorkflowQuestionOption {
	id?: string;
	label?: string;
	description?: string;
}

export interface WorkflowSwitchBranch {
	id?: string;
	label?: string;
	value?: string;
}

// Per-type payload. Fields are optional; only the subset relevant to a node's
// type is populated. Matches NodeData in model.go.
export interface WorkflowNodeData {
	// common
	label?: string;
	outputPorts?: number;

	// subAgent
	name?: string;
	description?: string;
	agentDefinition?: string;
	prompt?: string;
	agentType?: string;
	tools?: string;
	model?: string;
	memory?: string;
	color?: string;
	mode?: string;
	commandFilePath?: string;
	commandScope?: string;
	pluginName?: string;
	builtInType?: string;

	// askUserQuestion
	questionText?: string;
	options?: WorkflowQuestionOption[];

	// prompt
	variables?: Record<string, string>;

	// ifElse
	condition?: string;

	// switch
	expression?: string;
	branches?: WorkflowSwitchBranch[];

	// skill
	skillPath?: string;
	scope?: string;
	allowedTools?: string;
	validationStatus?: string;
	source?: string;
	executionMode?: string;
	executionPrompt?: string;

	// group container size (visual only)
	width?: number;
	height?: number;

	// mcp
	server?: string;
	tool?: string;
	/** MCP configuration mode. Empty = aiParameterConfig (backward compat).
	 * NOTE: the Go field is McpMode to avoid clashing with the subAgent Mode;
	 * the JSON tag is "mode" so it round-trips on import/export, but on the TS
	 * side we keep mcpMode to stay type-safe. */
	mcpMode?: 'manualParameterConfig' | 'aiParameterConfig' | 'aiToolSelection';
	/** aiToolSelection mode: natural-language task; agent picks the tool at runtime. */
	taskDescription?: string;
	aiParams?: string;

	// subAgentFlow
	flowId?: string;
}

export interface WorkflowNode {
	id: string;
	type: WorkflowNodeType;
	name: string;
	/** Group node id this node belongs to; position is then relative to it. */
	parentId?: string;
	position: WorkflowPosition;
	data: WorkflowNodeData;
}

export interface WorkflowConnection {
	from: string;
	to: string;
	fromPort?: string;
	toPort?: string;
}

export interface Workflow {
	id: string;
	name: string;
	description?: string;
	version: string;
	nodes: WorkflowNode[];
	connections: WorkflowConnection[];
	slashCommandOptions?: WorkflowSlashCommandOptions;
	conversationHistory?: WorkflowConversationHistory;
	createdAt: string;
	updatedAt: string;
}

export interface WorkflowSummary {
	name: string;
	description: string;
	version: string;
	nodeCount: number;
	updatedAt: string;
}

export interface WorkflowValidationIssue {
	severity: 'error' | 'warning';
	nodeId?: string;
	message: string;
}

export interface WorkflowValidationResult {
	valid: boolean;
	errors: WorkflowValidationIssue[];
	warnings: WorkflowValidationIssue[];
}

export interface WorkflowExportArtifact {
	path: string;
	kind: 'command' | 'agent' | 'skill';
	name: string;
}

export interface WorkflowExportPlan {
	workflowName: string;
	files: WorkflowExportArtifact[];
	/** Rough orchestrator prompt size (chars/4 heuristic). 0 when not computed. */
	estimatedTokens: number;
	dryRun: boolean;
}

// ─── Slash command options (A1) ────────────────────────────────────────────
// All optional; only set fields are emitted to the exported slash command
// frontmatter.

export interface WorkflowSlashCommandOptions {
	argumentHint?: string;
	allowedTools?: string;
	model?: 'default' | 'sonnet' | 'opus' | 'haiku' | 'inherit';
	context?: 'default' | 'fork';
	disableModelInvocation?: boolean;
	hooks?: WorkflowHooks;
}

export interface WorkflowHooks {
	PreToolUse?: WorkflowHookEntry[];
	PostToolUse?: WorkflowHookEntry[];
	Stop?: WorkflowHookEntry[];
}

export interface WorkflowHookEntry {
	matcher?: string;
	hooks: WorkflowHookAction[];
}

export interface WorkflowHookAction {
	type: 'command' | 'prompt';
	command?: string;
	once?: boolean;
}

// ─── Conversation history (D1) ─────────────────────────────────────────────
// AI-refinement chat (Edit-with-AI multi-turn). Persisted with the workflow.

export interface WorkflowConversationHistory {
	schemaVersion: string;
	messages: WorkflowConversationMessage[];
	currentIteration: number;
	maxIterations: number;
	createdAt: string;
	updatedAt: string;
}

export interface WorkflowConversationMessage {
	id: string;
	sender: 'user' | 'ai';
	content: string;
	timestamp: string;
	isLoading?: boolean;
	isError?: boolean;
}

export interface WorkflowImportResult {
	workflow: Workflow;
	warnings?: string[];
}

// ─── Run (A3) ──────────────────────────────────────────────────────────────
// One line of live output from a workflow run, streamed over the WebSocket hub.

export interface WorkflowRunLine {
	stream: 'stdout' | 'stderr';
	text: string;
	ts: number;
}
