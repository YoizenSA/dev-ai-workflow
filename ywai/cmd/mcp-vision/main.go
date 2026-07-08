// Tokenbank Vision MCP Server
//
// Exposes analyze_image and list_vision_models as MCP tools backed by
// Tokenbank's vision-capable models. Communicates over stdio via
// newline-delimited JSON-RPC (standard MCP transport).

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("mcp-vision: %v", err)
	}
}

func run() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.TokenBankURL == "" {
		return fmt.Errorf("tokenbank_url not set in config")
	}
	if cfg.TokenBankAPIKey == "" {
		return fmt.Errorf("tokenbank_api_key not set in config")
	}

	s := &server{
		baseURL:     strings.TrimRight(cfg.TokenBankURL, "/"),
		apiKey:      cfg.TokenBankAPIKey,
		httpCli:     &http.Client{Timeout: 60 * time.Second},
		visionModel: cfg.GetVisionModel(),
		visionModels: []visionModel{
			{ID: "mimo-v2.5", Name: "MiMo V2.5", Pricing: "$0.15 per 1M input tokens"},
			{ID: "kimi-k2.6", Name: "Kimi K2.6", Pricing: "$0.40 per 1M input tokens"},
			{ID: "qwen3.6-plus", Name: "Qwen 3.6 Plus", Pricing: "$0.50 per 1M input tokens"},
			{ID: "mimo-v2.5-pro", Name: "MiMo V2.5 Pro", Pricing: "$0.50 per 1M input tokens"},
		},
	}
	return s.serve()
}

// ─── MCP Protocol ───────────────────────────────────────────────────────────

type server struct {
	baseURL      string
	apiKey       string
	httpCli      *http.Client
	visionModels []visionModel
	visionModel  string // effective default vision model from user config
}

type visionModel struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Pricing string `json:"pricing"`
}

// jsonrpcMessage is a generic JSON-RPC 2.0 envelope.
type jsonrpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // absent for notifications
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// jsonrpcResponse is a JSON-RPC 2.0 response (success).
type jsonrpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      any         `json:"id"`
	Result  any         `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *server) serve() error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 10MB buffer for large base64 payloads

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var msg jsonrpcMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue // skip malformed
		}
		// Notifications have no id — skip them.
		if len(msg.ID) == 0 {
			continue
		}

		resp := s.handle(msg)
		resp.JSONRPC = "2.0"
		resp.ID = msg.ID // preserve the raw id as-is

		out, err := json.Marshal(resp)
		if err != nil {
			return fmt.Errorf("marshal response: %w", err)
		}
		fmt.Println(string(out))
	}
	return scanner.Err()
}

func (s *server) handle(msg jsonrpcMessage) jsonrpcResponse {
	switch msg.Method {
	case "initialize":
		return s.handleInitialize()
	case "tools/list":
		return s.handleToolsList()
	case "tools/call":
		return s.handleToolsCall(msg.Params)
	default:
		return jsonrpcResponse{Error: &rpcError{Code: -32601, Message: "method not found"}}
	}
}

// ─── Initialize ─────────────────────────────────────────────────────────────

type initializeResult struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    map[string]any  `json:"capabilities"`
	ServerInfo      serverInfo      `json:"serverInfo"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (s *server) handleInitialize() jsonrpcResponse {
	return jsonrpcResponse{
		Result: initializeResult{
			ProtocolVersion: "0.1.0",
			Capabilities:    map[string]any{"tools": map[string]any{}},
			ServerInfo: serverInfo{
				Name:    "tokenbank-vision",
				Version: "0.1.0",
			},
		},
	}
}

// ─── Tools / List ───────────────────────────────────────────────────────────

func (s *server) handleToolsList() jsonrpcResponse {
	return jsonrpcResponse{
		Result: map[string]any{
			"tools": []any{
				map[string]any{
					"name":        "analyze_image",
					"description": "Analyze an image using a Tokenbank vision model. Provide the image as a base64 data URI and a text prompt describing what to analyze.",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"image": map[string]any{
								"type":        "string",
								"description": "Base64-encoded image as a data URI (e.g. data:image/png;base64,...). Max 10MB.",
							},
							"prompt": map[string]any{
								"type":        "string",
								"description": "The question or instruction about the image.",
							},
							"model": map[string]any{
								"type":        "string",
								"description": "Vision model to use (default: mimo-v2.5).",
								"default":     "mimo-v2.5",
							},
						},
						"required": []any{"image", "prompt"},
					},
				},
				map[string]any{
					"name":        "list_vision_models",
					"description": "List available vision-capable models and their pricing.",
					"inputSchema": map[string]any{
						"type":       "object",
						"properties": map[string]any{},
					},
				},
			},
		},
	}
}

// ─── Tools / Call ───────────────────────────────────────────────────────────

type toolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolCallResult struct {
	Content []toolContent `json:"content"`
}

