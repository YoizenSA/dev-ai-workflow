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
		if r.Method != http.MethodPost || r.URL.Path != "/api/session" {
			t.Errorf("expected POST /api/session, got %s %s", r.Method, r.URL.Path)
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
			"data": map[string]interface{}{
				"id":    "sess-123",
				"title": body["title"],
			},
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

// TestServerSession_Create_ModelObject is a regression test for the
// "json: cannot unmarshal object into Go struct field Session.model of type
// string" failure. opencode >= 1.17 returns `model` as an {id, providerID}
// object; the session must decode that without error and expose it via
// SessionModel().
func TestServerSession_Create_ModelObject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/session" {
			t.Errorf("expected POST /api/session, got %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// This is the shape opencode 1.17.9 emits (mirrored from a live probe),
		// wrapped in the v2 {data: {...}} envelope.
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":    "sess-obj",
				"title": "probe",
				"agent": "memory",
				"model": map[string]interface{}{
					"id":         "deepseek-v4-flash",
					"providerID": "fireworks-ai",
					"variant":    "",
				},
			},
		})
	}))
	defer srv.Close()

	api := newServerSessionAPI(srv.URL)
	session, err := api.Create(context.Background(), SessionCreateOpts{
		Title: "probe", Agent: "memory",
	})
	if err != nil {
		t.Fatalf("Create with object model: %v", err)
	}
	if got := session.SessionModel(); got != "fireworks-ai/deepseek-v4-flash" {
		t.Errorf("SessionModel: want %q, got %q", "fireworks-ai/deepseek-v4-flash", got)
	}
}

// TestSessionModel_Shapes covers the tolerant reader across all three shapes.
func TestSessionModel_Shapes(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"object", `{"id":"m","providerID":"p"}`, "p/m"},
		{"object_no_provider", `{"id":"m"}`, "m"},
		{"bare_string", `"provider/model"`, "provider/model"},
		{"empty", ``, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := Session{Model: json.RawMessage(c.raw)}
			if got := s.SessionModel(); got != c.want {
				t.Errorf("want %q, got %q", c.want, got)
			}
		})
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
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{"id": "sess-omit"},
		})
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

// TestServerSession_Create_ModelShape verifies the model object shape: variant
// is only sent when non-empty (older servers 400 on ""), and a model without
// a providerID is dropped entirely (the v2 API 400s on a provider-less model;
// the server default then applies).
func TestServerSession_Create_ModelShape(t *testing.T) {
	cases := []struct {
		name      string
		model     *ModelInput
		wantModel map[string]interface{} // nil means the model key must be absent
	}{
		{
			name:      "provider and id, no variant",
			model:     &ModelInput{ID: "muse-spark-1.3-contributor", ProviderID: "meta"},
			wantModel: map[string]interface{}{"id": "muse-spark-1.3-contributor", "providerID": "meta"},
		},
		{
			name:      "with variant",
			model:     &ModelInput{ID: "m", ProviderID: "p", Variant: "default"},
			wantModel: map[string]interface{}{"id": "m", "providerID": "p", "variant": "default"},
		},
		{
			name:      "bare id without provider is dropped",
			model:     &ModelInput{ID: "bare-model"},
			wantModel: nil,
		},
		{
			name:      "nil model is dropped",
			model:     nil,
			wantModel: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/session" {
					t.Errorf("expected POST /api/session, got %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
					return
				}
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				got, present := body["model"]
				if c.wantModel == nil {
					if present {
						t.Errorf("model must be omitted, got %v", got)
					}
				} else {
					gotMap, ok := got.(map[string]interface{})
					if !ok {
						t.Fatalf("expected model object, got %v", got)
					}
					for k, want := range c.wantModel {
						if gotMap[k] != want {
							t.Errorf("model[%q]: want %q, got %v", k, want, gotMap[k])
						}
					}
					if _, present := gotMap["variant"]; present && c.wantModel["variant"] == nil {
						t.Errorf("empty variant must be omitted, got %v", gotMap["variant"])
					}
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{"id": "sess-m"},
				})
			}))
			defer srv.Close()

			api := newServerSessionAPI(srv.URL)
			session, err := api.Create(context.Background(), SessionCreateOpts{Title: "t", Model: c.model})
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if session.ID != "sess-m" {
				t.Errorf("expected session ID 'sess-m', got %q", session.ID)
			}
		})
	}
}

