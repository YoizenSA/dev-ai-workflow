package main

import "testing"

// A TUI holds conversation state that exists nowhere else, so the classifier is
// what stands between an install and throwing away the user's session.
func TestOpenCodeProcess_IsServer(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want bool
	}{
		{"bare TUI", []string{"/home/u/.opencode/bin/opencode2"}, false},
		{"TUI with a prompt", []string{"opencode2", "run", "fix the build"}, false},
		{"server on a port", []string{"opencode2", "serve", "--port", "4097"}, true},
		{"service server", []string{"opencode2", "serve", "--service"}, true},
		{"v1 server", []string{"opencode", "serve"}, true},
		// "serve" as an argument value, not the subcommand. Reading it as a
		// server would kill the session that prompt belongs to.
		{"prompt mentioning serve", []string{"opencode2", "run", "--model", "serve"}, false},
		{"prompt whose text says serve", []string{"opencode2", "run", "make it serve traffic"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := openCodeProcess{pid: 1, args: tc.args}
			if got := p.isServer(); got != tc.want {
				t.Errorf("isServer(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestJoinPIDs(t *testing.T) {
	got := joinPIDs([]openCodeProcess{{pid: 12}, {pid: 34}})
	if got != "12, 34" {
		t.Errorf("joinPIDs = %q, want %q", got, "12, 34")
	}
}
