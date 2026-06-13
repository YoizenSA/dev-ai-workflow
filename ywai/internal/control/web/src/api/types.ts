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
}

// ─── WebSocket Messages ────────────────────────────────────────────────────

export type WSMessage<T = unknown> = {
  type: string
  payload: T
}
