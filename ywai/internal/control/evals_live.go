package control

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
)

// Live attempt tracking: while a benchmark attempt is in flight, the UI can
// tail the opencode session the agent is working in (same session the runner
// created), so "what is the agent doing right now" is answerable without
// opening the TUI.

type benchLiveAttempt struct {
	RunID     string `json:"runId"`
	Model     string `json:"model"`
	Round     int    `json:"round"`
	SessionID string `json:"sessionId"`
	BaseURL   string `json:"-"`
}

var (
	benchLiveMu      sync.Mutex
	benchLiveCurrent *benchLiveAttempt
)

func setBenchLive(a *benchLiveAttempt) {
	benchLiveMu.Lock()
	defer benchLiveMu.Unlock()
	benchLiveCurrent = a
}

func getBenchLive(runID string) *benchLiveAttempt {
	benchLiveMu.Lock()
	defer benchLiveMu.Unlock()
	if benchLiveCurrent == nil || benchLiveCurrent.RunID != runID {
		return nil
	}
	return benchLiveCurrent
}

// LiveEvent is one rendered line of the agent's live transcript.
type LiveEvent struct {
	Kind string `json:"kind"` // user | text | tool | reasoning
	Text string `json:"text"`
	At   int64  `json:"at"` // unix millis
}

// handleEvalRunLive tails what the agent of an in-flight benchmark attempt is
// doing: it proxies the opencode session's messages, trimmed to a readable
// tail. Inactive or unknown runs return {active:false} so the UI can hide the
// panel instead of erroring.
func (s *Server) handleEvalRunLive(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	benchLiveMu.Lock()
	live := benchLiveCurrent
	benchLiveMu.Unlock()
	if live == nil || live.RunID != id {
		writeJSON(w, http.StatusOK, map[string]any{"active": false})
		return
	}

	events, err := tailSessionEvents(r.Context(), live.BaseURL, live.SessionID, 40)
	body := map[string]any{
		"active":    true,
		"runId":     live.RunID,
		"model":     live.Model,
		"round":     live.Round,
		"sessionId": live.SessionID,
	}
	if err != nil {
		body["error"] = err.Error()
	} else {
		body["events"] = events
	}
	writeJSON(w, http.StatusOK, body)
}

// tailSessionEvents reads the session's messages from the opencode server and
// renders the last limit transcript lines in chronological order.
func tailSessionEvents(ctx context.Context, baseURL, sessionID string, limit int) ([]LiveEvent, error) {
	base := strings.TrimRight(baseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/session/"+sessionID+"/message", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	opencode.ApplyServerAuth(req)
	cl := &http.Client{Timeout: 10 * time.Second}
	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opencode server returned %d", resp.StatusCode)
	}

	var parsed struct {
		Data []struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Time     struct {
				Created int64 `json:"created"`
			} `json:"time"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
				Name string `json:"name"`
			} `json:"content"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}

	var events []LiveEvent
	for _, m := range parsed.Data {
		at := m.Time.Created
		switch m.Type {
		case "user":
			events = append(events, LiveEvent{Kind: "user", Text: oneLine(m.Text), At: at})
		case "assistant":
			for _, part := range m.Content {
				switch part.Type {
				case "text":
					if t := oneLine(part.Text); t != "" {
						events = append(events, LiveEvent{Kind: "text", Text: t, At: at})
					}
				case "tool":
					events = append(events, LiveEvent{Kind: "tool", Text: oneLine(part.Name), At: at})
				default: // reasoning and anything else: too noisy for a tail
				}
			}
		}
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].At < events[j].At })
	if len(events) > limit {
		events = events[len(events)-limit:]
	}
	return events, nil
}

// oneLine collapses a text blob into a single bounded line for tail display.
func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) > 220 {
		s = s[:220] + "…"
	}
	return s
}
