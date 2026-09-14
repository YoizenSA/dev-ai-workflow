package main

import (
	"os"
	"path/filepath"
	"testing"
)

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

// A pid file holding -1 once failed `ywai env init` for one environment:
// readStopPIDFile accepted it, killPIDInt signaled the process group and
// the restart warning turned the whole install into exit 1.
func TestReadStopPIDFile_RejectsNonPositive(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name    string
		content string
		wantPID int
		wantErr bool
	}{
		{"valid pid", "12345\n", 12345, false},
		{"minus one", "-1\n", 0, true},
		{"zero", "0\n", 0, true},
		{"garbage", "not-a-pid\n", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, "service.pid")
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := readStopPIDFile(path)
			if tc.wantErr && err == nil {
				t.Errorf("readStopPIDFile(%q) = %d, nil error; want error", tc.content, got)
			}
			if !tc.wantErr && (err != nil || got != tc.wantPID) {
				t.Errorf("readStopPIDFile(%q) = (%d, %v); want (%d, nil)", tc.content, got, err, tc.wantPID)
			}
		})
	}
	if _, err := readStopPIDFile(filepath.Join(dir, "missing.pid")); err == nil {
		t.Error("readStopPIDFile(missing) = nil error; want error")
	}
}

func TestKillPIDInt_RejectsNonPositive(t *testing.T) {
	for _, pid := range []int{-1, 0} {
		if err := killPIDInt(pid); err == nil {
			t.Errorf("killPIDInt(%d) = nil; want error (must never signal a group)", pid)
		}
	}
}
