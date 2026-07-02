// Package mcp provides shared helpers for the ywai MCP install flow:
// validating required credentials, redacting secrets from log/UI messages,
// merging user-provided env vars on top of an os.Environ() base,
// and managing OAuth tokens for remote MCP servers.
package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// OAuthToken holds the token response from an OAuth authorization server.
type OAuthToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
}

// IsExpired returns true when the token has no expiry or is past its expiry.
func (t *OAuthToken) IsExpired() bool {
	return t.ExpiresAt.IsZero() || time.Now().After(t.ExpiresAt)
}

// tokenFile returns the path to the OAuth token store for the given server ID.
// Tokens are stored under ~/.ywai/mcp-tokens/ with 0600 perms.
func tokenDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home dir: %w", err)
	}
	dir := filepath.Join(home, ".ywai", "mcp-tokens")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("cannot create token dir %s: %w", dir, err)
	}
	return dir, nil
}

func tokenPath(serverID string) (string, error) {
	dir, err := tokenDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, serverID+".json"), nil
}

// LoadToken reads the OAuth token for the given server from disk.
// Returns nil when no token exists (not an error).
func LoadToken(serverID string) (*OAuthToken, error) {
	p, err := tokenPath(serverID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read token for %s: %w", serverID, err)
	}
	var tok OAuthToken
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, fmt.Errorf("cannot parse token for %s: %w", serverID, err)
	}
	return &tok, nil
}

// SaveToken persists the OAuth token for the given server to disk
// with file permissions 0600.
func SaveToken(serverID string, tok *OAuthToken) error {
	p, err := tokenPath(serverID)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal token: %w", err)
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return fmt.Errorf("cannot write token to %s: %w", p, err)
	}
	return nil
}

// TokenStore is a simple in-memory cache of OAuth tokens, keyed by server ID.
// Thread-safe for concurrent access.
var (
	tokenMu    sync.RWMutex
	tokenCache map[string]*OAuthToken
)

func init() {
	tokenCache = make(map[string]*OAuthToken)
}

// CachedLoadToken reads a token from the in-memory cache (fast path) or
// falls back to disk.
func CachedLoadToken(serverID string) (*OAuthToken, error) {
	tokenMu.RLock()
	t, ok := tokenCache[serverID]
	tokenMu.RUnlock()
	if ok {
		return t, nil
	}
	t, err := LoadToken(serverID)
	if err != nil {
		return nil, err
	}
	if t != nil {
		tokenMu.Lock()
		tokenCache[serverID] = t
		tokenMu.Unlock()
	}
	return t, nil
}

// CachedSaveToken persists a token to disk and updates the in-memory cache.
func CachedSaveToken(serverID string, tok *OAuthToken) error {
	if err := SaveToken(serverID, tok); err != nil {
		return err
	}
	tokenMu.Lock()
	tokenCache[serverID] = tok
	tokenMu.Unlock()
	return nil
}

// HasOAuth returns true when the catalog entry requires OAuth.
func (e CatalogEntry) HasOAuth() bool {
	return e.AuthType == "oauth"
}

// EnvSpec describes a single environment variable expected by an MCP server
// install. Required specs must be present with a non-empty value in the final
// merged environment; optional specs may be omitted.
type EnvSpec struct {
	Name        string
	Description string
	Required    bool
	Secret      bool
}

// ValidateCreds returns the subset of `spec` entries that are required but
// missing (or empty) in `provided`. The result is nil when every required
// spec is satisfied. The order of the returned slice is not guaranteed;
// callers should compare by set membership.
func ValidateCreds(spec []EnvSpec, provided map[string]string) []EnvSpec {
	var missing []EnvSpec
	for _, s := range spec {
		if !s.Required {
			continue
		}
		if v, ok := provided[s.Name]; ok && v != "" {
			continue
		}
		missing = append(missing, s)
	}
	return missing
}

// RedactMessage returns msg with the value portion of any env-var assignment
// whose name appears in `secrets` (case-insensitive) replaced by "***". It
// also redacts the password segment of URLs of the shape
// "scheme://user:password@host" so that credentials embedded in DATABASE_URL,
// REDIS_URL, etc. are not leaked. The original casing of names in the message
// is preserved.
func RedactMessage(msg string, secrets []string) string {
	if len(secrets) == 0 {
		return msg
	}
	if msg = redactURL(msg); msg != "" {
		msg = redactAssignments(msg, secrets)
	}
	return msg
}

// redactURL rewrites the password segment of the first URL of the shape
// "scheme://user:password@host" found in msg. The pattern is intentionally
// narrow: it only fires when "://" is followed by a non-empty user, a colon,
// a non-empty password, and an "@".
func redactURL(msg string) string {
	i := strings.Index(msg, "://")
	if i < 0 {
		return msg
	}
	start := i + 3
	colon := strings.IndexByte(msg[start:], ':')
	if colon < 0 {
		return msg
	}
	colon += start
	at := strings.IndexByte(msg[colon+1:], '@')
	if at < 0 {
		return msg
	}
	at += colon + 1
	return msg[:colon+1] + "***" + msg[at:]
}

// redactAssignments splits msg on whitespace, rewrites the value of any
// "NAME=VALUE" token whose NAME matches one in `secrets` (case-insensitive),
// and rejoins with single spaces.
func redactAssignments(msg string, secrets []string) string {
	secretSet := make(map[string]struct{}, len(secrets))
	for _, s := range secrets {
		secretSet[strings.ToUpper(s)] = struct{}{}
	}
	words := strings.Fields(msg)
	for i, w := range words {
		eq := strings.IndexByte(w, '=')
		if eq < 0 {
			continue
		}
		if _, ok := secretSet[strings.ToUpper(w[:eq])]; ok {
			words[i] = w[:eq+1] + "***"
		}
	}
	return strings.Join(words, " ")
}

// MergeEnv merges a slice of "KEY=VALUE" entries (typically os.Environ())
// with a flat map of overrides. Keys present in `provided` replace existing
// base entries; new keys are appended. It returns the merged slice, the
// number of provided keys that landed in the result (`setCount`, overrides
// included), and the subset of those whose key is listed in `secretNames`
// (`secretCount`).
func MergeEnv(base []string, provided map[string]string, secretNames []string) (env []string, setCount int, secretCount int) {
	if len(provided) == 0 {
		return base, 0, 0
	}
	secretSet := make(map[string]struct{}, len(secretNames))
	for _, n := range secretNames {
		secretSet[n] = struct{}{}
	}

	env = make([]string, len(base))
	copy(env, base)
	keyIndex := make(map[string]int, len(base))
	for i, e := range env {
		if eq := strings.IndexByte(e, '='); eq >= 0 {
			keyIndex[e[:eq]] = i
		}
	}

	for k, v := range provided {
		entry := k + "=" + v
		if idx, ok := keyIndex[k]; ok {
			env[idx] = entry
		} else {
			env = append(env, entry)
		}
		setCount++
		if _, isSecret := secretSet[k]; isSecret {
			secretCount++
		}
	}
	return env, setCount, secretCount
}
