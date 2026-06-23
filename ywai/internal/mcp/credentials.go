// Package mcp provides shared helpers for the ywai MCP install flow:
// validating required credentials, redacting secrets from log/UI messages,
// and merging user-provided env vars on top of an os.Environ() base.
package mcp

import "strings"

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
