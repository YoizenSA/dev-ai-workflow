package evals

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "modernc.org/sqlite"
)

func newRunnerDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+t.TempDir()+"/oc.db")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE session_v2 (
		id text PRIMARY KEY, project_id text NOT NULL, slug text NOT NULL,
		directory text NOT NULL, title text, version text NOT NULL,
		cost real DEFAULT 0 NOT NULL, tokens_input integer DEFAULT 0 NOT NULL,
		tokens_output integer DEFAULT 0 NOT NULL, agent text, model text,
		time_created integer NOT NULL, time_updated integer NOT NULL);
	CREATE TABLE session_message (
		id text PRIMARY KEY, session_id text NOT NULL, type text NOT NULL,
		seq integer NOT NULL, time_created integer NOT NULL, time_updated integer NOT NULL,
		data text NOT NULL);
	CREATE TABLE session (
		id text PRIMARY KEY, project_id text NOT NULL, slug text NOT NULL, directory text NOT NULL,
		title text NOT NULL, version text NOT NULL, cost real DEFAULT 0,
		tokens_input integer DEFAULT 0, tokens_output integer DEFAULT 0,
		agent text, model text, time_created integer NOT NULL, time_updated integer NOT NULL);
	CREATE TABLE part (
		id text PRIMARY KEY, message_id text NOT NULL, session_id text NOT NULL,
		time_created integer NOT NULL, time_updated integer NOT NULL, data text NOT NULL);
	CREATE TABLE project (id text PRIMARY KEY, name text, worktree text);
	`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// healthyServer answers the ProbeMode health read so a Runner can reach the
// DB-backed code paths in tests.
func healthyServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
}

// Server accepts only the FLAT prompt shape {text: ...} (beta-18684, the live
// schema): prompt must admit the prompt and read the assistant answer back.
func TestPromptFlatShape(t *testing.T) {
	db := newRunnerDB(t)
	defer db.Close()
	var prompted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		switch {
		case r.URL.Path == "/api/session" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		case r.URL.Path == "/api/session" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "ses_1"}})
		case r.URL.Path == "/api/session/ses_1/prompt":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if _, ok := body["text"]; !ok {
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(map[string]any{"_tag": "InvalidRequestError", "message": "Missing key at [text]"})
				return
			}
			prompted = true
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "msg_1"}})
		case r.URL.Path == "/api/session/ses_1/wait":
			w.WriteHeader(204)
		case r.URL.Path == "/api/session/ses_1/model":
			w.WriteHeader(204)
		case r.URL.Path == "/api/session/ses_1/message":
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{"id": "msg_1", "type": "assistant", "finish": "done", "content": []map[string]any{{"type": "text", "text": "flat ok"}}},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	r := &Runner{BaseURL: srv.URL, DB: db}
	resp, err := r.prompt(context.Background(), "ses_1", "ask", "m", "p", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if !prompted {
		t.Fatal("flat prompt shape never reached the server")
	}
	if resp != "flat ok" {
		t.Fatalf("resp=%q", resp)
	}
}

// Server accepts only the NESTED shape {prompt:{text}} (older schema) —
// prompt must retry with it and still succeed.
func TestPromptNestedFallback(t *testing.T) {
	db := newRunnerDB(t)
	defer db.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		switch {
		case r.URL.Path == "/api/session" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		case r.URL.Path == "/api/session/ses_1/prompt":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if _, ok := body["prompt"]; !ok {
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(map[string]any{"_tag": "InvalidRequestError", "message": "Missing key at [prompt]"})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "msg_2"}})
		case r.URL.Path == "/api/session/ses_1/wait":
			w.WriteHeader(204)
		case r.URL.Path == "/api/session/ses_1/message":
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{"id": "msg_2", "type": "assistant", "finish": "done", "payload": map[string]any{"text": "nested ok"}},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	r := &Runner{BaseURL: srv.URL, DB: db}
	resp, err := r.prompt(context.Background(), "ses_1", "ask", "m", "p", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if resp != "nested ok" {
		t.Fatalf("resp=%q", resp)
	}
}

// metrics readback: tool calls nested in content[] at any index. metrics
// gates on ProbeMode, so the test fronts a healthy server.
func TestMetrics(t *testing.T) {
	db := newRunnerDB(t)
	defer db.Close()
	hl := healthyServer()
	defer hl.Close()
	_, err := db.Exec(`INSERT INTO session_v2 (id, project_id, slug, directory, title, version, cost, tokens_input, tokens_output, time_created, time_updated)
		VALUES ('ses_m', 'proj', 's', '/repo', 't', 'beta', 1.5, 1000, 100, 1, 2);
	INSERT INTO session_message (id, session_id, type, seq, time_created, time_updated, data) VALUES
		('m1', 'ses_m', 'assistant', 1, 1, 1, '{"content":[{"type":"tool","name":"read","state":{"input":{"filePath":"/repo/a.go"}}}]}'),
		('m2', 'ses_m', 'assistant', 2, 2, 2, '{"content":[{"type":"reasoning","text":"x"},{"type":"tool","name":"bash","state":{"input":{"command":"ls"}}}]}'),
		('m3', 'ses_m', 'assistant', 3, 3, 3, '{"content":[{"type":"tool","name":"read","state":{"input":{"filePath":"/repo/a.go"}}}]}');`)
	if err != nil {
		t.Fatal(err)
	}
	r := &Runner{BaseURL: hl.URL, DB: db}
	m := r.metrics(context.Background(), "ses_m")
	if m.Turns != 3 || m.Calls != 3 || m.Reads != 2 {
		t.Fatalf("metrics=%+v", m)
	}
	if m.WorstFile != 2 { // a.go read twice
		t.Fatalf("worstFile=%d", m.WorstFile)
	}
	if m.TokensIn != 1000 || m.TokensOut != 100 {
		t.Fatalf("tokens=%d/%d", m.TokensIn, m.TokensOut)
	}
}
