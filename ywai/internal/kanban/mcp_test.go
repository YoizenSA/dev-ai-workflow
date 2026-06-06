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
		if len(toolsRaw) != 12 {
			t.Fatalf("expected 12 tools, got %d", len(toolsRaw))
		}
	} else {
		if len(tools) != 12 {
			t.Fatalf("expected 12 tools, got %d", len(tools))
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

func TestMCPAdapter_AddActivity(t *testing.T) {
	adapter := NewMCPAdapter()

	// Initialize first
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

	// Create a session and delegation first
	createSessionReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: json.RawMessage(
			`{"name":"kanban_create_session","arguments":{"project":"test","goal":"Activity MCP test"}}`,
		),
	}
	resp = adapter.handleRequest(createSessionReq)
	if resp == nil {
		t.Fatal("expected response for create_session")
	}
	result, ok := resp.Result.(*ToolsCallResult)
	if !ok {
		t.Fatalf("expected ToolsCallResult, got %T", resp.Result)
	}
	text := result.Content[0].Text
	parts := strings.Split(text, "Session created: ")
	if len(parts) < 2 {
		t.Fatalf("unexpected text format: %s", text)
	}
	idParts := strings.Split(parts[1], " ")
	sessionID := idParts[0]

	// Create delegation
	createDelReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  json.RawMessage(fmt.Sprintf(`{"name":"kanban_create_delegation","arguments":{"session_id":"%s","agent":"dev","task_summary":"Test activity","dependencies":[]}}`, sessionID)),
	}
	resp = adapter.handleRequest(createDelReq)
	if resp == nil {
		t.Fatal("expected response for create_delegation")
	}
	result, ok = resp.Result.(*ToolsCallResult)
	if !ok {
		t.Fatalf("expected ToolsCallResult, got %T", resp.Result)
	}
	delText := result.Content[0].Text
	delParts := strings.Split(delText, "Delegation created: ")
	if len(delParts) < 2 {
		t.Fatalf("unexpected delegation text format: %s", delText)
	}
	delIDParts := strings.Split(delParts[1], " ")
	delegationID := delIDParts[0]

	// Test kanban_add_activity
	addActivityReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params: json.RawMessage(
			fmt.Sprintf(`{"name":"kanban_add_activity","arguments":{"delegation_id":"%s","type":"progress","content":"Implementing feature"}}`, delegationID),
		),
	}
	resp = adapter.handleRequest(addActivityReq)
	if resp == nil {
		t.Fatal("expected response for add_activity")
	}
	result, ok = resp.Result.(*ToolsCallResult)
	if !ok {
		t.Fatalf("expected ToolsCallResult, got %T", resp.Result)
	}
	actText := result.Content[0].Text
	if !strings.Contains(actText, "Activity added:") {
		t.Fatalf("expected 'Activity added:' in response, got: %s", actText)
	}
	if !strings.Contains(actText, "progress") {
		t.Fatalf("expected 'progress' in response, got: %s", actText)
	}
}

