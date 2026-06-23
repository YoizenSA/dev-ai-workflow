package mcp

import (
	"slices"
	"strings"
	"testing"
)

// These tests pin the contract of the credentials package from
// ywai/internal/mcp/credentials.go (TDD slice 0 of the "Real MCP Install"
// plan). They are RED right now: the package and the three functions
// (ValidateCreds, RedactMessage, MergeEnv) do not exist yet, so the file
// will not compile until @dev implements them.
//
// Assumptions baked into the tests (derived from the architect's plan, see
// memory #185 / session ses_10e70d5f9ffe3rAn5rBmEocB93):
//
//   - ValidateCreds returns the subset of the spec that is still missing
//     after applying `provided`. The order of the returned slice is NOT
//     pinned, so tests compare by set membership (sorted names).
//   - A provided value of "" counts as missing for required specs.
//   - RedactMessage replaces the value half of `NAME=VALUE` env-var
//     assignments (case-insensitive on the NAME side) with "***". It also
//     redacts the password segment of any URL of the shape
//     "scheme://user:secret@host". The original casing of the name in the
//     message is preserved in the output.
//   - MergeEnv takes a `base` slice in os.Environ() format and a flat
//     `provided` map. It returns the merged slice plus two counters:
//     `setCount` is the number of keys from `provided` that appear in the
//     result (overrides count), and `secretCount` is the subset of those
//     whose key is listed in `secretNames`.

// ─── helpers ──────────────────────────────────────────────────────────────

// missingNames returns the Names of a slice of EnvSpec. Useful for tests
// that only care about which specs are missing, not their full content.
func missingNames(specs []EnvSpec) []string {
	out := make([]string, 0, len(specs))
	for _, s := range specs {
		out = append(out, s.Name)
	}
	return out
}

// envToMap converts a KEY=VALUE slice (os.Environ() format) into a map for
// order-independent assertions on MergeEnv output.
func envToMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, e := range env {
		if idx := strings.IndexByte(e, '='); idx >= 0 {
			m[e[:idx]] = e[idx+1:]
		}
	}
	return m
}

// ─── ValidateCreds ────────────────────────────────────────────────────────

func TestValidateCreds_Empty(t *testing.T) {
	// Empty spec + nil provided → nothing is missing. The result may be nil
	// or a zero-length slice; both are acceptable.
	got := ValidateCreds(nil, nil)
	if len(got) != 0 {
		t.Errorf("ValidateCreds(nil, nil) = %v, want empty (len 0)", missingNames(got))
	}
}

func TestValidateCreds_AllOptional(t *testing.T) {
	spec := []EnvSpec{
		{Name: "OPTIONAL_A", Required: false},
		{Name: "OPTIONAL_B", Required: false},
		{Name: "OPTIONAL_C", Required: false},
	}
	got := ValidateCreds(spec, nil)
	if len(got) != 0 {
		t.Errorf("ValidateCreds(3 optional, nil) missing = %v, want empty",
			missingNames(got))
	}
}

func TestValidateCreds_AllPresent(t *testing.T) {
	spec := []EnvSpec{
		{Name: "TOKEN_A", Required: true},
		{Name: "TOKEN_B", Required: true},
	}
	provided := map[string]string{
		"TOKEN_A": "value-a",
		"TOKEN_B": "value-b",
	}
	got := ValidateCreds(spec, provided)
	if len(got) != 0 {
		t.Errorf("ValidateCreds(2 required, both provided) missing = %v, want empty",
			missingNames(got))
	}
}

func TestValidateCreds_MissingRequired(t *testing.T) {
	spec := []EnvSpec{
		{Name: "TOKEN_A", Required: true},
		{Name: "TOKEN_B", Required: true},
	}
	provided := map[string]string{
		"TOKEN_A": "value-a",
		// TOKEN_B deliberately absent.
	}
	got := ValidateCreds(spec, provided)
	gotNames := missingNames(got)
	slices.Sort(gotNames)
	want := []string{"TOKEN_B"}
	if !slices.Equal(gotNames, want) {
		t.Errorf("ValidateCreds(missing one) missing = %v, want %v", gotNames, want)
	}
}

