// Package workflows implements the Workflow Studio: a visual multi-agent
// workflow editor whose source-of-truth is JSON on disk and whose export
// target is opencode's native primitives (slash commands + agents + skills).
//
// The on-disk workflow JSON format is stable so existing workflows import unchanged.
package workflows

import "time"

// Node types. See workflow-definition.ts.
const (
	NodeTypeStart           = "start"
	NodeTypeEnd             = "end"
	NodeTypePrompt          = "prompt"
	NodeTypeSubAgent        = "subAgent"
	NodeTypeAskUserQuestion = "askUserQuestion"
	NodeTypeIfElse          = "ifElse"
	NodeTypeSwitch          = "switch"
	NodeTypeBranch          = "branch" // legacy alias of switch
	NodeTypeSkill           = "skill"
	NodeTypeMCP             = "mcp"
	NodeTypeSubAgentFlow    = "subAgentFlow"
	NodeTypeCodex           = "codex"
	NodeTypeGroup           = "group"
)

// Workflow is a directed graph of nodes and connections. The JSON shape is
// stable so workflow JSON round-trips on import/export.
type Workflow struct {
	ID                 string                `json:"id"`
	Name               string                `json:"name"`
	Description        string                `json:"description,omitempty"`
	Version            string                `json:"version"`
	Nodes              []Node                `json:"nodes"`
	Connections        []Connection          `json:"connections"`
	SlashCommandOptions *SlashCommandOptions `json:"slashCommandOptions,omitempty"`
	// ConversationHistory records the AI-refinement chat for this workflow
	// (Edit-with-AI multi-turn). Persisted with the workflow JSON.
	ConversationHistory *ConversationHistory `json:"conversationHistory,omitempty"`
	CreatedAt           time.Time            `json:"createdAt"`
	UpdatedAt           time.Time            `json:"updatedAt"`
}

// Node is a single step in the graph. Type selects which fields of Data apply.
type Node struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
	// ParentID, when set, is the id of a group node this node belongs to. Its
	// Position is then relative to that group's origin (React Flow parent extent).
	ParentID string   `json:"parentId,omitempty"`
	Position Position `json:"position"`
	Data     NodeData `json:"data"`
}

// Position is the canvas coordinate of a node.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Connection is a directed edge from one node's output port to another's input.
type Connection struct {
	From     string `json:"from"`
	To       string `json:"to"`
	FromPort string `json:"fromPort,omitempty"`
	ToPort   string `json:"toPort,omitempty"`
}

// NodeData holds the per-type payload of a node. Fields are optional and only
// the subset relevant to the node's Type is populated. JSON tags use the
// camelCase names from workflow-definition.ts so the formats stay
// interchangeable.
type NodeData struct {
	// common
	Label       string `json:"label,omitempty"`
	OutputPorts int    `json:"outputPorts,omitempty"`

	// group container size (visual only)
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`

	// start / end
	// (label only)

	// subAgent
	Name             string `json:"name,omitempty"`            // agent name (may differ from node Name)
	AgentDescription string `json:"description,omitempty"`     // frontmatter description (≤200 chars)
	AgentDefinition  string `json:"agentDefinition,omitempty"` // system prompt / identity
	Prompt           string `json:"prompt,omitempty"`          // task to perform
	AgentType        string `json:"agentType,omitempty"`       // "claudeCode" | "other"
	Tools            string `json:"tools,omitempty"`           // comma-separated tool names
	Model            string `json:"model,omitempty"`           // sonnet|opus|haiku|inherit|provider/id
	Memory           string `json:"memory,omitempty"`          // user|project|local
	Color            string `json:"color,omitempty"`           // red|blue|green|...
	Mode             string `json:"mode,omitempty"`            // all|primary
	CommandFilePath  string `json:"commandFilePath,omitempty"` // ref to existing agent .md
	CommandScope     string `json:"commandScope,omitempty"`    // user|project
	PluginName       string `json:"pluginName,omitempty"`
	BuiltInType      string `json:"builtInType,omitempty"` // general-purpose|explore|plan
	// DelegateTo lists agent ids (comma-separated) this sub-agent may delegate
	// to, BEYOND what the graph's outgoing edges imply. Use this for utility
	// agents like "finder" that several sub-agents need but aren't part of the
	// main execution flow (no visible edge). Mirrors delegations.json's task map.
	DelegateTo string `json:"delegateTo,omitempty"`

	// askUserQuestion
	QuestionText string           `json:"questionText,omitempty"`
	Options      []QuestionOption `json:"options,omitempty"`

	// prompt
	Variables map[string]string `json:"variables,omitempty"`

	// ifElse
	Condition string `json:"condition,omitempty"`

	// switch
	Expression string         `json:"expression,omitempty"`
	Branches   []SwitchBranch `json:"branches,omitempty"`

	// skill
	SkillPath        string `json:"skillPath,omitempty"`
	Scope            string `json:"scope,omitempty"` // user|project|local
	AllowedTools     string `json:"allowedTools,omitempty"`
	ValidationStatus string `json:"validationStatus,omitempty"` // valid|missing|invalid
	Source           string `json:"source,omitempty"`           // claude|copilot
	ExecutionMode    string `json:"executionMode,omitempty"`    // load|execute
	ExecutionPrompt  string `json:"executionPrompt,omitempty"`

	// mcp
	Server string `json:"server,omitempty"`
	Tool   string `json:"tool,omitempty"`
	// Mode selects how the MCP node is configured at runtime. Default (empty)
	// is treated as MCPModeAIParameterConfig so existing workflows keep their
	// prior behaviour (they carried AIParams).
	//   - manualParameterConfig: server + tool + explicit params
	//   - aiParameterConfig:     server + tool, agent fills params from AIParams
	//   - aiToolSelection:       server only, agent picks tool at runtime from
	//                            TaskDescription
	McpMode string `json:"mcpMode,omitempty"`
	// AIParams is a natural-language description of how the agent should fill the
	// tool's parameters at runtime ("AI Parameter Configuration" feature, used by
	// manualParameterConfig and aiParameterConfig modes).
	AIParams string `json:"aiParams,omitempty"`
	// TaskDescription is the natural-language task used in aiToolSelection mode:
	// the agent queries the MCP server at runtime, selects a tool, and fills its
	// params from this description.
	TaskDescription string `json:"taskDescription,omitempty"`

	// subAgentFlow (reference to a reusable sub-flow defined in the same file)
	FlowID string `json:"flowId,omitempty"`
}

// QuestionOption is one selectable answer for an askUserQuestion node.
type QuestionOption struct {
	ID          string `json:"id,omitempty"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
}

