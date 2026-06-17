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

export interface OpenCodeConfig {
  provider?: string
  model?: string
  smallModel?: string
  defaultAgent?: string
  maxTokens?: number
  temperature?: number
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