func (s *server) handleToolsCall(raw json.RawMessage) jsonrpcResponse {
	var params toolsCallParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return jsonrpcResponse{Error: &rpcError{Code: -32602, Message: "invalid params: " + err.Error()}}
	}

	switch params.Name {
	case "analyze_image":
		return s.callAnalyzeImage(params.Arguments)
	case "list_vision_models":
		return s.callListVisionModels()
	default:
		return jsonrpcResponse{Error: &rpcError{Code: -32602, Message: "unknown tool: " + params.Name}}
	}
}

// ─── analyze_image ──────────────────────────────────────────────────────────

type analyzeImageArgs struct {
	Image  string `json:"image"`
	Prompt string `json:"prompt"`
	Model  string `json:"model"`
}

// maxBase64Size is 10MB — base64 data URI payload limit.
const maxBase64Size = 10 * 1024 * 1024

// dataURIRe validates data:image/...;base64,... URIs.
var dataURIRe = regexp.MustCompile(`^data:image/[a-zA-Z]+;base64,[A-Za-z0-9+/=]+$`)

func (s *server) callAnalyzeImage(raw json.RawMessage) jsonrpcResponse {
	var args analyzeImageArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return jsonrpcResponse{Error: &rpcError{Code: -32602, Message: "invalid arguments: " + err.Error()}}
	}
	if args.Image == "" {
		return jsonrpcResponse{Error: &rpcError{Code: -32602, Message: "image is required"}}
	}
	if args.Prompt == "" {
		return jsonrpcResponse{Error: &rpcError{Code: -32602, Message: "prompt is required"}}
	}

	if len(args.Image) > maxBase64Size {
		return jsonrpcResponse{Error: &rpcError{Code: -32602, Message: fmt.Sprintf("image too large: %d bytes (max %d)", len(args.Image), maxBase64Size)}}
	}

	if !dataURIRe.MatchString(args.Image) {
		return jsonrpcResponse{Error: &rpcError{Code: -32602, Message: "image must be a data URI (data:image/...;base64,...)"}}
	}

	model := args.Model
	if model == "" {
		model = s.visionModel
	}

	// Build the OpenAI-compatible chat request.
	payload := buildVisionPayload(model, args.Prompt, args.Image)
	body, err := json.Marshal(payload)
	if err != nil {
		return jsonrpcResponse{Error: &rpcError{Code: -32603, Message: "marshal request: " + err.Error()}}
	}

	req, err := http.NewRequest(http.MethodPost, s.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return jsonrpcResponse{Error: &rpcError{Code: -32603, Message: "create request: " + err.Error()}}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	cli := s.httpCli
	if cli == nil {
		cli = http.DefaultClient
	}
	resp, err := cli.Do(req)
	if err != nil {
		return jsonrpcResponse{Error: &rpcError{Code: -32603, Message: "tokenbank request failed: " + err.Error()}}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return jsonrpcResponse{Error: &rpcError{Code: -32603, Message: "read response: " + err.Error()}}
	}

	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(respBody))
		if len(msg) > 500 {
			msg = msg[:500]
		}
		return jsonrpcResponse{Error: &rpcError{Code: -32603, Message: fmt.Sprintf("tokenbank API error (status %d): %s", resp.StatusCode, msg)}}
	}

	// Parse the OpenAI-compatible response.
	var chatResp chatCompletionResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return jsonrpcResponse{Error: &rpcError{Code: -32603, Message: "parse response: " + err.Error()}}
	}

	var text string
	if len(chatResp.Choices) > 0 {
		text = chatResp.Choices[0].Message.Content
	}

	return jsonrpcResponse{
		Result: toolCallResult{
			Content: []toolContent{{Type: "text", Text: text}},
		},
	}
}

// ─── OpenAI-compatible request / response types ─────────────────────────────

type visionPayload struct {
	Model     string          `json:"model"`
	Messages  []visionMessage `json:"messages"`
	MaxTokens int             `json:"max_tokens,omitempty"`
}

type visionMessage struct {
	Role    string          `json:"role"`
	Content []visionContent `json:"content"`
}

type visionContent struct {
	Type     string         `json:"type"`
	Text     string         `json:"text,omitempty"`
	ImageURL *visionImageURL `json:"image_url,omitempty"`
}

type visionImageURL struct {
	URL string `json:"url"`
}

type chatCompletionResponse struct {
	Choices []choice `json:"choices"`
}

type choice struct {
	Message responseMessage `json:"message"`
}

type responseMessage struct {
	Content string `json:"content"`
}

func buildVisionPayload(model, prompt, imageDataURI string) visionPayload {
	return visionPayload{
		Model: model,
		Messages: []visionMessage{
			{
				Role: "user",
				Content: []visionContent{
					{Type: "text", Text: prompt},
					{Type: "image_url", ImageURL: &visionImageURL{URL: imageDataURI}},
				},
			},
		},
		MaxTokens: 1000,
	}
}

// ─── list_vision_models ─────────────────────────────────────────────────────

func (s *server) callListVisionModels() jsonrpcResponse {
	out, _ := json.Marshal(s.visionModels)
	return jsonrpcResponse{
		Result: toolCallResult{
			Content: []toolContent{{Type: "text", Text: string(out)}},
		},
	}
}
