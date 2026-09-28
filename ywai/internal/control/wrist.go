package control

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
)

const defaultWristURL = "http://127.0.0.1:5768"

// wristGate is the opt-in for watch glances. Missing file means off.
type wristGate struct {
	mu      sync.RWMutex
	path    string
	enabled bool
	url     string
}

func loadWristGate(path string) *wristGate {
	g := &wristGate{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		return g
	}
	var doc struct {
		Enabled bool   `json:"enabled"`
		URL     string `json:"url"`
	}
	if json.Unmarshal(data, &doc) == nil {
		g.enabled = doc.Enabled
		if normalized, err := normalizeWristURL(doc.URL); err == nil {
			g.url = normalized
		}
	}
	return g
}

func (g *wristGate) URL() string {
	if g == nil {
		return defaultWristURL
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.url == "" {
		return defaultWristURL
	}
	return g.url
}

func (g *wristGate) Enabled() bool {
	if g == nil {
		return false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.enabled
}

func (g *wristGate) Set(on bool, rawURL string) error {
	if g == nil {
		return fmt.Errorf("wrist switch unavailable")
	}
	normalized, err := normalizeWristURL(rawURL)
	if err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.enabled = on
	g.url = normalized
	if g.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(g.path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(map[string]any{"enabled": on, "url": normalized})
	if err != nil {
		return err
	}
	return os.WriteFile(g.path, append(data, '\n'), 0o644)
}

func normalizeWristURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultWristURL, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("wrist url must be http(s) with a host")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

// WristNotice is the one-line glance a phone (and the watch mirroring it) can show.
// Kinds are done, failed, and waiting. Anything else is noise and is rejected.
type WristNotice struct {
	Kind      string `json:"kind"`
	Agent     string `json:"agent"`
	Label     string `json:"label"`
	Detail    string `json:"detail,omitempty"`
	Remaining int    `json:"remaining,omitempty"`
}

// Glance turns a notice into the title and body the push payload carries.
func Glance(n WristNotice) (title, body string, err error) {
	agent := wristLine(n.Agent, 40)
	if agent == "" {
		return "", "", fmt.Errorf("missing agent")
	}
	switch n.Kind {
	case "done":
		title = "Agent done"
	case "failed":
		title = "Agent failed"
	case "waiting":
		title = "Agent waiting"
	default:
		return "", "", fmt.Errorf("unknown kind %q", n.Kind)
	}
	body = agent
	if label := wristLine(n.Label, 120); label != "" {
		body += " · " + label
	}
	if n.Remaining > 0 {
		body += fmt.Sprintf(" · %d still running", n.Remaining)
	}
	if detail := wristLine(n.Detail, 220); detail != "" {
		body += "\n" + detail
	}
	return title, trimRunes(body, 360), nil
}

func wristLine(value string, max int) string {
	fields := strings.Fields(value)
	return trimRunes(strings.Join(fields, " "), max)
}

func trimRunes(value string, max int) string {
	if utf8.RuneCountInString(value) <= max {
		return value
	}
	runes := []rune(value)
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}

func requestFromLoopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