// TestServerSession_SendsBasicAuth pins the auth wiring: the session client
// must attach the server Basic credentials (env first, then the persisted ywai
// auth file). Without them the v2 API answers 401 on every /api/session route
// — the bare /session path 405s before auth is even checked, which is exactly
// how the Consolidate Memories 405 masked the missing credentials.
func TestServerSession_SendsBasicAuth(t *testing.T) {
	t.Setenv("OPENCODE_SERVER_USERNAME", "opencode")
	t.Setenv("OPENCODE_SERVER_PASSWORD", "test-password")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "opencode" || pass != "test-password" {
			t.Errorf("missing or wrong Basic auth (user=%q ok=%v)", user, ok)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{"id": "sess-auth"},
		})
	}))
	defer srv.Close()

	api := newServerSessionAPI(srv.URL)
	session, err := api.Create(context.Background(), SessionCreateOpts{Title: "t"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if session.ID != "sess-auth" {
		t.Errorf("expected session ID 'sess-auth', got %q", session.ID)
	}
}

// ─── Get / Status ──────────────────────────────────────────────────────────

func TestServerSession_Get(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/session/sess-123" {
			t.Errorf("expected GET /api/session/sess-123, got %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{"id": "sess-123", "title": "probe"},
		})
	}))
	defer srv.Close()

	api := newServerSessionAPI(srv.URL)
	session, err := api.Get(context.Background(), "sess-123")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if session.ID != "sess-123" {
		t.Errorf("expected session ID 'sess-123', got %q", session.ID)
	}
}

func TestServerSession_Status(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The v2 API has no /api/session/status route — status is derived
		// from the session list.
		if r.Method != http.MethodGet || r.URL.Path != "/api/session" {
			t.Errorf("expected GET /api/session, got %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "sess-1", "title": "one"},
				{"id": "sess-2", "title": "two"},
			},
		})
	}))
	defer srv.Close()

	api := newServerSessionAPI(srv.URL)
	result, err := api.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(result.Sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(result.Sessions))
	}
	if result.Sessions[0].ID != "sess-1" || result.Sessions[1].ID != "sess-2" {
		t.Errorf("unexpected sessions: %+v", result.Sessions)
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
		// delivery must be a value opencode accepts ("steer" | "queue"); the
		// legacy "immediate" is now a 400. Guard against accidental regression.
		if body["delivery"] != "steer" {
			t.Errorf("expected delivery 'steer', got %v", body["delivery"])
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
		Delivery: "steer",
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
		if r.Method != http.MethodDelete || r.URL.Path != "/api/session/sess-123" {
			t.Errorf("expected DELETE /api/session/sess-123, got %s %s", r.Method, r.URL.Path)
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
		// Never respond — simulates a long-running agent. Block on the request
		// context (not select{}) so the handler returns once the client cancels;
		// otherwise srv.Close() waits forever on the dangling connection (which
		// hangs the test on Windows).
		<-r.Context().Done()
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

// TestLocalClient_Sessions_ReturnsNil pins the LocalClient contract: without a
// server there is no session API, and consumers nil-check instead of calling
// methods that could never succeed.
func TestLocalClient_Sessions_ReturnsNil(t *testing.T) {
	c := NewLocalClient()
	if sa := c.Sessions(); sa != nil {
		t.Fatal("Sessions() should return nil for the local client")
	}
}

// ─── Client Interface Compliance ───────────────────────────────────────────

func TestServerClient_ImplementsClient(t *testing.T) {
	var _ Client = (*ServerClient)(nil)
}

func TestLocalClient_ImplementsClient(t *testing.T) {
	var _ Client = (*LocalClient)(nil)
}
