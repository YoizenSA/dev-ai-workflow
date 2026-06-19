package engram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPClient_Status_Connected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL)
	st, err := c.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if !st.Connected {
		t.Fatal("Expected Connected=true")
	}
}

func TestHTTPClient_Status_Unavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.Close() // closed → connection refused

	c := NewHTTPClient(srv.URL)
	_, err := c.Status(context.Background())
	if err != ErrEngramUnavailable {
		t.Fatalf("Expected ErrEngramUnavailable, got %v", err)
	}
}

func TestHTTPClient_RecentObservations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/observations/recent" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("limit") != "5" {
			t.Errorf("expected limit=5, got %q", r.URL.Query().Get("limit"))
		}
		resp := []map[string]interface{}{
			{"id": 1, "sync_id": "obs_1", "type": "save", "content": "first", "scope": "project"},
			{"id": 2, "sync_id": "obs_2", "type": "observation", "content": "second"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL)
	obs, err := c.RecentObservations(context.Background(), 5)
	if err != nil {
		t.Fatalf("RecentObservations() error: %v", err)
	}
	if len(obs) != 2 || obs[0].SyncID != "obs_1" {
		t.Fatalf("Unexpected observations: %+v", obs)
	}
}

func TestHTTPClient_RecentObservations_WrappedData(t *testing.T) {
	// engram may wrap arrays in {"data": [...]}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{{"id": 10, "sync_id": "w1", "content": "wrapped"}},
		})
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL)
	obs, err := c.RecentObservations(context.Background(), 0)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(obs) != 1 || obs[0].SyncID != "w1" {
		t.Fatalf("Unexpected: %+v", obs)
	}
}

func TestHTTPClient_Save(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/save" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id": 99, "sync_id": "obs_new", "type": "save", "content": "captured",
		})
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL)
	obs, err := c.Save(context.Background(), SaveRequest{
		Type: "save", Content: "captured", Scope: "project",
	})
	if err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if obs.SyncID != "obs_new" {
		t.Fatalf("Unexpected: %+v", obs)
	}
	if gotBody["type"] != "save" || gotBody["content"] != "captured" {
		t.Fatalf("Unexpected body: %+v", gotBody)
	}
}

func TestHTTPClient_DeleteObservation(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/observations/obs_9" && r.Method == http.MethodDelete {
			called = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL)
	if err := c.DeleteObservation(context.Background(), "obs_9"); err != nil {
		t.Fatalf("DeleteObservation() error: %v", err)
	}
	if !called {
		t.Fatal("DELETE not called")
	}
}

func TestHTTPClient_Search(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("q") != "react" {
			t.Errorf("expected q=react, got %q", r.URL.Query().Get("q"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": 50, "sync_id": "s1", "content": "react 19 stuff"},
		})
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL)
	obs, err := c.Search(context.Background(), SearchRequest{Query: "react", Limit: 10})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(obs) != 1 || obs[0].SyncID != "s1" {
		t.Fatalf("Unexpected: %+v", obs)
	}
}

func TestHTTPClient_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL)
	_, err := c.GetStats(context.Background())
	if err == nil {
		t.Fatal("Expected error for 500")
	}
}

func TestConsolidationPlan_Parse(t *testing.T) {
	raw := `{
		"updates": [{"observation_id":"o1","reason":"dup","new_content":"x","new_importance":9}],
		"deletes": [{"observation_id":"o2","reason":"obsolete"}],
		"new_summaries": [{"type":"summary","content":"topic Z","importance":8}],
		"digest": "system knows A,B"
	}`
	var plan ConsolidationPlan
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(plan.Updates) != 1 || plan.Updates[0].ObservationID != "o1" {
		t.Fatalf("Unexpected updates: %+v", plan.Updates)
	}
	if len(plan.Deletes) != 1 || plan.Deletes[0].ObservationID != "o2" {
		t.Fatalf("Unexpected deletes: %+v", plan.Deletes)
	}
	if len(plan.NewSummaries) != 1 || plan.NewSummaries[0].Type != "summary" {
		t.Fatalf("Unexpected summaries: %+v", plan.NewSummaries)
	}
	if plan.Digest != "system knows A,B" {
		t.Fatalf("Unexpected digest: %q", plan.Digest)
	}
}
