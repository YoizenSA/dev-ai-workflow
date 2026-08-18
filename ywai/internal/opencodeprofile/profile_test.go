package opencodeprofile

import (
	"os"
	"path/filepath"
	"testing"

	agentprofiles "github.com/Yoizen/dev-ai-workflow/ywai/internal/agents"
)

func TestParseName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"dev", NameDev, false},
		{"DEV", NameDev, false},
		{"qa", NameQA, false},
		{"infra", NameInfra, false},
		{"work", "", true},
		{"", "", true},
	}
	for _, tc := range cases {
		got, err := ParseName(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("ParseName(%q) want error", tc.in)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Fatalf("ParseName(%q) = %q, %v want %q", tc.in, got, err, tc.want)
		}
	}
}

func TestDirsFor(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	d, err := DirsFor(home, NameQA)
	if err != nil {
		t.Fatal(err)
	}
	if d.Config != filepath.Join(home, ".ywai", "opencode-profiles", "qa", "config") {
		t.Fatalf("config = %s", d.Config)
	}
	if d.Data != filepath.Join(home, ".ywai", "opencode-profiles", "qa", "data") {
		t.Fatalf("data = %s", d.Data)
	}
}

func TestLaunchEnv(t *testing.T) {
	t.Parallel()
	d := Dirs{Config: "/tmp/cfg", Data: "/tmp/data"}
	env := LaunchEnv(d, NameDev)
	want := map[string]string{
		"OPENCODE_CONFIG_DIR": "/tmp/cfg",
		"XDG_DATA_HOME":       "/tmp/data",
		"OPENCODE_PROFILE":    "dev",
	}
	got := map[string]string{}
	for _, e := range env {
		k, v, ok := splitKV(e)
		if !ok {
			t.Fatalf("bad env %q", e)
		}
		got[k] = v
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("env %s = %q want %q", k, got[k], v)
		}
	}
}

func TestKeepAgent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		profile, agent, group string
		keep                  bool
	}{
		{NameDev, "orchestrator", "core", true},
		{NameDev, "dev", "core", true},
		{NameDev, "devops", "core", false},
		{NameDev, "qa-orchestrator", "qa-automation", false},
		{NameQA, "qa-orchestrator", "qa-automation", true},
		{NameQA, "orchestrator", "core", true},
		{NameQA, "dev", "core", false},
		{NameInfra, "devops", "core", true},
		{NameInfra, "infra-docs", "experiment", true},
		{NameInfra, "orchestrator", "core", true},
		{NameInfra, "dev", "core", false},
	}
	for _, tc := range cases {
		got := KeepAgent(tc.profile, tc.agent, tc.group)
		if got != tc.keep {
			t.Fatalf("KeepAgent(%s,%s,%s)=%v want %v", tc.profile, tc.agent, tc.group, got, tc.keep)
		}
	}
}

func TestFilterProfiles(t *testing.T) {
	t.Parallel()
	all := map[string]agentprofiles.AgentProfile{
		"orchestrator":    {Name: "orchestrator", Group: "core"},
		"dev":             {Name: "dev", Group: "core"},
		"devops":          {Name: "devops", Group: "core"},
		"qa-orchestrator": {Name: "qa-orchestrator", Group: "qa-automation"},
		"infra-docs":      {Name: "infra-docs", Group: "experiment"},
	}
	got := FilterProfiles(all, NameInfra)
	if _, ok := got["devops"]; !ok {
		t.Fatal("infra missing devops")
	}
	if _, ok := got["dev"]; ok {
		t.Fatal("infra should not keep dev")
	}
	if _, ok := got["infra-docs"]; !ok {
		t.Fatal("infra missing experiment agent")
	}
}

func TestCopySharedConfig(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	dst := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "opencode.json"), []byte(`{"plugin":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "AGENTS.md"), []byte("# agents"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CopySharedConfig(src, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "opencode.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
}

func splitKV(s string) (string, string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return s[:i], s[i+1:], true
		}
	}
	return "", "", false
}
