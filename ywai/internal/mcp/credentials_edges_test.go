package mcp

import "testing"

// Edge-case tests for the credentials package (TDD slice 0, validate phase).
//
// These tests cover three uncovered branches in the redactor path that the
// pinned 17 contract tests do not exercise:
//
//  1. EmptySecrets         — RedactMessage's early return when secrets is nil
//                            or an empty slice. Important for callers that
//                            invoke the redactor before any spec is loaded
//                            (e.g. during debug logging).
//  2. UrlWithoutPassword   — redactURL's "colon not found" early return:
//                            URLs of the form "scheme://user@host" (no
//                            password segment) must NOT be (mis)redacted.
//  3. UrlWithoutAt         — redactURL's "@ not found" early return:
//                            truncated/malformed URLs of the form
//                            "scheme://user:password" (no host) must be
//                            left untouched, never partial-redacted.
//
// All three tests assert the message is returned unchanged, which matches
// the documented contract of redactURL:
//
//	The pattern is intentionally narrow: it only fires when "://" is
//	followed by a non-empty user, a colon, a non-empty password, and
//	an "@".
//
// These tests do NOT modify or contradict the 17 pinned contract tests.

// TestRedactMessage_EmptySecrets pins the early-return path of
// RedactMessage when the caller passes no secret names. This is the path
// exercised before any spec is loaded, or when an MCP server declares no
// secret env vars at all.
func TestRedactMessage_EmptySecrets(t *testing.T) {
	cases := []struct {
		name string
		msg  string
	}{
		{"nil secrets, env-var message", "GITHUB_TOKEN=ghp_abc123"},
		{"empty slice, url message", "postgres://user:secret@host/db"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RedactMessage(c.msg, nil)
			if got != c.msg {
				t.Errorf("RedactMessage(%q, nil) = %q, want unchanged %q",
					c.msg, got, c.msg)
			}
			got = RedactMessage(c.msg, []string{})
			if got != c.msg {
				t.Errorf("RedactMessage(%q, []string{}) = %q, want unchanged %q",
					c.msg, got, c.msg)
			}
		})
	}
}

// TestRedactMessage_UrlWithoutPassword pins the narrow-pattern contract:
// a URL of the form "scheme://user@host" (userinfo, no password) has no
// ":" separator after the user, so there is no password segment to
// redact. The redactor must return the message unchanged instead of
// inventing a "***" where none belongs.
func TestRedactMessage_UrlWithoutPassword(t *testing.T) {
	in := "https://user@host/db"
	got := RedactMessage(in, []string{"DATABASE_URL"})
	if got != in {
		t.Errorf("RedactMessage(url no password) = %q, want unchanged %q", got, in)
	}
}

// TestRedactMessage_UrlWithoutAt pins the other narrow-pattern branch:
// a truncated or malformed URL of the form "scheme://user:password"
// (no "@host" suffix) has no host separator. The redactor must NOT
// emit a partial redaction like "scheme://user:***" — it must leave
// the whole message unchanged.
func TestRedactMessage_UrlWithoutAt(t *testing.T) {
	in := "https://user:password"
	got := RedactMessage(in, []string{"DATABASE_URL"})
	if got != in {
		t.Errorf("RedactMessage(url no at) = %q, want unchanged %q", got, in)
	}
}
