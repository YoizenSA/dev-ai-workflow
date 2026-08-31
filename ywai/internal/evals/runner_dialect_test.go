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

func newV2DB(t *testing.T) *sql.DB {
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

// v2-style server: probe answers /api/session, prompt accepts the FLAT shape
// {text: ...} (beta-18684), wait is 204, messages return assistant text.
func TestPromptV2FlatShape(t *testing.T) {
	db := newV2DB(t)
	defer db.Close()
	var prompted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		switch {
		case r.URL.Path == "/api/session" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		case r.URL.Path == "/api/session" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "ses_v2_1"}})
		case r.URL.Path == "/api/session/ses_v2_1/prompt":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if _, ok := body["text"]; !ok {
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(map[string]any{"_tag": "InvalidRequestError", "message": "Missing key at [text]"})
				return
			}
			prompted = true
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "msg_1"}})
		case r.URL.Path == "/api/session/ses_v2_1/wait":
			w.WriteHeader(204)
		case r.URL.Path == "/api/session/ses_v2_1/model":
			w.WriteHeader(204)
		case r.URL.Path == "/api/session/ses_v2_1/message":
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{"id": "msg_1", "type": "assistant", "finish": "done", "content": []map[string]any{{"type": "text", "text": "flat ok"}}},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	r := &Runner{BaseURL: srv.URL, DB: db}
	resp, err := r.prompt(context.Background(), "ses_v2_1", "ask", "m", "p", "hello")
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

// Same flow but the server accepts only the NESTED shape {prompt:{text}} (newer
// schema) — promptV2 must retry and still succeed.
func TestPromptV2NestedFallback(t *testing.T) {
	db := newV2DB(t)
	defer db.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		switch {
		case r.URL.Path == "/api/session" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		case r.URL.Path == "/api/session/ses_v2_1/prompt":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if _, ok := body["prompt"]; !ok {
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(map[string]any{"_tag": "InvalidRequestError", "message": "Missing key at [prompt]"})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "msg_2"}})
		case r.URL.Path == "/api/session/ses_v2_1/wait":
			w.WriteHeader(204)
		case r.URL.Path == "/api/session/ses_v2_1/message":
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{"id": "msg_2", "type": "assistant", "finish": "done", "payload": map[string]any{"text": "nested ok"}},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	r := &Runner{BaseURL: srv.URL, DB: db}
	resp, err := r.prompt(context.Background(), "ses_v2_1", "ask", "m", "p", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if resp != "nested ok" {
		t.Fatalf("resp=%q", resp)
	}
}

// metricsV2 readback: tool calls nested in content[] at any index.
func TestMetricsV2(t *testing.T) {
	db := newV2DB(t)
	defer db.Close()
	_, err := db.Exec(`INSERT INTO session_v2 (id, project_id, slug, directory, title, version, cost, tokens_input, tokens_output, time_created, time_updated)
		VALUES ('ses_m', 'proj', 's', '/repo', 't', 'beta', 1.5, 1000, 100, 1, 2);
	INSERT INTO session_message (id, session_id, type, seq, time_created, time_updated, data) VALUES
		('m1', 'ses_m', 'assistant', 1, 1, 1, '{"content":[{"type":"tool","name":"read","state":{"input":{"filePath":"/repo/a.go"}}}]}'),
		('m2', 'ses_m', 'assistant', 2, 2, 2, '{"content":[{"type":"reasoning","text":"x"},{"type":"tool","name":"bash","state":{"input":{"command":"ls"}}}]}'),
		('m3', 'ses_m', 'assistant', 3, 3, 3, '{"content":[{"type":"tool","name":"read","state":{"input":{"filePath":"/repo/a.go"}}}]}');`)
	if err != nil {
		t.Fatal(err)
	}
	r := &Runner{DB: db, Mode: ModeV2}
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
