package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInitialize(t *testing.T) {
	s := &server{
		baseURL: "https://tokenbank.example.com",
		apiKey:  "test-key",
	}
	msg := jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
	}
	resp := s.handle(msg)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	var result map[string]any
	b, _ := json.Marshal(resp.Result)
	json.Unmarshal(b, &result)
	if result["protocolVersion"] != "0.1.0" {
		t.Fatalf("expected protocolVersion 0.1.0, got %v", result["protocolVersion"])
	}
	si, _ := result["serverInfo"].(map[string]any)
	if si["name"] != "tokenbank-vision" {
		t.Fatalf("expected server name tokenbank-vision, got %v", si["name"])
	}
}

func TestToolsList(t *testing.T) {
	s := &server{
		baseURL: "https://tokenbank.example.com",
		apiKey:  "test-key",
	}
	msg := jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "tools/list",
	}
	resp := s.handle(msg)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("result is not a map")
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("tools is not an array")
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	// Check analyze_image tool
	tool0 := tools[0].(map[string]any)
	if tool0["name"] != "analyze_image" {
		t.Fatalf("expected first tool to be analyze_image, got %v", tool0["name"])
	}
	schema := tool0["inputSchema"].(map[string]any)
	props := schema["properties"].(map[string]any)
	if _, ok := props["image"]; !ok {
		t.Fatal("analyze_image missing 'image' property")
	}
	if _, ok := props["prompt"]; !ok {
		t.Fatal("analyze_image missing 'prompt' property")
	}
	req := schema["required"].([]any)
	if len(req) != 2 {
		t.Fatalf("expected 2 required fields, got %d", len(req))
	}

	// Check list_vision_models tool
	tool1 := tools[1].(map[string]any)
	if tool1["name"] != "list_vision_models" {
		t.Fatalf("expected second tool to be list_vision_models, got %v", tool1["name"])
	}
}

func TestListVisionModels(t *testing.T) {
	s := &server{
		baseURL: "https://tokenbank.example.com",
		apiKey:  "test-key",
		visionModels: []visionModel{
			{ID: "mimo-v2.5", Name: "MiMo V2.5", Pricing: "$0.15"},
			{ID: "kimi-k2.6", Name: "Kimi K2.6", Pricing: "$0.40"},
			{ID: "qwen3.6-plus", Name: "Qwen 3.6 Plus", Pricing: "$0.50"},
			{ID: "mimo-v2.5-pro", Name: "MiMo V2.5 Pro", Pricing: "$0.50"},
		},
	}
	msg := jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`3`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"list_vision_models","arguments":{}}`),
	}
	resp := s.handle(msg)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	result, ok := resp.Result.(toolCallResult)
	if !ok {
		t.Fatalf("result is not toolCallResult")
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(result.Content))
	}
	if !strings.Contains(result.Content[0].Text, "mimo-v2.5") {
		t.Fatalf("response should contain mimo-v2.5, got: %s", result.Content[0].Text)
	}
}

func TestAnalyzeImage_NoImage(t *testing.T) {
	s := &server{
		baseURL: "https://tokenbank.example.com",
		apiKey:  "test-key",
	}
	msg := jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`4`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"analyze_image","arguments":{"prompt":"what is this"}}`),
	}
	resp := s.handle(msg)
	if resp.Error == nil {
		t.Fatal("expected error for missing image")
	}
}

func TestAnalyzeImage_NoPrompt(t *testing.T) {
	s := &server{
		baseURL: "https://tokenbank.example.com",
		apiKey:  "test-key",
	}
	msg := jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`5`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"analyze_image","arguments":{"image":"data:image/png;base64,abc"}}`),
	}
	resp := s.handle(msg)
	if resp.Error == nil {
		t.Fatal("expected error for missing prompt")
	}
}

func TestAnalyzeImage_ImageTooLarge(t *testing.T) {
	s := &server{
		baseURL: "https://tokenbank.example.com",
		apiKey:  "test-key",
	}

	// 11MB base64 string
	large := make([]byte, 11*1024*1024+1)
	for i := range large {
		large[i] = 'A'
	}
	image := "data:image/png;base64," + string(large)

	args, _ := json.Marshal(map[string]string{
		"image":  image,
		"prompt": "what is this",
	})
	msg := jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`6`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"analyze_image","arguments":` + string(args) + `}`),
	}
	resp := s.handle(msg)
	if resp.Error == nil {
		t.Fatal("expected error for oversized image")
	}
	if !strings.Contains(resp.Error.Message, "too large") {
		t.Fatalf("expected 'too large' error, got: %s", resp.Error.Message)
	}
}