func TestValidateCreds_EmptyStringValue(t *testing.T) {
	// An empty string is NOT a valid value: a required spec whose provided
	// value is "" must still be reported as missing.
	spec := []EnvSpec{
		{Name: "GITHUB_TOKEN", Required: true},
	}
	provided := map[string]string{
		"GITHUB_TOKEN": "",
	}
	got := ValidateCreds(spec, provided)
	gotNames := missingNames(got)
	want := []string{"GITHUB_TOKEN"}
	if !slices.Equal(gotNames, want) {
		t.Errorf("ValidateCreds(empty string value) missing = %v, want %v", gotNames, want)
	}
}

func TestValidateCreds_MultipleMissing(t *testing.T) {
	spec := []EnvSpec{
		{Name: "A", Required: true},
		{Name: "B", Required: true},
		{Name: "C", Required: true},
	}
	provided := map[string]string{
		"A": "1",
		// B and C deliberately absent.
	}
	got := ValidateCreds(spec, provided)
	gotNames := missingNames(got)
	slices.Sort(gotNames)
	want := []string{"B", "C"}
	if !slices.Equal(gotNames, want) {
		t.Errorf("ValidateCreds(multiple missing) missing = %v, want %v", gotNames, want)
	}
}

// ─── RedactMessage ────────────────────────────────────────────────────────

func TestRedactMessage_TokenAssignment(t *testing.T) {
	// Standard env-var assignment: the value portion (whitespace-delimited
	// token after the first "=") is replaced with "***".
	in := "GITHUB_TOKEN=ghp_abc123 def"
	got := RedactMessage(in, []string{"GITHUB_TOKEN"})
	want := "GITHUB_TOKEN=*** def"
	if got != want {
		t.Errorf("RedactMessage(%q) = %q, want %q", in, got, want)
	}
}

func TestRedactMessage_AuthHeader(t *testing.T) {
	// "Authorization: Bearer xyz" is NOT a NAME=VALUE env-var assignment, so
	// the redactor must not mistake "xyz" for the value of GITHUB_TOKEN.
	// This pins format-awareness: the redactor looks for the env-var
	// *assignment* shape, not for the value floating in arbitrary text.
	in := "Authorization: Bearer xyz"
	got := RedactMessage(in, []string{"GITHUB_TOKEN"})
	if got != in {
		t.Errorf("RedactMessage(AuthHeader) = %q, want unchanged %q", got, in)
	}
}

func TestRedactMessage_PasswordInUrl(t *testing.T) {
	// URL with an embedded password: the segment between ":" and "@" is
	// replaced with "***". This is the pattern that catches accidental
	// logging of DATABASE_URL, REDIS_URL, etc.
	in := "postgres://user:secret@host/db"
	got := RedactMessage(in, []string{"DATABASE_URL"})
	want := "postgres://user:***@host/db"
	if got != want {
		t.Errorf("RedactMessage(url password) = %q, want %q", got, want)
	}
}

func TestRedactMessage_NoSecret(t *testing.T) {
	// Message contains no env-var assignment and no URL → unchanged.
	in := "normal log line"
	got := RedactMessage(in, []string{"GITHUB_TOKEN"})
	if got != in {
		t.Errorf("RedactMessage(no secret) = %q, want unchanged %q", got, in)
	}
}

func TestRedactMessage_MultipleSecrets(t *testing.T) {
	// Two env-var assignments on the same line, each with a different name.
	// Both values must be redacted independently.
	in := "GITHUB_TOKEN=a POSTGRES_URL=b"
	got := RedactMessage(in, []string{"GITHUB_TOKEN", "POSTGRES_URL"})
	want := "GITHUB_TOKEN=*** POSTGRES_URL=***"
	if got != want {
		t.Errorf("RedactMessage(multiple) = %q, want %q", got, want)
	}
}

func TestRedactMessage_CaseInsensitive(t *testing.T) {
	// Name match is case-insensitive: "github_token" in the message must be
	// recognized as GITHUB_TOKEN. The name in the output keeps its original
	// casing (we do NOT rewrite it to uppercase).
	in := "github_token=value"
	got := RedactMessage(in, []string{"GITHUB_TOKEN"})
	want := "github_token=***"
	if got != want {
		t.Errorf("RedactMessage(case-insensitive) = %q, want %q", got, want)
	}
}

// ─── MergeEnv ─────────────────────────────────────────────────────────────

