package opencode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ─── Create Session ────────────────────────────────────────────────────────

func TestServerSession_Create(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/session" {
			t.Errorf("expected POST /session, got %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}

		if body["title"] != "test-mission/feat-1" {
			t.Errorf("expected title 'test-mission/feat-1', got %v", body["title"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    "sess-123",
			"title": body["title"],
		})
	}))
	defer srv.Close()

	api := newServerSessionAPI(srv.URL)
	session, err := api.Create(context.Background(), SessionCreateOpts{
		Title: "test-mission/feat-1",
		Agent: "dev",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if session.ID != "sess-123" {
		t.Errorf("expected session ID 'sess-123', got %q", session.ID)
	}
}

// TestServerSession_Create_OmitsEmptyFields verifies that empty optional fields
// (agent, directory, workspace, parentID) are NOT sent in the request body.
// The opencode server rejects requests that include these as empty strings with
// HTTP 400 {"_tag":"BadRequest"} — they must be omitted entirely.
func TestServerSession_Create_OmitsEmptyFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}

		// title should be present
		if body["title"] != "Goal Refinement" {
			t.Errorf("expected title 'Goal Refinement', got %v", body["title"])
		}
		// Empty optional fields must be omitted, not sent as "".
		for _, key := range []string{"agent", "directory", "workspace", "parentID"} {
			if _, present := body[key]; present {
				t.Errorf("empty field %q must be omitted from request body, got %v", key, body[key])
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"id": "sess-omit"})
	}))
	defer srv.Close()

	api := newServerSessionAPI(srv.URL)
	session, err := api.Create(context.Background(), SessionCreateOpts{
		Title:     "Goal Refinement",
		Agent:     "", // empty — must be omitted
		Directory: "",
		Workspace: "",
		ParentID:  "",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if session.ID != "sess-omit" {
		t.Errorf("expected session ID 'sess-omit', got %q", session.ID)
	}
}

// ─── Prompt ────────────────────────────────────────────────────────────────

func TestServerSession_Prompt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The opencode server expects POST /api/session/{id}/prompt with body
		// {prompt: {text: "..."}} and responds with {data: {id, sessionID, ...}}.
		if r.Method != http.MethodPost || r.URL.Path != "/api/session/sess-123/prompt" {
			t.Errorf("expected POST /api/session/sess-123/prompt, got %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}

		// Body must be {prompt: {text}}, NOT {parts: [...]}.
		prompt, ok := body["prompt"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected body.prompt object, got %v", body["prompt"])
		}
		if prompt["text"] != "Implement feature X" {
			t.Errorf("expected prompt.text 'Implement feature X', got %v", prompt["text"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":        "msg-456",
				"sessionID": "sess-123",
			},
		})
	}))
	defer srv.Close()

	api := newServerSessionAPI(srv.URL)
	result, err := api.Prompt(context.Background(), "sess-123", PromptInput{
		Text:     "Implement feature X",
		Delivery: "immediate",
	})
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if result.MessageID != "msg-456" {
		t.Errorf("expected messageID 'msg-456', got %q", result.MessageID)
	}
	if result.SessionID != "sess-123" {
		t.Errorf("expected sessionID 'sess-123', got %q", result.SessionID)
	}
}

// ─── Wait ───────────────────────────────────────────────────────────────────

func TestServerSession_Wait(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/session/sess-123/wait" {
			t.Errorf("expected POST /api/session/sess-123/wait, got %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	api := newServerSessionAPI(srv.URL)
	if err := api.Wait(context.Background(), "sess-123"); err != nil {
		t.Fatalf("Wait: %v", err)
	}
}

// ─── Messages ───────────────────────────────────────────────────────────────

func TestServerSession_Messages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/session/sess-123/message" {
			t.Errorf("expected GET /api/session/sess-123/message, got %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}

		// The opencode server wraps the message list in {data: [...], cursor: {...}}.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "msg-1", "role": "user", "text": "do stuff"},
				{"id": "msg-2", "role": "assistant", "text": "done"},
			},
			"cursor": map[string]interface{}{"previous": nil, "next": nil},
		})
	}))
	defer srv.Close()

	api := newServerSessionAPI(srv.URL)
	messages, err := api.Messages(context.Background(), "sess-123")
	if err != nil {
		t.Fatalf("Messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Role != "user" {
		t.Errorf("expected first message role 'user', got %q", messages[0].Role)
	}
	if messages[1].Role != "assistant" {
		t.Errorf("expected second message role 'assistant', got %q", messages[1].Role)
	}
}

