package fastfs

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// MCPAdapter speaks JSON-RPC MCP over stdio for the ywai-fastfs server.
type MCPAdapter struct {
	svc *Service
}

// NewMCPAdapter creates an adapter rooted at workspace (empty = cwd).
func NewMCPAdapter(workspace string) (*MCPAdapter, error) {
	svc, err := NewService(workspace)
	if err != nil {
		return nil, err
	}
	return &MCPAdapter{svc: svc}, nil
}

// Run reads newline-delimited JSON-RPC from stdin and writes responses to stdout.
func (m *MCPAdapter) Run() {
	sc := bufio.NewScanner(os.Stdin)
	// Allow large tool payloads.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue
		}
		if resp := m.handle(req); resp != nil {
			data, _ := json.Marshal(resp)
			fmt.Println(string(data))
		}
	}
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (m *MCPAdapter) handle(req rpcRequest) *rpcResponse {
	switch req.Method {
	case "initialize":
		return &rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
				"serverInfo":      map[string]interface{}{"name": "ywai-fastfs", "version": "1.0.0"},
			},
		}
	case "notifications/initialized", "initialized":
		return nil
	case "tools/list":
		return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{"tools": toolDefs()}}
	case "tools/call":
		return m.callTool(req)
	case "ping":
		return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{}}
	default:
		if len(req.ID) == 0 || string(req.ID) == "null" {
			return nil
		}
		return &rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32601, Message: "Method not found"},
		}
	}
}

func toolDefs() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "fastfs_find",
			"description": "Glob file paths under the workspace (gitignore-aware, no process spawn). Prefer over bash find/fd.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern": map[string]interface{}{"type": "string", "description": "Glob e.g. *.go or **/*.ts"},
					"max":     map[string]interface{}{"type": "integer", "description": "Max paths (default 500)"},
				},
			},
		},
		{
			"name":        "fastfs_search",
			"description": "Regex content search with mtime file cache and parallel workers. Prefer over bash rg/grep for exploration. For structural questions use codegraph_explore first.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern":           map[string]interface{}{"type": "string", "description": "Go regexp"},
					"glob":              map[string]interface{}{"type": "string", "description": "Optional file glob filter"},
					"max_matches":       map[string]interface{}{"type": "integer"},
					"case_insensitive":  map[string]interface{}{"type": "boolean"},
				},
				"required": []string{"pattern"},
			},
		},
		{
			"name":        "fastfs_read_outline",
			"description": "Summarized file view: signatures + elided sample. Prefer over dumping entire files.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string", "description": "Workspace-relative path"},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "fastfs_read_slice",
			"description": "Read a bounded line range (default max 200 lines). Use after outline when you need exact lines.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":      map[string]interface{}{"type": "string"},
					"start":     map[string]interface{}{"type": "integer", "description": "1-based start line"},
					"end":       map[string]interface{}{"type": "integer", "description": "1-based end line"},
					"max_lines": map[string]interface{}{"type": "integer"},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "fastfs_stat",
			"description": "File metadata + process cache stats (hits/misses).",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string"},
				},
				"required": []string{"path"},
			},
		},
	}
}

func (m *MCPAdapter) callTool(req rpcRequest) *rpcResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errResp(req.ID, -32602, err.Error())
	}
	args := map[string]interface{}{}
	if len(params.Arguments) > 0 && string(params.Arguments) != "null" {
		_ = json.Unmarshal(params.Arguments, &args)
	}

	var (
		payload interface{}
		err     error
	)
	switch params.Name {
	case "fastfs_find":
		payload, err = m.svc.Find(FindOptions{
			Pattern: strArg(args, "pattern"),
			Max:     intArg(args, "max"),
		})
	case "fastfs_search":
		payload, err = m.svc.Search(SearchOptions{
			Pattern:         strArg(args, "pattern"),
			Glob:            strArg(args, "glob"),
			MaxMatches:      intArg(args, "max_matches"),
			CaseInsensitive: boolArg(args, "case_insensitive"),
		})
	case "fastfs_read_outline":
		payload, err = m.svc.ReadOutline(strArg(args, "path"))
	case "fastfs_read_slice":
		payload, err = m.svc.ReadSlice(strArg(args, "path"), intArg(args, "start"), intArg(args, "end"), intArg(args, "max_lines"))
	case "fastfs_stat":
		payload, err = m.svc.Stat(strArg(args, "path"))
	default:
		return errResp(req.ID, -32601, "unknown tool: "+params.Name)
	}
	if err != nil {
		return &rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"content": []map[string]interface{}{{"type": "text", "text": err.Error()}},
				"isError": true,
			},
		}
	}
	text, _ := json.MarshalIndent(payload, "", "  ")
	return &rpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{{"type": "text", "text": string(text)}},
		},
	}
}

func errResp(id json.RawMessage, code int, msg string) *rpcResponse {
	return &rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}}
}

func strArg(args map[string]interface{}, key string) string {
	v, _ := args[key].(string)
	return v
}

func intArg(args map[string]interface{}, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	default:
		return 0
	}
}

func boolArg(args map[string]interface{}, key string) bool {
	v, _ := args[key].(bool)
	return v
}
