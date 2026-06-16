package web

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/engram"
)

// MemoryEvalRequest configures one recall-quality run.
type MemoryEvalRequest struct {
	SampleSize int    `json:"sample_size,omitempty"` // how many prompts to evaluate (default 100, max 500)
	K          int    `json:"k,omitempty"`           // top-k cutoff for hit/precision (default 10)
	Project    string `json:"project,omitempty"`     // optional filter
	MinLen     int    `json:"min_len,omitempty"`     // skip prompts shorter than this (default 20)
}

// MemoryEvalMiss is one prompt for which retrieval failed.
type MemoryEvalMiss struct {
	PromptID  int    `json:"prompt_id"`
	Content   string `json:"content"`
	SessionID string `json:"session_id"`
	Project   string `json:"project,omitempty"`
	TopResult string `json:"top_result,omitempty"` // title of the top-1 search hit, for inspection
}

// MemoryEvalSample is one prompt + its measurement.
type MemoryEvalSample struct {
	PromptID  int     `json:"prompt_id"`
	SessionID string  `json:"session_id"`
	Hit       bool    `json:"hit"`        // any result with prompt.session_id in top-k
	HitRank   int     `json:"hit_rank"`   // 1-based rank of first hit, 0 if no hit
	Precision float64 `json:"precision"`  // hits / k
	Snippet   string  `json:"snippet"`    // first 80 chars of prompt
}

// MemoryEvalResult is the response of a recall-quality run.
type MemoryEvalResult struct {
	StartedAt          time.Time          `json:"started_at"`
	DurationMS         int64              `json:"duration_ms"`
	K                  int                `json:"k"`
	Project            string             `json:"project,omitempty"`
	TotalPrompts       int                `json:"total_prompts"`        // total prompts inspected
	Evaluable          int                `json:"evaluable"`            // prompts whose session has at least one observation
	Evaluated          int                `json:"evaluated"`            // = Evaluable (post-filter)
	Skipped            int                `json:"skipped"`              // too short, no session_id, no obs for session, etc.
	HitRate            float64            `json:"hit_rate"`             // any same-session hit in top-k
	PrecisionAt        float64            `json:"precision_at_k"`       // mean precision@k (strict — same session_id)
	MRR                float64            `json:"mrr"`                  // mean reciprocal rank of first strict hit
	ProjectHitRate     float64            `json:"project_hit_rate"`     // any same-project hit in top-k (loose)
	ProjectPrecisionAt float64            `json:"project_precision_at_k"`
	Samples            []MemoryEvalSample `json:"samples"`
	Misses             []MemoryEvalMiss   `json:"misses"`
}