// ─── Delete ─────────────────────────────────────────────────────────────────

func TestServerSession_Delete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/session/sess-123" {
			t.Errorf("expected DELETE /session/sess-123, got %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	api := newServerSessionAPI(srv.URL)
	if err := api.Delete(context.Background(), "sess-123"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

// ─── Questions ──────────────────────────────────────────────────────────────

func TestServerSession_ListQuestions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/question" {
			t.Errorf("expected GET /question, got %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "q-1", "text": "What framework?", "sessionID": "sess-123"},
		})
	}))
	defer srv.Close()

	api := newServerSessionAPI(srv.URL)
	questions, err := api.ListQuestions(context.Background())
	if err != nil {
		t.Fatalf("ListQuestions: %v", err)
	}
	if len(questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(questions))
	}
	if questions[0].ID != "q-1" {
		t.Errorf("expected question ID 'q-1', got %q", questions[0].ID)
	}
}

func TestServerSession_ReplyQuestion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/question/q-1/reply" {
			t.Errorf("expected POST /question/q-1/reply, got %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	api := newServerSessionAPI(srv.URL)
	if err := api.ReplyQuestion(context.Background(), "q-1", "React"); err != nil {
		t.Fatalf("ReplyQuestion: %v", err)
	}
}

// ─── Error Cases ────────────────────────────────────────────────────────────

func TestServerSession_Create_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	api := newServerSessionAPI(srv.URL)
	_, err := api.Create(context.Background(), SessionCreateOpts{Title: "test"})
	if err == nil {
		t.Fatal("expected error from 500 response")
	}
}

func TestServerSession_Wait_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Never respond — simulates a long-running agent
		select {}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1)
	defer cancel()

	api := newServerSessionAPI(srv.URL)
	err := api.Wait(ctx, "sess-123")
	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
}

// ─── Local Session Stub ────────────────────────────────────────────────────

func TestLocalSessionAPI_AllMethodsError(t *testing.T) {
	api := &localSessionAPI{}
	ctx := context.Background()

	if _, err := api.Create(ctx, SessionCreateOpts{}); err == nil {
		t.Error("Create should error")
	}
	if _, err := api.Get(ctx, "x"); err == nil {
		t.Error("Get should error")
	}
	if _, err := api.Status(ctx); err == nil {
		t.Error("Status should error")
	}
	if _, err := api.Prompt(ctx, "x", PromptInput{}); err == nil {
		t.Error("Prompt should error")
	}
	if err := api.Wait(ctx, "x"); err == nil {
		t.Error("Wait should error")
	}
	if _, err := api.Messages(ctx, "x"); err == nil {
		t.Error("Messages should error")
	}
	if err := api.Delete(ctx, "x"); err == nil {
		t.Error("Delete should error")
	}
	if _, err := api.ListQuestions(ctx); err == nil {
		t.Error("ListQuestions should error")
	}
	if err := api.ReplyQuestion(ctx, "x", "a"); err == nil {
		t.Error("ReplyQuestion should error")
	}
}

// ─── ServerClient.Sessions() ───────────────────────────────────────────────

func TestServerClient_Sessions_ReturnsAPI(t *testing.T) {
	c := NewServerClient("http://127.0.0.1:4096")
	sa := c.Sessions()
	if sa == nil {
		t.Fatal("Sessions() should not return nil")
	}
	// Should return the same instance on repeated calls
	sa2 := c.Sessions()
	if sa != sa2 {
		t.Error("Sessions() should return the same instance")
	}
}

func TestLocalClient_Sessions_ReturnsStub(t *testing.T) {
	c := NewLocalClient()
	sa := c.Sessions()
	if sa == nil {
		t.Fatal("Sessions() should not return nil")
	}
	// The stub should return ErrSessionsUnavailable for all calls
	if err := sa.Wait(context.Background(), "x"); err == nil {
		t.Error("local Sessions().Wait() should error")
	}
}

// ─── Client Interface Compliance ───────────────────────────────────────────

func TestServerClient_ImplementsClient(t *testing.T) {
	var _ Client = (*ServerClient)(nil)
}

func TestLocalClient_ImplementsClient(t *testing.T) {
	var _ Client = (*LocalClient)(nil)
}
