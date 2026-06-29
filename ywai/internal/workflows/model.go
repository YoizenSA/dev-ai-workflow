// Package workflows implements the Workflow Studio: a visual multi-agent
// workflow editor whose source-of-truth is JSON on disk and whose export
// target is opencode's native primitives (slash commands + agents + skills).
//
// The on-disk workflow JSON mirrors the cc-wf-studio (Claude Code Workflow
// Studio) format so existing workflows import unchanged.
package workflows

import "time"

// Node types, matching cc-wf-studio's NodeType enum. See workflow-definition.ts.
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

// Workflow is a directed graph of nodes and connections. The JSON shape mirrors
// cc-wf-studio's workflow.json so files round-trip between the two tools.
type Workflow struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Version     string       `json:"version"`
	Nodes       []Node       `json:"nodes"`
	Connections []Connection `json:"connections"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// Node is a single step in the graph. Type selects which fields of Data apply.
type Node struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Name     string   `json:"name"`
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
// camelCase names from cc-wf-studio's workflow-definition.ts so the formats
// stay interchangeable.
type NodeData struct {
	// common
	Label       string `json:"label,omitempty"`
	OutputPorts int    `json:"outputPorts,omitempty"`

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
