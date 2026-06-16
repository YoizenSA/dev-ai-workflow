package web

import (
	"context"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/engram"
)

// stubEngram returns canned prompts + search results so we can assert the
// metric math without hitting the real engram server.
type stubEngram struct {
	fakeEngramClient
	prompts []engram.Prompt
	hits    map[string][]engram.Observation // query -> results
}

func (s *stubEngram) RecentPrompts(ctx context.Context, limit int) ([]engram.Prompt, error) {
	return s.prompts, nil
}

func (s *stubEngram) Search(ctx context.Context, req engram.SearchRequest) ([]engram.Observation, error) {
	return s.hits[req.Query], nil
}

func (s *stubEngram) RecentObservations(ctx context.Context, limit int) ([]engram.Observation, error) {
	// Synthetic obs: one per relevant session so every test prompt is evaluable.
	return []engram.Observation{
		{ID: 1, SessionID: "S1"},
		{ID: 2, SessionID: "S2"},
		{ID: 3, SessionID: "S3"},
	}, nil
}

func TestRunMemoryEval_BasicMetrics(t *testing.T) {
	long := func(n int) string {
		s := ""
		for i := 0; i < n; i++ {
			s += "a"
		}
		return s
	}
	// 3 prompts:
	//   P1 — top-1 hit in same session                → precision = 1/5, MRR = 1.0
	//   P2 — hit at rank 3 in same session            → precision = 1/5, MRR = 1/3
	//   P3 — no result with the same session_id       → miss, precision = 0
	// One short prompt is skipped.
	s := &stubEngram{
		prompts: []engram.Prompt{
			{ID: 1, SessionID: "S1", Content: long(50)},
			{ID: 2, SessionID: "S2", Content: long(50)},
			{ID: 3, SessionID: "S3", Content: long(50)},
			{ID: 4, SessionID: "S4", Content: "too short"},
		},
		hits: map[string][]engram.Observation{
			long(50): nil, // each search uses content as query (truncated identical here)
		},
	}
	// Override per-prompt: since content is identical for these synthetic
	// prompts, give the same hits to all. We can't tell them apart by query
	// in this stub; instead we override the stub to return per-call results.
	calls := 0
	s.hits = nil
	// Replace Search with a state-machine.
	orig := s
	wrapped := &searchWrapper{stubEngram: orig, calls: &calls}
	req := MemoryEvalRequest{SampleSize: 10, K: 5, MinLen: 20}
	res, err := runMemoryEval(context.Background(), wrapped, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Evaluated != 3 {
		t.Fatalf("evaluated=%d, want 3", res.Evaluated)
	}
	if res.Skipped != 1 {
		t.Fatalf("skipped=%d, want 1", res.Skipped)
	}
	// 2 hits out of 3 → hit_rate = 0.666…
	if res.HitRate < 0.66 || res.HitRate > 0.67 {
		t.Fatalf("hit_rate=%v, want ~0.666", res.HitRate)
	}
	// precision: (1/5 + 1/5 + 0) / 3 = 0.1333…
	if res.PrecisionAt < 0.13 || res.PrecisionAt > 0.14 {
		t.Fatalf("precision_at_k=%v, want ~0.133", res.PrecisionAt)
	}
	// MRR: (1 + 1/3 + 0) / 3 = 0.444…
	if res.MRR < 0.44 || res.MRR > 0.45 {
		t.Fatalf("mrr=%v, want ~0.444", res.MRR)
	}
	if len(res.Misses) != 1 {
		t.Fatalf("misses=%d, want 1", len(res.Misses))
	}
}

// searchWrapper returns deterministic per-call results so we can simulate
// rank-1 hit, rank-3 hit, then a miss.
type searchWrapper struct {
	*stubEngram
	calls *int
}

func (s *searchWrapper) Search(ctx context.Context, req engram.SearchRequest) ([]engram.Observation, error) {
	*s.calls++
	switch *s.calls {
	case 1: // prompt P1 → top-1 same session
		return []engram.Observation{
			{ID: 100, SessionID: "S1"},
			{ID: 101, SessionID: "OTHER"},
			{ID: 102, SessionID: "OTHER"},
			{ID: 103, SessionID: "OTHER"},
			{ID: 104, SessionID: "OTHER"},
		}, nil
	case 2: // prompt P2 → rank-3 same session
		return []engram.Observation{
			{ID: 200, SessionID: "OTHER"},
			{ID: 201, SessionID: "OTHER"},
			{ID: 202, SessionID: "S2"},
			{ID: 203, SessionID: "OTHER"},
			{ID: 204, SessionID: "OTHER"},
		}, nil
	case 3: // prompt P3 → no hit
		return []engram.Observation{
			{ID: 300, SessionID: "OTHER", Title: "Unrelated"},
		}, nil
	}
	return nil, nil
}
