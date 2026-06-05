package kanban

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// --- MCP Protocol Types ---

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type InitializeParams struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    json.RawMessage `json:"capabilities"`
	ClientInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"clientInfo"`
}

type InitializeResult struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    json.RawMessage `json:"capabilities"`
	ServerInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
}

type ToolsListResult struct {
	Tools []ToolDefinition `json:"tools"`
}

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type ToolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolsCallResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// --- MCP Adapter ---

// MCPAdapter wraps the kanban server with MCP protocol
type MCPAdapter struct {
	server *Server
	port   int
	client *http.Client
}

// NewMCPAdapter creates a new MCP adapter, using the singleton kanban server.
func NewMCPAdapter() *MCPAdapter {
	s, err := GetOrStart(DefaultUIPort) // Get or start singleton server on UI port
	if err != nil {
		log.Fatalf("kanban: failed to start server: %v", err)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	return &MCPAdapter{
		server: s,
		port:   s.Port(),
		client: client,
	}
}

// Run starts the MCP adapter, reading from stdin and writing to stdout.
func (m *MCPAdapter) Run() {
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue
		}

		resp := m.handleRequest(req)
		if resp != nil {
			data, _ := json.Marshal(resp)
			fmt.Println(string(data))
		}
	}
}

// handleRequest dispatches to the appropriate handler.
func (m *MCPAdapter) handleRequest(req JSONRPCRequest) *JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return m.handleInitialize(req)
	case "tools/list":
		return m.handleToolsList(req)
	case "tools/call":
		return m.handleToolsCall(req)
	default:
		// For unknown methods, return an error if it has an ID (not a notification)
		if req.ID != 0 {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    -32601,
					Message: "Method not found",
				},
			}
		}
		return nil
	}
}

func (m *MCPAdapter) handleInitialize(req JSONRPCRequest) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "ywai-kanban",
				"version": "1.0.0",
			},
		},
	}
}

func (m *MCPAdapter) handleToolsList(req JSONRPCRequest) *JSONRPCResponse {
	tools := []map[string]interface{}{
		{
			"name":        "kanban_create_session",
			"description": "Create a new kanban session",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{
						"type":        "string",
						"description": "Project or repository name",
					},
					"goal": map[string]interface{}{
						"type":        "string",
						"description": "Session goal",
					},
				},
				"required": []string{"goal"},
			},
		},
		{
			"name":        "kanban_create_delegation",
			"description": "Create a new delegation card",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type": "string",
					},
					"agent": map[string]interface{}{
						"type": "string",
						"enum": []string{"architect", "dev", "qa", "reviewer", "devops"},
					},
					"task_summary": map[string]interface{}{
						"type": "string",
					},
					"dependencies": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"required": []string{"session_id", "agent", "task_summary"},
			},
		},
		{
			"name":        "kanban_update_delegation",
			"description": "Update delegation status and column",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type": "string",
					},
					"column": map[string]interface{}{
						"type": "string",
						"enum": []string{"backlog", "ready", "in_progress", "review", "done"},
					},
					"status": map[string]interface{}{
						"type": "string",
						"enum": []string{"pending", "running", "review", "changes", "blocked", "done"},
					},
					"handoff_preview": map[string]interface{}{
						"type": "string",
					},
					"blocker": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []string{"id"},
			},
		},
		{
			"name":        "kanban_list_sessions",
			"description": "List active kanban sessions",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "kanban_get_board",
			"description": "Get kanban board for a session",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []string{"session_id"},
			},
		},
		{
			"name":        "kanban_get_ui_url",
			"description": "Get the Kanban UI URL to open in browser",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "kanban_delete_session",
			"description": "Delete a kanban session and all its delegations",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Session ID to delete",
					},
				},
				"required": []string{"session_id"},
			},
		},
		{
			"name":        "kanban_add_activity",
			"description": "Add a progress update, decision request, or question to a delegation",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"delegation_id": map[string]interface{}{
						"type":        "string",
						"description": "Delegation ID to attach activity to",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Activity type",
						"enum":        []string{"progress", "decision", "question", "blocked"},
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Activity content / message",
					},
					"options": map[string]interface{}{
						"type":        "array",
						"description": "Optional choices for decisions/questions",
						"items":       map[string]interface{}{"type": "string"},
					},
				},
				"required": []string{"delegation_id", "type", "content"},
			},
		},
		{
			"name":        "kanban_get_activities",
			"description": "Get activity timeline for a delegation",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"delegation_id": map[string]interface{}{
						"type":        "string",
						"description": "Delegation ID",
					},
				},
				"required": []string{"delegation_id"},
			},
		},
		{
			"name":        "kanban_get_pending_decisions",
			"description": "Get unresolved decisions, questions, and blockers for a session",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Session ID",
					},
				},
				"required": []string{"session_id"},
			},
		},
		{
			"name":        "kanban_get_graph",
			"description": "Get dependency graph for a session (nodes + edges). Shows task dependencies and helps identify blockers.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Session ID",
					},
				},
				"required": []string{"session_id"},
			},
		},
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
}

func (m *MCPAdapter) handleToolsCall(req JSONRPCRequest) *JSONRPCResponse {
	var params ToolsCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return m.errorResponse(req.ID, -32602, "Invalid params")
	}

	var result interface{}
	var err error

	switch params.Name {
	case "kanban_create_session":
		result, err = m.callCreateSession(params.Arguments)
	case "kanban_create_delegation":
		result, err = m.callCreateDelegation(params.Arguments)
	case "kanban_update_delegation":
		result, err = m.callUpdateDelegation(params.Arguments)
	case "kanban_list_sessions":
		result, err = m.callListSessions()
	case "kanban_get_board":
		result, err = m.callGetBoard(params.Arguments)
	case "kanban_get_ui_url":
		result, err = m.callGetUIURL()
	case "kanban_delete_session":
		result, err = m.callDeleteSession(params.Arguments)
	case "kanban_add_activity":
		result, err = m.callAddActivity(params.Arguments)
	case "kanban_get_activities":
		result, err = m.callGetActivities(params.Arguments)
	case "kanban_get_pending_decisions":
		result, err = m.callGetPendingDecisions(params.Arguments)
	case "kanban_get_graph":
		result, err = m.callGetGraph(params.Arguments)
	default:
		return m.errorResponse(req.ID, -32601, "Tool not found: "+params.Name)
	}

	if err != nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"content": []ToolContent{
					{Type: "text", Text: "Error: " + err.Error()},
				},
				"isError": true,
			},
		}
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (m *MCPAdapter) errorResponse(id int, code int, message string) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
}

// --- HTTP Helpers ---

func (m *MCPAdapter) doRequest(method, path string, body []byte) ([]byte, error) {
	url := fmt.Sprintf("http://localhost:%d%s", m.port, path)
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (m *MCPAdapter) callCreateSession(args json.RawMessage) (*ToolsCallResult, error) {
	var req struct {
		Project string `json:"project"`
		Goal    string `json:"goal"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}

	body, err := json.Marshal(map[string]interface{}{"project": req.Project, "goal": req.Goal})
	if err != nil {
		return nil, err
	}

	respBody, err := m.doRequest("POST", "/api/sessions", body)
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(respBody, &session); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("http://localhost:%d", m.port)
	return &ToolsCallResult{
		Content: []ToolContent{
			{Type: "text", Text: fmt.Sprintf("Session created: %s (goal: %s)\n🚀 Kanban UI: %s", session.ID, session.Goal, url)},
		},
	}, nil
}

