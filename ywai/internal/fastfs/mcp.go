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
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
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
			"description": "Find files in the workspace by glob pattern. Respects .gitignore and returns paths only; use fastfs_search to inspect file contents.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern": map[string]interface{}{"type": "string", "description": "Glob pattern, for example *.go or **/*.ts. Omit to match all files."},
					"max":     map[string]interface{}{"type": "integer", "description": "Maximum number of paths to return (default: 500)."},
				},
			},
		},
		{
			"name":        "fastfs_search",
			"description": "Search file contents with a Go regular expression. Use for text discovery; use codegraph_explore first for symbols, call flows, or other structural questions.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern":          map[string]interface{}{"type": "string", "description": "Go regular expression to match."},
					"glob":             map[string]interface{}{"type": "string", "description": "Optional glob that limits which files are searched."},
					"max_matches":      map[string]interface{}{"type": "integer", "description": "Maximum matches to return."},
					"case_insensitive": map[string]interface{}{"type": "boolean", "description": "Whether to ignore letter case while matching."},
				},
				"required": []string{"pattern"},
			},
		},
		{
			"name":        "fastfs_read_outline",
			"description": "Summarize one text file with its signatures and an abbreviated sample. The path must identify a file, not a directory; use fastfs_find to discover files and fastfs_read_slice for exact lines.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string", "description": "Path to one text file, relative to the workspace or absolute. Directories are not supported."},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "fastfs_read_slice",
			"description": "Read a bounded, inclusive line range from one text file. Use fastfs_read_outline first to locate the relevant lines; reads are capped at 200 lines by default.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":      map[string]interface{}{"type": "string", "description": "Path to one text file, relative to the workspace or absolute."},
					"start":     map[string]interface{}{"type": "integer", "description": "First line to return, using 1-based numbering (default: 1)."},
					"end":       map[string]interface{}{"type": "integer", "description": "Last line to return, inclusive (default: end of file)."},
					"max_lines": map[string]interface{}{"type": "integer", "description": "Maximum number of lines to return (default: 200)."},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "fastfs_stat",
			"description": "Return metadata for a workspace path and FastFS process-cache statistics. Use it to check whether a path is a file or directory before reading it.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string", "description": "Path to inspect, relative to the workspace or absolute."},
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