// RunMemoryEval → POST /api/engram/memory-evals
func (h *Handlers) RunMemoryEval(w http.ResponseWriter, r *http.Request) {
	c, ok := h.requireEngram(w)
	if !ok {
		return
	}
	var req MemoryEvalRequest
	if err := decodeJSONBody(r, &req); err != nil && r.ContentLength > 0 {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	// Defaults + clamps.
	if req.SampleSize <= 0 {
		req.SampleSize = 100
	}
	if req.SampleSize > 500 {
		req.SampleSize = 500
	}
	if req.K <= 0 {
		req.K = 10
	}
	if req.K > 50 {
		req.K = 50
	}
	if req.MinLen <= 0 {
		req.MinLen = 20
	}

	start := time.Now()
	result, err := runMemoryEval(r.Context(), c, req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	result.DurationMS = time.Since(start).Milliseconds()
	writeJSON(w, http.StatusOK, result)
}

// runMemoryEval is the pure-logic helper, exported via the handler.
func runMemoryEval(
	ctx context.Context,
	c engram.Client,
	req MemoryEvalRequest,
) (MemoryEvalResult, error) {
	result := MemoryEvalResult{
		StartedAt: time.Now(),
		K:         req.K,
		Project:   req.Project,
		Samples:   []MemoryEvalSample{},
		Misses:    []MemoryEvalMiss{},
	}

	// Pre-fetch the universe of observations so we can:
	//  1. Filter prompts to only those whose session has at least one
	//     observation (otherwise "same session_id" is unsatisfiable and the
	//     metric is meaningless).
	//  2. Validate the project filter.
	allObs, err := c.RecentObservations(ctx, 1000)
	if err != nil {
		return result, err
	}
	obsBySession := make(map[string]bool, len(allObs))
	for _, o := range allObs {
		if o.SessionID != "" {
			obsBySession[o.SessionID] = true
		}
	}

	// Pull a generous batch of prompts and filter client-side. Engram's
	// /prompts/recent returns by created_at desc.
	prompts, err := c.RecentPrompts(ctx, req.SampleSize*5)
	if err != nil {
		return result, err
	}
	result.TotalPrompts = len(prompts)

	var totalPrecision, totalMRR float64
	var totalProjectPrecision float64
	var projectHits int
	for _, p := range prompts {
		if result.Evaluated >= req.SampleSize {
			break
		}
		content := strings.TrimSpace(p.Content)
		if len(content) < req.MinLen {
			result.Skipped++
			continue
		}
		if p.SessionID == "" {
			result.Skipped++
			continue
		}
		if req.Project != "" && p.Project != req.Project {
			result.Skipped++
			continue
		}
		// Strict relevance requires at least one observation in this prompt's
		// session — otherwise no top-k could ever hit. Drop unsatisfiable
		// prompts from the sample.
		if !obsBySession[p.SessionID] {
			result.Skipped++
			continue
		}
		result.Evaluable++

		// Engram's search is space-separated AND with no wildcards/operators,
		// so even one off-topic verb in the prompt zeroes the match. We try
		// progressively shorter queries (top-3 → top-2 → top-1 longest content
		// words) and use the first non-empty result set.
		queries := buildProgressiveQueries(content)
		if len(queries) == 0 {
			result.Skipped++
			result.Evaluable--
			continue
		}
		var hits []engram.Observation
		var lastErr error
		for _, q := range queries {
			hits, lastErr = c.Search(ctx, engram.SearchRequest{Query: q, Limit: req.K})
			if lastErr != nil || len(hits) > 0 {
				break
			}
		}
		err = lastErr
		if err != nil {
			// One failed search shouldn't tank the run — treat as a miss.
			result.Misses = append(result.Misses, MemoryEvalMiss{
				PromptID:  p.ID,
				Content:   snippet(content, 200),
				SessionID: p.SessionID,
				Project:   p.Project,
				TopResult: "(search error: " + err.Error() + ")",
			})
			result.Evaluated++
			continue
		}

		var matches int
		var projectMatches int
		firstRank := 0
		for i, h := range hits {
			if h.SessionID == p.SessionID {
				matches++
				if firstRank == 0 {
					firstRank = i + 1
				}
			}
			if p.Project != "" && h.Project == p.Project {
				projectMatches++
			}
		}
		precision := float64(matches) / float64(req.K)
		mrr := 0.0
		if firstRank > 0 {
			mrr = 1.0 / float64(firstRank)
		}
		totalPrecision += precision
		totalMRR += mrr
		totalProjectPrecision += float64(projectMatches) / float64(req.K)
		if projectMatches > 0 {
			projectHits++
		}

		result.Samples = append(result.Samples, MemoryEvalSample{
			PromptID:  p.ID,
			SessionID: p.SessionID,
			Hit:       matches > 0,
			HitRank:   firstRank,
			Precision: precision,
			Snippet:   snippet(content, 80),
		})

		if matches == 0 {
			topTitle := ""
			if len(hits) > 0 {
				topTitle = hits[0].Title
				if topTitle == "" {
					topTitle = snippet(hits[0].Content, 60)
				}
			}
			result.Misses = append(result.Misses, MemoryEvalMiss{
				PromptID:  p.ID,
				Content:   snippet(content, 200),
				SessionID: p.SessionID,
				Project:   p.Project,
				TopResult: topTitle,
			})
		}
		result.Evaluated++
	}

	if result.Evaluated > 0 {
		n := float64(result.Evaluated)
		result.PrecisionAt = totalPrecision / n
		result.MRR = totalMRR / n
		result.ProjectPrecisionAt = totalProjectPrecision / n
		result.ProjectHitRate = float64(projectHits) / n

		var hits int
		for _, s := range result.Samples {
			if s.Hit {
				hits++
			}
		}
		result.HitRate = float64(hits) / n
	}

	// Sort misses by content length descending — longer prompts are more
	// surprising to miss.
	sort.SliceStable(result.Misses, func(i, j int) bool {
		return len(result.Misses[i].Content) > len(result.Misses[j].Content)
	})

	return result, nil
}

func snippet(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// stopWords are skipped when building search queries. Tiny multilingual set —
// covers the most common noise terms in user prompts. We don't need a real
// stemmer because FTS5 already does prefix matching when asked.
var stopWords = map[string]bool{
	"the": true, "and": true, "or": true, "but": true, "for": true, "with": true,
	"this": true, "that": true, "from": true, "are": true, "you": true,
	"can": true, "could": true, "would": true, "should": true, "what": true,
	"how": true, "why": true, "when": true, "where": true, "que": true,
	"como": true, "para": true, "los": true, "las": true, "una": true,
	"con": true, "del": true, "por": true, "este": true, "esta": true,
	"sin": true, "sobre": true, "entre": true, "todo": true, "todos": true,
	"todas": true, "esto": true, "eso": true, "muy": true, "mas": true,
	"más": true, "han": true, "hay": true,
}

// buildProgressiveQueries returns candidate queries from a raw prompt,
// narrowest first. Engram's search ANDs terms with no operators, so a single
// user verb (e.g. "review", "fix", "explain") that doesn't appear in any
// observation zeros the match. We try:
//   1. The top-3 longest tokens joined — narrow but maximally specific.
//   2. Each of the top-3 tokens alone — broadens to single-word hits.
// First non-empty hit list wins.
func buildProgressiveQueries(text string) []string {
	var tokens []string
	for _, w := range strings.FieldsFunc(strings.ToLower(text), isWordBreak) {
		w = strings.TrimSpace(w)
		if len(w) < 4 || stopWords[w] {
			continue
		}
		tokens = append(tokens, w)
	}
	sort.SliceStable(tokens, func(i, j int) bool {
		return len(tokens[i]) > len(tokens[j])
	})
	if len(tokens) == 0 {
		return nil
	}
	top := tokens
	if len(top) > 3 {
		top = top[:3]
	}
	queries := []string{strings.Join(top, " ")}
	for _, t := range top {
		queries = append(queries, t)
	}
	return queries
}

// buildSearchQuery is kept for tests; returns the narrowest candidate.
func buildSearchQuery(text string) string {
	qs := buildProgressiveQueries(text)
	if len(qs) == 0 {
		return ""
	}
	return qs[0]
}

func isWordBreak(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r',
		'.', ',', ';', ':', '!', '?',
		'(', ')', '[', ']', '{', '}',
		'"', '\'', '`', '/', '\\', '|',
		'<', '>', '=', '+', '-', '_',
		'@', '#', '$', '%', '&', '*', '~':
		return true
	}
	return false
}
