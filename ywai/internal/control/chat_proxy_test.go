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
			ID    string `json:"id"`
			Role  string `json:"role"`
			Parts []struct {
				Kind string `json:"kind"`
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"messages"`
	}
	json.NewDecoder(resp.Body).Decode(&msgs)
	resp.Body.Close()
	if len(msgs.Messages) < 2 {
		t.Fatalf("expected >=2 messages (user+assistant), got %d", len(msgs.Messages))
	}
	textOf := func(parts []struct {
		Kind string `json:"kind"`
		Text string `json:"text"`
	}) string {
		var s string
		for _, p := range parts {
			if p.Kind == "text" {
				s += p.Text
			}
		}
		return s
	}
	var haveUser, haveAssistant bool
	for _, m := range msgs.Messages {
		if m.Role == "user" && strings.Contains(textOf(m.Parts), "pong") {
			haveUser = true
		}
		if m.Role == "assistant" && textOf(m.Parts) != "" {
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

func TestChatProxyProviders(t *testing.T) {
	url := liveOpenCodeURL(t)
	cp := NewChatProxy(url)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/chat/providers", cp.handleProviders)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/chat/providers")
	if err != nil {
		t.Fatalf("get providers: %v", err)
	}
	var data struct {
		Providers []struct {
			ID     string         `json:"id"`
			Models map[string]any `json:"models"`
		} `json:"providers"`
		Default map[string]string `json:"default"`
	}
	json.NewDecoder(resp.Body).Decode(&data)
	resp.Body.Close()
	if len(data.Providers) == 0 {
		t.Fatal("expected at least one provider")
	}
	if data.Providers[0].ID == "" || len(data.Providers[0].Models) == 0 {
		t.Errorf("provider missing id or models: %+v", data.Providers[0])
	}
}

func TestChatPinsStore(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	s := newChatPinsStore()
	if len(s.list()) != 0 {
		t.Fatal("new store should be empty")
	}
	s.pin("ses_a")
	s.pin("ses_a") // dedup
	s.pin("ses_b")
	if got := s.list(); len(got) != 2 {
		t.Fatalf("expected 2 pins, got %v", got)
	}
	s.unpin("ses_a")
	got := s.list()
	if len(got) != 1 || got[0] != "ses_b" {
		t.Fatalf("expected [ses_b], got %v", got)
	}

	// Persistence: a fresh store reads the same file.
	if got := newChatPinsStore().list(); len(got) != 1 || got[0] != "ses_b" {
		t.Fatalf("pins not persisted, got %v", got)
	}
}

func TestChatTemplatesStore(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	s := newChatTemplatesStore()
	created, ok := s.upsert(promptTemplate{Name: "greet", Content: "hello"})
	if !ok || created.ID == "" {
		t.Fatal("create should assign an id")
	}
	if _, ok := s.upsert(promptTemplate{ID: created.ID, Name: "greet2", Content: "hi"}); !ok {
		t.Fatal("update of existing id should succeed")
	}
	if _, ok := s.upsert(promptTemplate{ID: "missing", Name: "x", Content: "y"}); ok {
		t.Fatal("update of missing id should fail")
	}
	list := s.list()
	if len(list) != 1 || list[0].Name != "greet2" || list[0].Content != "hi" {
		t.Fatalf("unexpected list: %+v", list)
	}
	s.delete(created.ID)
	if len(s.list()) != 0 {
		t.Fatal("delete should empty the store")
	}
}
