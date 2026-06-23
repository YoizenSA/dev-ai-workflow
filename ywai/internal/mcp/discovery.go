package mcp

// discovery.go — tool discovery for MCP servers.
//
// Two exported probes that the rest of ywai uses to learn what tools a
// given MCP server exposes:
//
//   - DiscoverStdio: spawn a stdio subprocess, run initialize + tools/list,
//     return the tool names. The caller controls the timeout via ctx; an
//     8s safety cap is layered on top via context.WithTimeout so a caller
//     passing context.Background() still gets a bounded wait.
//
//   - DiscoverHTTP: POST a single tools/list JSON-RPC request to a remote
//     MCP endpoint, return the tool names. 6s safety cap on the http.Client.
//
// The transport code here was extracted from ywai/internal/kanban/tool_discovery.go
// (the old unexported discoverStdioMCPTools and discoverMCPTools). The kanban
// package now keeps thin wrappers that call into this package; see
// internal/kanban/tool_discovery.go for those.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// DiscoverStdio starts a stdio MCP server subprocess, sends initialize +
// tools/list JSON-RPC requests, and returns the discovered tool names.
// The process is killed after discovery (or when ctx fires, whichever comes
// first). Blocks until discovery completes or ctx is done.
//
// The caller controls the timeout: any deadline in ctx is honored, and a
// maximum 8s cap is layered on top via context.WithTimeout. When the ctx is
// canceled or its deadline is exceeded, the returned error wraps the ctx
// error so callers can use errors.Is(err, context.DeadlineExceeded) or
// errors.Is(err, context.Canceled) to discriminate.
//
// env is an optional map of KEY=VALUE entries injected on top of the parent
// process's environment. Pass nil for no extra env.
//
// An empty tool list is not an error: the server responded validly, the probe
// just found nothing. The function returns ([]string{}, nil) — well, actually
// the zero-length slice is nil; the test pins len() == 0, not non-nil.
func DiscoverStdio(ctx context.Context, command []string, env map[string]string) ([]string, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	reader := bufio.NewReader(stdout)

	// Send initialize request.
	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "ywai-kanban",
				"version": "1.0.0",
			},
		},
	}
	if err := sendJSONRPC(stdin, initReq); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("stdio discovery: %w", ctxErr)
		}
		return nil, fmt.Errorf("send initialize: %w", err)
	}

	// Read initialize response (skip any notifications the server emitted first).
	if _, err := readJSONRPCResponse(reader); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("stdio discovery: %w", ctxErr)
		}
		return nil, fmt.Errorf("read initialize: %w", err)
	}

	// Send initialized notification.
	initialized := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	_ = sendJSONRPC(stdin, initialized)

	// Send tools/list request.
	toolsReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}
	if err := sendJSONRPC(stdin, toolsReq); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("stdio discovery: %w", ctxErr)
		}
		return nil, fmt.Errorf("send tools/list: %w", err)
	}

	// Read tools/list response.
	resp, err := readJSONRPCResponse(reader)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("stdio discovery: %w", ctxErr)
		}
		return nil, fmt.Errorf("read tools/list: %w", err)
	}

	// Parse tool names from result.
	var names []string
	if result, ok := resp["result"].(map[string]interface{}); ok {
		if tools, ok := result["tools"].([]interface{}); ok {
			for _, t := range tools {
				if tool, ok := t.(map[string]interface{}); ok {
					if name, ok := tool["name"].(string); ok && name != "" {
						names = append(names, name)
					}
				}
			}
		}
	}
	return names, nil
}

// DiscoverHTTP POSTs a tools/list JSON-RPC request to a remote MCP endpoint
// and returns the discovered tool names. The caller controls the timeout
// via ctx; a 6s safety cap is layered on top via http.Client.Timeout.
//
// HTTP transport does not implement the MCP initialize handshake in this
// probe — the existing kanban probe did not either, and the discovery
// contract for HTTP servers is "send tools/list, parse the response". When
// ctx is canceled or its deadline is exceeded, the returned *url.Error from
// the http client wraps the ctx error, so errors.Is(err,
// context.DeadlineExceeded) works directly.
//
// An empty tool list is not an error: the endpoint responded validly, the
// probe just found nothing. Returns ([]string{}, nil).
func DiscoverHTTP(ctx context.Context, url string) ([]string, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}
	payload, _ := json.Marshal(reqBody)

	client := &http.Client{Timeout: 6 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var rpcResp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	names := []string{}
	for _, t := range rpcResp.Result.Tools {
		if t.Name != "" {
			names = append(names, t.Name)
		}
	}
	return names, nil
}

// sendJSONRPC writes one newline-delimited JSON-RPC message to w. The MCP
// stdio transport is line-oriented: every message ends with \n and the
// server's response reader (readJSONRPCResponse) splits on that.
func sendJSONRPC(w io.Writer, msg map[string]interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

// readJSONRPCResponse scans reader line-by-line until it finds a JSON object
// with an "id" field (the response to a request, as opposed to a
// server-initiated notification) or the line budget runs out. Notifications
// (e.g. "notifications/message") are skipped.
func readJSONRPCResponse(reader *bufio.Reader) (map[string]interface{}, error) {
	for i := 0; i < 50; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var resp map[string]interface{}
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			continue
		}
		// Skip notifications (no id field).
		if _, ok := resp["id"]; !ok {
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("no response after 50 lines")
}
