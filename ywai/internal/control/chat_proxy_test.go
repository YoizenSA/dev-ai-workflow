package control

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// liveOpenCodeURL returns a reachable OpenCode base URL or "" to skip.
func liveOpenCodeURL(t *testing.T) string {
	url := strings.TrimRight(os.Getenv("OPENCODE_URL"), "/")
	if url == "" {
		url = "http://localhost:4096"
	}
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(url + "/app")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Skipf("no OpenCode server at %s (set OPENCODE_URL): %v", url, err)
	}
	resp.Body.Close()
	return url
}

// newProxyMux wires the chat proxy routes to a standalone mux for testing.
func newProxyMux(url string) *http.ServeMux {
	cp := NewChatProxy(url)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/chat/sessions", cp.handleListSessions)
	mux.HandleFunc("POST /api/chat/sessions", cp.handleCreateSession)
	mux.HandleFunc("GET /api/chat/sessions/{id}", cp.handleGetMessages)
	mux.HandleFunc("POST /api/chat/sessions/{id}/messages", cp.handleSendMessage)
	return mux
}

func TestChatProxyEndToEnd(t *testing.T) {
	url := liveOpenCodeURL(t)
	srv := httptest.NewServer(newProxyMux(url))
	defer srv.Close()

	// Create a session.
	resp, err := http.Post(srv.URL+"/api/chat/sessions", "application/json", strings.NewReader(""))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	var session struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&session)
	resp.Body.Close()
	if session.ID == "" {
		t.Fatal("create session returned no id")
	}

	// List sessions must be wrapped in {"sessions": [...]} and contain ours.
	resp, err = http.Get(srv.URL + "/api/chat/sessions")
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	var listed struct {
		Sessions []struct {
			ID string `json:"id"`
		} `json:"sessions"`
	}
	json.NewDecoder(resp.Body).Decode(&listed)
	resp.Body.Close()
	found := false
	for _, s := range listed.Sessions {
		if s.ID == session.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("created session %s not in list", session.ID)
	}

	// Send a message; the prompt call blocks until the assistant finishes.
	body := strings.NewReader(`{"content":"reply with the single word: pong"}`)
	resp, err = http.Post(srv.URL+"/api/chat/sessions/"+session.ID+"/messages", "application/json", body)
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("send message status = %d", resp.StatusCode)
	}

	// Message history must flatten to {id, role, content}.
	resp, err = http.Get(srv.URL + "/api/chat/sessions/" + session.ID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	var msgs struct {
		Messages []struct {
			ID      string `json:"id"`
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	json.NewDecoder(resp.Body).Decode(&msgs)
	resp.Body.Close()
	if len(msgs.Messages) < 2 {
		t.Fatalf("expected >=2 messages (user+assistant), got %d", len(msgs.Messages))
	}
	var haveUser, haveAssistant bool
	for _, m := range msgs.Messages {
		if m.Role == "user" && strings.Contains(m.Content, "pong") {
			haveUser = true
		}
		if m.Role == "assistant" && m.Content != "" {
			haveAssistant = true
		}
	}
	if !haveUser {
		t.Error("user message not found in history")
	}
	if !haveAssistant {
		t.Error("assistant reply not found in history")
	}
}