func TestMergeEnv_NilProvided(t *testing.T) {
	// Nil provided map → result equals base exactly; counters are zero.
	base := []string{"A=1", "B=2"}
	env, setCount, secretCount := MergeEnv(base, nil, nil)
	if !slices.Equal(env, base) {
		t.Errorf("MergeEnv(nil provided) env = %v, want %v", env, base)
	}
	if setCount != 0 {
		t.Errorf("setCount = %d, want 0", setCount)
	}
	if secretCount != 0 {
		t.Errorf("secretCount = %d, want 0", secretCount)
	}
}

func TestMergeEnv_OverrideBase(t *testing.T) {
	// When `provided` has a key that already exists in `base`, the provided
	// value wins. The base value must NOT appear in the result, and the
	// entry must not be duplicated.
	base := []string{"GOPATH=/usr/local"}
	provided := map[string]string{"GOPATH": "/home/user"}
	env, setCount, secretCount := MergeEnv(base, provided, nil)

	if len(env) != 1 {
		t.Fatalf("MergeEnv(override) env has %d entries, want 1: %v", len(env), env)
	}
	m := envToMap(env)
	if got := m["GOPATH"]; got != "/home/user" {
		t.Errorf("GOPATH = %q, want /home/user (override must win)", got)
	}
	if setCount != 1 {
		t.Errorf("setCount = %d, want 1 (override counts as a set)", setCount)
	}
	if secretCount != 0 {
		t.Errorf("secretCount = %d, want 0", secretCount)
	}
}

func TestMergeEnv_AppendNew(t *testing.T) {
	// New key in `provided` that is not in `base` → appended to the result.
	// All base entries must survive.
	base := []string{"A=1", "B=2"}
	provided := map[string]string{"C": "3"}
	env, setCount, secretCount := MergeEnv(base, provided, nil)

	if len(env) != 3 {
		t.Fatalf("MergeEnv(append new) env has %d entries, want 3: %v", len(env), env)
	}
	m := envToMap(env)
	wantMap := map[string]string{"A": "1", "B": "2", "C": "3"}
	if !mapsEqual(m, wantMap) {
		t.Errorf("MergeEnv(append new) env = %v, want %v", m, wantMap)
	}
	if setCount != 1 {
		t.Errorf("setCount = %d, want 1 (only C is set from provided)", setCount)
	}
	if secretCount != 0 {
		t.Errorf("secretCount = %d, want 0", secretCount)
	}
}

func TestMergeEnv_SecretCount(t *testing.T) {
	// Counters: setCount is the number of provided keys that landed in the
	// result (3), secretCount is the subset whose key is in secretNames (2).
	base := []string{"HOME=/home/user"}
	provided := map[string]string{
		"GITHUB_TOKEN": "x",    // secret
		"POSTGRES_URL": "y",    // secret
		"LOG_LEVEL":    "info", // non-secret
	}
	secretNames := []string{"GITHUB_TOKEN", "POSTGRES_URL"}
	env, setCount, secretCount := MergeEnv(base, provided, secretNames)

	if len(env) != 4 {
		t.Fatalf("MergeEnv(secret count) env has %d entries, want 4: %v",
			len(env), env)
	}
	m := envToMap(env)
	for _, k := range []string{"HOME", "GITHUB_TOKEN", "POSTGRES_URL", "LOG_LEVEL"} {
		if _, ok := m[k]; !ok {
			t.Errorf("env missing key %q: %v", k, m)
		}
	}
	if setCount != 3 {
		t.Errorf("setCount = %d, want 3", setCount)
	}
	if secretCount != 2 {
		t.Errorf("secretCount = %d, want 2", secretCount)
	}
}

func TestMergeEnv_EmptyProvided(t *testing.T) {
	// Non-nil but empty `provided` → same as nil: base returned unchanged,
	// counters zero.
	base := []string{"A=1", "B=2"}
	provided := map[string]string{}
	env, setCount, secretCount := MergeEnv(base, provided, nil)
	if !slices.Equal(env, base) {
		t.Errorf("MergeEnv(empty provided) env = %v, want %v", env, base)
	}
	if setCount != 0 {
		t.Errorf("setCount = %d, want 0", setCount)
	}
	if secretCount != 0 {
		t.Errorf("secretCount = %d, want 0", secretCount)
	}
}

// mapsEqual is a small helper to compare two string maps. We don't pull in
// reflect.DeepEqual for one call site.
func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