// SwitchBranch is one case of a switch node.
type SwitchBranch struct {
	ID    string `json:"id,omitempty"`
	Label string `json:"label,omitempty"`
	Value string `json:"value,omitempty"`
}

// MCP node configuration modes. See NodeData.Mode.
const (
	MCPModeManualParameterConfig = "manualParameterConfig"
	MCPModeAIParameterConfig     = "aiParameterConfig"
	MCPModeAIToolSelection       = "aiToolSelection"
)

// SlashCommandOptions configures the exported slash command's frontmatter so
// the generated /<workflow> command carries allowed-tools, model, hooks, etc.
// All fields optional; only set fields are emitted to the frontmatter.
type SlashCommandOptions struct {
	ArgumentHint          string         `json:"argumentHint,omitempty"`          // "[arg1] [arg2] | [alt1]"
	AllowedTools          string         `json:"allowedTools,omitempty"`          // comma-separated
	Model                 string         `json:"model,omitempty"`                 // default|sonnet|opus|haiku|inherit
	Context               string         `json:"context,omitempty"`               // default|fork
	DisableModelInvocation bool          `json:"disableModelInvocation,omitempty"` // emit disable-model-invocation: true
	Hooks                 *WorkflowHooks `json:"hooks,omitempty"`
}

// WorkflowHooks is the three Claude Code hook buckets. Each holds a list of
// HookEntry keyed by the hook type (PreToolUse/PostToolUse/Stop).
type WorkflowHooks struct {
	PreToolUse  []HookEntry `json:"PreToolUse,omitempty"`
	PostToolUse []HookEntry `json:"PostToolUse,omitempty"`
	Stop        []HookEntry `json:"Stop,omitempty"`
}

// HookEntry matches Claude Code's hook entry: an optional matcher and the
// actions to run when it fires.
type HookEntry struct {
	Matcher string      `json:"matcher,omitempty"`
	Hooks   []HookAction `json:"hooks"`
}

// HookAction is one runnable hook action.
type HookAction struct {
	Type string `json:"type"` // command | prompt
	// Command is the shell command for type=="command".
	Command string `json:"command,omitempty"`
	// Once, when true, runs the action only once per session.
	Once bool `json:"once,omitempty"`
}

// ConversationHistory records the AI-refinement chat (Edit-with-AI multi-turn)
// for a workflow. Persisted with the workflow JSON so the conversation survives
// reloads.
type ConversationHistory struct {
	SchemaVersion    string               `json:"schemaVersion"`
	Messages         []ConversationMessage `json:"messages"`
	CurrentIteration int                  `json:"currentIteration"`
	MaxIterations    int                  `json:"maxIterations"`
	CreatedAt        time.Time            `json:"createdAt"`
	UpdatedAt        time.Time            `json:"updatedAt"`
}

// ConversationMessage is one turn in the refinement chat.
type ConversationMessage struct {
	ID        string    `json:"id"`
	Sender    string    `json:"sender"` // user | ai
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	IsLoading bool      `json:"isLoading,omitempty"`
	IsError   bool      `json:"isError,omitempty"`
}
