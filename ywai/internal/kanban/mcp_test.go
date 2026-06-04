package kanban

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestMCPAdapter_Flow(t *testing.T) {
	// Start MCP adapter
	adapter := NewMCPAdapter()

	// Test handlers directly
	initReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{}`),
	}
	resp := adapter.handleRequest(initReq)
	if resp == nil {
		t.Fatal("expected response for initialize")
	}
	if resp.ID != 1 {
		t.Fatalf("expected id 1, got %d", resp.ID)
	}

	// Test tools/list
	listReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
		Params:  json.RawMessage(`{}`),
	}
	resp = adapter.handleRequest(listReq)
	if resp == nil {
		t.Fatal("expected response for tools/list")
	}
	resultMap, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	tools, ok := resultMap["tools"].([]map[string]interface{})
	if !ok {
		// Try generic []interface{}
		toolsRaw, ok := resultMap["tools"].([]interface{})
		if !ok {
			t.Fatalf("expected tools array, got %T", resultMap["tools"])
		}
		if len(toolsRaw) != 7 {
			t.Fatalf("expected 7 tools, got %d", len(toolsRaw))
		}
	} else {
		if len(tools) != 7 {
			t.Fatalf("expected 7 tools, got %d", len(tools))
		}
	}

	// Test create_session
	createReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"kanban_create_session","arguments":{"goal":"Test session"}}`),
	}
	resp = adapter.handleRequest(createReq)
	if resp == nil {
		t.Fatal("expected response for create_session")
	}

	// Extract session ID from response text
	result, ok := resp.Result.(*ToolsCallResult)
	if !ok {
		resultMap, ok := resp.Result.(map[string]interface{})
		if !ok {
			t.Fatalf("expected ToolsCallResult, got %T", resp.Result)
		}
		contentRaw, ok := resultMap["content"].([]interface{})
		if !ok || len(contentRaw) == 0 {
			t.Fatal("expected content array")
		}
		contentMap, ok := contentRaw[0].(map[string]interface{})
		if !ok {
			t.Fatal("expected content map")
		}
		text, ok := contentMap["text"].(string)
		if !ok {
			t.Fatal("expected text in content")
		}
		// Extract UUID from text like "Session created: xxx (goal: yyy)"
		parts := strings.Split(text, "Session created: ")
		if len(parts) < 2 {
			t.Fatalf("unexpected text format: %s", text)
		}
		idParts := strings.Split(parts[1], " ")
		sessionID := idParts[0]

		// Test create_delegation
		delegationReq := JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      4,
			Method:  "tools/call",
			Params:  json.RawMessage(fmt.Sprintf(`{"name":"kanban_create_delegation","arguments":{"session_id":"%s","agent":"dev","task_summary":"Do work","dependencies":[]}}`, sessionID)),
		}
		resp = adapter.handleRequest(delegationReq)
		if resp == nil {
			t.Fatal("expected response for create_delegation")
		}

		// Test get_board
		boardReq := JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      5,
			Method:  "tools/call",
			Params:  json.RawMessage(fmt.Sprintf(`{"name":"kanban_get_board","arguments":{"session_id":"%s"}}`, sessionID)),
		}
		resp = adapter.handleRequest(boardReq)
		if resp == nil {
			t.Fatal("expected response for get_board")
		}
		resultMap, ok = resp.Result.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map result, got %T", resp.Result)
		}
		contentRaw, ok = resultMap["content"].([]interface{})
		if !ok || len(contentRaw) == 0 {
			t.Fatal("expected content array")
		}
		contentMap, ok = contentRaw[0].(map[string]interface{})
		if !ok {
			t.Fatal("expected content map")
		}
		boardText, ok := contentMap["text"].(string)
		if !ok {
			t.Fatal("expected text in content")
		}
		if !strings.Contains(boardText, "Test session") {
			t.Fatalf("expected board to contain session goal, got: %s", boardText)
		}
		if !strings.Contains(boardText, "Do work") {
			t.Fatalf("expected board to contain delegation task, got: %s", boardText)
		}
	} else {
		// Direct struct access
		text := result.Content[0].Text
		parts := strings.Split(text, "Session created: ")
		if len(parts) < 2 {
			t.Fatalf("unexpected text format: %s", text)
		}
		idParts := strings.Split(parts[1], " ")
		sessionID := idParts[0]

		// Test create_delegation
		delegationReq := JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      4,
			Method:  "tools/call",
			Params:  json.RawMessage(fmt.Sprintf(`{"name":"kanban_create_delegation","arguments":{"session_id":"%s","agent":"dev","task_summary":"Do work","dependencies":[]}}`, sessionID)),
		}
		resp = adapter.handleRequest(delegationReq)
		if resp == nil {
			t.Fatal("expected response for create_delegation")
		}

		// Test get_board
		boardReq := JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      5,
			Method:  "tools/call",
			Params:  json.RawMessage(fmt.Sprintf(`{"name":"kanban_get_board","arguments":{"session_id":"%s"}}`, sessionID)),
		}
		resp = adapter.handleRequest(boardReq)
		if resp == nil {
			t.Fatal("expected response for get_board")
		}
		result, ok := resp.Result.(*ToolsCallResult)
		if !ok {
			t.Fatalf("expected ToolsCallResult, got %T", resp.Result)
		}
		boardText := result.Content[0].Text
		if !strings.Contains(boardText, "Test session") {
			t.Fatalf("expected board to contain session goal, got: %s", boardText)
		}
		if !strings.Contains(boardText, "Do work") {
			t.Fatalf("expected board to contain delegation task, got: %s", boardText)
		}
	}
}