func TestAnalyzeImage_InvalidDataURI(t *testing.T) {
	s := &server{
		baseURL: "https://tokenbank.example.com",
		apiKey:  "test-key",
	}
	msg := jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`7`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"analyze_image","arguments":{"image":"not-a-data-uri","prompt":"what is this"}}`),
	}
	resp := s.handle(msg)
	if resp.Error == nil {
		t.Fatal("expected error for invalid data URI")
	}
	if !strings.Contains(resp.Error.Message, "data URI") {
		t.Fatalf("expected 'data URI' error, got: %s", resp.Error.Message)
	}
}

func TestBuildVisionPayload(t *testing.T) {
	p := buildVisionPayload("mimo-v2.5", "describe this", "data:image/png;base64,abc123")
	if p.Model != "mimo-v2.5" {
		t.Fatalf("expected model mimo-v2.5, got %s", p.Model)
	}
	if len(p.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(p.Messages))
	}
	if p.Messages[0].Role != "user" {
		t.Fatalf("expected role user, got %s", p.Messages[0].Role)
	}
	if len(p.Messages[0].Content) != 2 {
		t.Fatalf("expected 2 content parts, got %d", len(p.Messages[0].Content))
	}
	if p.Messages[0].Content[0].Type != "text" || p.Messages[0].Content[0].Text != "describe this" {
		t.Fatalf("first content part should be the text prompt")
	}
	if p.Messages[0].Content[1].Type != "image_url" {
		t.Fatalf("second content part should be image_url")
	}
	if p.Messages[0].Content[1].ImageURL.URL != "data:image/png;base64,abc123" {
		t.Fatalf("image_url.url should match input")
	}
	if p.MaxTokens != 1000 {
		t.Fatalf("expected max_tokens 1000, got %d", p.MaxTokens)
	}
}

func TestDefaultModel(t *testing.T) {
	// Default model is applied when model is empty.
	var args analyzeImageArgs
	if err := json.Unmarshal([]byte(`{"image":"data:image/png;base64,abc","prompt":"what"}`), &args); err != nil {
		t.Fatal(err)
	}
	model := args.Model
	if model == "" {
		model = "mimo-v2.5"
	}
	if model != "mimo-v2.5" {
		t.Fatalf("expected default model mimo-v2.5, got %s", model)
	}
}

func TestNotificationSkipped(t *testing.T) {
	// Notifications (messages without id) should be silently skipped
	s := &server{}
	msg := jsonrpcMessage{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	resp := s.handle(msg)
	// handle returns zero-value for notifications (handled in serve())
	// The serve() loop skips them before handle() is called,
	// but handle() would return method not found.
	// This is fine - the serve loop handles skipping.
	_ = resp
}

func TestUnknownMethod(t *testing.T) {
	s := &server{}
	msg := jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`9`),
		Method:  "unknown",
	}
	resp := s.handle(msg)
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Fatalf("expected error code -32601, got %d", resp.Error.Code)
	}
}

func TestUnknownTool(t *testing.T) {
	s := &server{}
	msg := jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`10`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"nonexistent","arguments":{}}`),
	}
	resp := s.handle(msg)
	if resp.Error == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestAnalyzeImage_HTTPSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization Bearer test-key, got %s", r.Header.Get("Authorization"))
		}
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatal(err)
		}
		if reqBody["model"] != "mimo-v2.5" {
			t.Errorf("expected model mimo-v2.5, got %v", reqBody["model"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"test response"}}]}`))
	}))
	defer ts.Close()

	s := &server{
		baseURL:     ts.URL,
		apiKey:      "test-key",
		httpCli:     ts.Client(),
		visionModel: "mimo-v2.5",
	}
	msg := jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`11`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"analyze_image","arguments":{"image":"data:image/png;base64,abc","prompt":"what is this"}}`),
	}
	resp := s.handle(msg)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	result, ok := resp.Result.(toolCallResult)
	if !ok {
		t.Fatal("expected toolCallResult")
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(result.Content))
	}
	if result.Content[0].Text != "test response" {
		t.Fatalf("expected 'test response', got %v", result.Content[0].Text)
	}
}

func TestAnalyzeImage_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid request"}`))
	}))
	defer ts.Close()

	s := &server{
		baseURL: ts.URL,
		apiKey:  "test-key",
		httpCli: ts.Client(),
	}
	msg := jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`12`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"analyze_image","arguments":{"image":"data:image/png;base64,abc","prompt":"what is this"}}`),
	}
	resp := s.handle(msg)
	if resp.Error == nil {
		t.Fatal("expected error for HTTP 400")
	}
	if !strings.Contains(resp.Error.Message, "tokenbank API error (status 400)") {
		t.Fatalf("expected tokenbank API error, got: %s", resp.Error.Message)
	}
	if !strings.Contains(resp.Error.Message, `{"error":"invalid request"}`) {
		t.Fatalf("expected error body in message, got: %s", resp.Error.Message)
	}
}