func TestMCPAdapter_GetActivities(t *testing.T) {
	adapter := NewMCPAdapter()

	// Initialize
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

	// Create session and delegation
	createSessionReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: json.RawMessage(
			`{"name":"kanban_create_session","arguments":{"project":"test","goal":"Activity list test"}}`,
		),
	}
	resp = adapter.handleRequest(createSessionReq)
	result, _ := resp.Result.(*ToolsCallResult)
	text := result.Content[0].Text
	parts := strings.Split(text, "Session created: ")
	idParts := strings.Split(parts[1], " ")
	sessionID := idParts[0]

	createDelReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  json.RawMessage(fmt.Sprintf(`{"name":"kanban_create_delegation","arguments":{"session_id":"%s","agent":"dev","task_summary":"Test activity list","dependencies":[]}}`, sessionID)),
	}
	resp = adapter.handleRequest(createDelReq)
	result, _ = resp.Result.(*ToolsCallResult)
	delText := result.Content[0].Text
	delParts := strings.Split(delText, "Delegation created: ")
	delIDParts := strings.Split(delParts[1], " ")
	delegationID := delIDParts[0]

	// Add an activity first
	addReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params: json.RawMessage(
			fmt.Sprintf(`{"name":"kanban_add_activity","arguments":{"delegation_id":"%s","type":"decision","content":"Approve?","options":["yes","no"]}}`, delegationID),
		),
	}
	adapter.handleRequest(addReq)

	// Test kanban_get_activities
	getActivitiesReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params: json.RawMessage(
			fmt.Sprintf(`{"name":"kanban_get_activities","arguments":{"delegation_id":"%s"}}`, delegationID),
		),
	}
	resp = adapter.handleRequest(getActivitiesReq)
	if resp == nil {
		t.Fatal("expected response for get_activities")
	}
	result, ok := resp.Result.(*ToolsCallResult)
	if !ok {
		t.Fatalf("expected ToolsCallResult, got %T", resp.Result)
	}
	actText := result.Content[0].Text
	if !strings.Contains(actText, "Approve?") {
		t.Fatalf("expected activities to contain 'Approve?', got: %s", actText)
	}
	if !strings.Contains(actText, "decision") {
		t.Fatalf("expected 'decision' in activities output, got: %s", actText)
	}
}

func TestMCPAdapter_GetPendingDecisions(t *testing.T) {
	adapter := NewMCPAdapter()

	// Initialize
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

	// Create session and delegation
	createSessionReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: json.RawMessage(
			`{"name":"kanban_create_session","arguments":{"project":"test","goal":"Pending decisions test"}}`,
		),
	}
	resp = adapter.handleRequest(createSessionReq)
	result, _ := resp.Result.(*ToolsCallResult)
	text := result.Content[0].Text
	parts := strings.Split(text, "Session created: ")
	idParts := strings.Split(parts[1], " ")
	sessionID := idParts[0]

	createDelReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  json.RawMessage(fmt.Sprintf(`{"name":"kanban_create_delegation","arguments":{"session_id":"%s","agent":"dev","task_summary":"Test pending","dependencies":[]}}`, sessionID)),
	}
	resp = adapter.handleRequest(createDelReq)
	result, _ = resp.Result.(*ToolsCallResult)
	delText := result.Content[0].Text
	delParts := strings.Split(delText, "Delegation created: ")
	delIDParts := strings.Split(delParts[1], " ")
	delegationID := delIDParts[0]

	// Add a decision activity (should appear in pending)
	addReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params: json.RawMessage(
			fmt.Sprintf(`{"name":"kanban_add_activity","arguments":{"delegation_id":"%s","type":"decision","content":"Choose tech stack","options":["React","Vue"]}}`, delegationID),
		),
	}
	adapter.handleRequest(addReq)

	// Add a progress activity (should NOT appear in pending)
	addProgressReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params: json.RawMessage(
			fmt.Sprintf(`{"name":"kanban_add_activity","arguments":{"delegation_id":"%s","type":"progress","content":"Working..."}}`, delegationID),
		),
	}
	adapter.handleRequest(addProgressReq)

	// Test kanban_get_pending_decisions
	getPendingReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "tools/call",
		Params: json.RawMessage(
			fmt.Sprintf(`{"name":"kanban_get_pending_decisions","arguments":{"session_id":"%s"}}`, sessionID),
		),
	}
	resp = adapter.handleRequest(getPendingReq)
	if resp == nil {
		t.Fatal("expected response for get_pending_decisions")
	}
	result, ok := resp.Result.(*ToolsCallResult)
	if !ok {
		t.Fatalf("expected ToolsCallResult, got %T", resp.Result)
	}
	pendingText := result.Content[0].Text
	if !strings.Contains(pendingText, "Choose tech stack") {
		t.Fatalf("expected pending to contain 'Choose tech stack', got: %s", pendingText)
	}
	if !strings.Contains(pendingText, "decision") {
		t.Fatalf("expected 'decision' in pending output, got: %s", pendingText)
	}
	// Progress should NOT appear in pending
	if strings.Contains(pendingText, "Working...") {
		t.Fatalf("progress activity should NOT appear in pending decisions, got: %s", pendingText)
	}
}
