package envprofile

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

func writeAdoJSON(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCopyAdoConfigMergesWithoutClobbering(t *testing.T) {
	testRoot(t)
	xdgCfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgCfg)
	t.Setenv("YWAI_PROFILE", "")
	t.Setenv("OPENCODE_CONFIG_DIR", "")

	globalAdo := filepath.Join(config.OpenCodeUserConfigDir(), "ado.json")
	writeAdoJSON(t, globalAdo,
		`{"defaultProfile":"yoizen","profiles":{"yoizen":{"org":"o","project":"p","patEnvVar":"AZURE_DEVOPS_PAT"},"other":{"org":"o2","project":"p2","patEnvVar":"OTHER_PAT"}}}`)

	p, err := Create("dev", "dev")
	if err != nil {
		t.Fatal(err)
	}
	envAdo := filepath.Join(Dirs(p)["config"], "opencode", "ado.json")
	writeAdoJSON(t, envAdo,
		`{"defaultProfile":"mine","profiles":{"mine":{"org":"m","project":"mp","patEnvVar":"MINE_PAT"},"yoizen":{"org":"env-org","project":"env-p","patEnvVar":"ENV_PAT"}}}`)

	got, err := CopyAdoConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	// Only the missing profile copies; the env default and its own yoizen
	// entry win over global.
	if !slices.Equal(got.Profiles, []string{"other"}) || got.DefaultProfile {
		t.Fatalf("copy = %+v, want profiles [other], no default", got)
	}
	env, _ := readJSONObject(envAdo)
	profs := env["profiles"].(map[string]any)
	if profs["yoizen"].(map[string]any)["org"] != "env-org" {
		t.Errorf("env profile clobbered: %v", profs["yoizen"])
	}
	if _, ok := profs["other"]; !ok {
		t.Errorf("global profile missing after copy: %v", profs)
	}
	if env["defaultProfile"] != "mine" {
		t.Errorf("env default clobbered: %v", env["defaultProfile"])
	}

	// Idempotent.
	again, err := CopyAdoConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Profiles) != 0 || again.DefaultProfile {
		t.Errorf("second copy = %+v, want nothing", again)
	}
}

func TestCopyAdoConfigAdoptsDefaultWhenMissing(t *testing.T) {
	testRoot(t)
	xdgCfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgCfg)
	t.Setenv("YWAI_PROFILE", "")
	t.Setenv("OPENCODE_CONFIG_DIR", "")

	globalAdo := filepath.Join(config.OpenCodeUserConfigDir(), "ado.json")
	writeAdoJSON(t, globalAdo,
		`{"defaultProfile":"yoizen","profiles":{"yoizen":{"org":"o","project":"p","patEnvVar":"AZURE_DEVOPS_PAT"}}}`)

	p, err := Create("dev", "dev")
	if err != nil {
		t.Fatal(err)
	}
	got, err := CopyAdoConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Profiles, []string{"yoizen"}) || !got.DefaultProfile {
		t.Fatalf("copy = %+v, want profiles [yoizen] + default", got)
	}
}

func TestCopyAdoConfigNoGlobalIsEmpty(t *testing.T) {
	testRoot(t)
	xdgCfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgCfg)
	t.Setenv("YWAI_PROFILE", "")
	t.Setenv("OPENCODE_CONFIG_DIR", "")

	p, err := Create("dev", "dev")
	if err != nil {
		t.Fatal(err)
	}
	got, err := CopyAdoConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Profiles) != 0 || got.DefaultProfile {
		t.Errorf("copy without global config = %+v, want empty", got)
	}
}

func TestCopyAdoConfigRefusesInsideScope(t *testing.T) {
	testRoot(t)
	p, err := Create("dev", "dev")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("YWAI_PROFILE", "dev")
	if _, err := CopyAdoConfig(p); err == nil {
		t.Error("copy inside a profile scope must fail")
	}
}