func (m *MCPAdapter) callCreateDelegation(args json.RawMessage) (*ToolsCallResult, error) {
	var req struct {
		SessionID    string   `json:"session_id"`
		Agent        string   `json:"agent"`
		TaskSummary  string   `json:"task_summary"`
		Dependencies []string `json:"dependencies"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}

	body, err := json.Marshal(map[string]interface{}{
		"session_id":   req.SessionID,
		"agent":        req.Agent,
		"task_summary": req.TaskSummary,
		"dependencies": req.Dependencies,
	})
	if err != nil {
		return nil, err
	}

	respBody, err := m.doRequest("POST", "/api/delegations", body)
	if err != nil {
		return nil, err
	}

	var delegation Delegation
	if err := json.Unmarshal(respBody, &delegation); err != nil {
		return nil, err
	}

	return &ToolsCallResult{
		Content: []ToolContent{
			{Type: "text", Text: fmt.Sprintf("Delegation created: %s (agent: %s, task: %s)", delegation.ID, delegation.Agent, delegation.TaskSummary)},
		},
	}, nil
}

func (m *MCPAdapter) callUpdateDelegation(args json.RawMessage) (*ToolsCallResult, error) {
	var req struct {
		ID             string `json:"id"`
		Column         string `json:"column"`
		Status         string `json:"status"`
		HandoffPreview string `json:"handoff_preview"`
		Blocker        string `json:"blocker"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}

	bodyMap := make(map[string]string)
	if req.Column != "" {
		bodyMap["column"] = req.Column
	}
	if req.Status != "" {
		bodyMap["status"] = req.Status
	}
	if req.HandoffPreview != "" {
		bodyMap["handoff_preview"] = req.HandoffPreview
	}
	if req.Blocker != "" {
		bodyMap["blocker"] = req.Blocker
	}

	body, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, err
	}

	respBody, err := m.doRequest("PATCH", fmt.Sprintf("/api/delegations/%s", req.ID), body)
	if err != nil {
		return nil, err
	}

	var delegation Delegation
	if err := json.Unmarshal(respBody, &delegation); err != nil {
		return nil, err
	}

	return &ToolsCallResult{
		Content: []ToolContent{
			{Type: "text", Text: fmt.Sprintf("Delegation updated: %s (status: %s, column: %s)", delegation.ID, delegation.Status, delegation.Column)},
		},
	}, nil
}

func (m *MCPAdapter) callListSessions() (*ToolsCallResult, error) {
	respBody, err := m.doRequest("GET", "/api/sessions", nil)
	if err != nil {
		return nil, err
	}

	var sessions []Session
	if err := json.Unmarshal(respBody, &sessions); err != nil {
		return nil, err
	}

	var text string
	if len(sessions) == 0 {
		text = "No active sessions found."
	} else {
		lines := make([]string, 0, len(sessions))
		for _, s := range sessions {
			lines = append(lines, fmt.Sprintf("- %s: %s (%s)", s.ID, s.Goal, s.Status))
		}
		text = fmt.Sprintf("Active sessions (%d):\n%s", len(sessions), strings.Join(lines, "\n"))
	}

	return &ToolsCallResult{
		Content: []ToolContent{
			{Type: "text", Text: text},
		},
	}, nil
}

func (m *MCPAdapter) callGetBoard(args json.RawMessage) (*ToolsCallResult, error) {
	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}

	respBody, err := m.doRequest("GET", fmt.Sprintf("/api/sessions/%s/board", req.SessionID), nil)
	if err != nil {
		return nil, err
	}

	var board BoardView
	if err := json.Unmarshal(respBody, &board); err != nil {
		return nil, err
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Board for session: %s (%s)", board.Session.ID, board.Session.Goal))
	lines = append(lines, "")

	columns := []string{"backlog", "ready", "in_progress", "review", "done"}
	for _, col := range columns {
		delegations := board.Columns[col]
		lines = append(lines, fmt.Sprintf("## %s (%d)", col, len(delegations)))
		for _, d := range delegations {
			lines = append(lines, fmt.Sprintf("- [%s] %s: %s", d.Status, d.Agent, d.TaskSummary))
		}
		lines = append(lines, "")
	}

	return &ToolsCallResult{
		Content: []ToolContent{
			{Type: "text", Text: strings.Join(lines, "\n")},
		},
	}, nil
}

func (m *MCPAdapter) callGetUIURL() (*ToolsCallResult, error) {
	url := fmt.Sprintf("http://localhost:%d", m.port)
	return &ToolsCallResult{
		Content: []ToolContent{
			{Type: "text", Text: fmt.Sprintf("🚀 ywai Kanban UI: %s\n📊 Open this URL in your browser to view the Kanban board", url)},
		},
	}, nil
}

func (m *MCPAdapter) callDeleteSession(args json.RawMessage) (*ToolsCallResult, error) {
	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}

	_, err := m.doRequest("DELETE", fmt.Sprintf("/api/sessions/%s", req.SessionID), nil)
	if err != nil {
		return nil, err
	}

	return &ToolsCallResult{
		Content: []ToolContent{
			{Type: "text", Text: fmt.Sprintf("🗑️ Session %s deleted successfully.", req.SessionID)},
		},
	}, nil
}

func (m *MCPAdapter) callAddActivity(args json.RawMessage) (*ToolsCallResult, error) {
	var req struct {
		DelegationID string   `json:"delegation_id"`
		Type         string   `json:"type"`
		Content      string   `json:"content"`
		Options      []string `json:"options"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}

	body, err := json.Marshal(map[string]interface{}{
		"type":    req.Type,
		"content": req.Content,
		"options": req.Options,
	})
	if err != nil {
		return nil, err
	}

	respBody, err := m.doRequest("POST", fmt.Sprintf("/api/delegations/%s/activities", req.DelegationID), body)
	if err != nil {
		return nil, err
	}

	var activity ActivityEvent
	if err := json.Unmarshal(respBody, &activity); err != nil {
		return nil, err
	}

	return &ToolsCallResult{
		Content: []ToolContent{
			{Type: "text", Text: fmt.Sprintf("Activity added: %s (%s)", activity.ID, activity.Type)},
		},
	}, nil
}

func (m *MCPAdapter) callGetActivities(args json.RawMessage) (*ToolsCallResult, error) {
	var req struct {
		DelegationID string `json:"delegation_id"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}

	respBody, err := m.doRequest("GET", fmt.Sprintf("/api/delegations/%s/activities", req.DelegationID), nil)
	if err != nil {
		return nil, err
	}

	var activities []ActivityEvent
	if err := json.Unmarshal(respBody, &activities); err != nil {
		return nil, err
	}

	lines := []string{fmt.Sprintf("Activities for delegation %s:", req.DelegationID)}
	for _, a := range activities {
		status := ""
		if a.ResolvedAt != nil {
			status = fmt.Sprintf(" [resolved: %s]", a.Resolution)
		} else {
			status = " [pending]"
		}
		lines = append(lines, fmt.Sprintf("- [%s] %s%s", a.Type, a.Content, status))
	}

	return &ToolsCallResult{
		Content: []ToolContent{
			{Type: "text", Text: strings.Join(lines, "\n")},
		},
	}, nil
}

func (m *MCPAdapter) callGetPendingDecisions(args json.RawMessage) (*ToolsCallResult, error) {
	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}

	respBody, err := m.doRequest("GET", fmt.Sprintf("/api/sessions/%s/decisions", req.SessionID), nil)
	if err != nil {
		return nil, err
	}

	var activities []ActivityEvent
	if err := json.Unmarshal(respBody, &activities); err != nil {
		return nil, err
	}

	if len(activities) == 0 {
		return &ToolsCallResult{
			Content: []ToolContent{
				{Type: "text", Text: "No pending decisions for this session."},
			},
		}, nil
	}

	lines := []string{fmt.Sprintf("Pending decisions for session %s:", req.SessionID)}
	for _, a := range activities {
		opts := ""
		if len(a.Options) > 0 {
			opts = fmt.Sprintf(" [options: %s]", strings.Join(a.Options, ", "))
		}
		lines = append(lines, fmt.Sprintf("- [%s] %s%s (id: %s)", a.Type, a.Content, opts, a.ID))
	}

	return &ToolsCallResult{
		Content: []ToolContent{
			{Type: "text", Text: strings.Join(lines, "\n")},
		},
	}, nil
}

func (m *MCPAdapter) callGetGraph(args json.RawMessage) (*ToolsCallResult, error) {
	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}

	respBody, err := m.doRequest("GET", fmt.Sprintf("/api/sessions/%s/graph", req.SessionID), nil)
	if err != nil {
		return nil, err
	}

	var graph GraphView
	if err := json.Unmarshal(respBody, &graph); err != nil {
		return nil, err
	}

	if len(graph.Nodes) == 0 {
		return &ToolsCallResult{
			Content: []ToolContent{
				{Type: "text", Text: "No delegations in this session."},
			},
		}, nil
	}

	lines := []string{fmt.Sprintf("Dependency graph for session %s:", req.SessionID)}
	lines = append(lines, "")

	// Build a quick lookup for nodes
	nodeMap := map[string]GraphNode{}
	for _, n := range graph.Nodes {
		nodeMap[n.ID] = n
	}

	lines = append(lines, fmt.Sprintf("Nodes (%d):", len(graph.Nodes)))
	for _, n := range graph.Nodes {
		lines = append(lines, fmt.Sprintf("  [%s] %s (%s/%s)", n.ID[:8], n.TaskSummary, n.Agent, n.Status))
	}

	lines = append(lines, "")
	if len(graph.Edges) == 0 {
		lines = append(lines, "Edges: none (no dependencies)")
	} else {
		lines = append(lines, fmt.Sprintf("Edges (%d dependencies):", len(graph.Edges)))
		for _, e := range graph.Edges {
			fromLabel := e.From[:8]
			if n, ok := nodeMap[e.From]; ok {
				fromLabel = n.TaskSummary
			}
			toLabel := e.To[:8]
			if n, ok := nodeMap[e.To]; ok {
				toLabel = n.TaskSummary
			}
			lines = append(lines, fmt.Sprintf("  %s → %s", fromLabel, toLabel))
		}
	}

	// Highlight blocked tasks
	var blocked []string
	for _, n := range graph.Nodes {
		if n.Status == "blocked" {
			blocked = append(blocked, fmt.Sprintf("  🚫 %s (%s)", n.TaskSummary, n.Agent))
		}
	}
	if len(blocked) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Blocked tasks:")
		lines = append(lines, blocked...)
	}

	return &ToolsCallResult{
		Content: []ToolContent{
			{Type: "text", Text: strings.Join(lines, "\n")},
		},
	}, nil
}
