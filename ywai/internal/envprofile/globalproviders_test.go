package envprofile

import (
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const credentialDDL = "CREATE TABLE `credential` (`id` text PRIMARY KEY, `integration_id` text, `label` text NOT NULL, `value` text NOT NULL, `connector_id` text, `method_id` text, `active` integer, `time_created` integer NOT NULL, `time_updated` integer NOT NULL)"

func makeCredentialDB(t *testing.T, path string, rows [][]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(credentialDDL); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if _, err := db.Exec("INSERT INTO credential VALUES (?,?,?,?,?,?,?,?,?)", r...); err != nil {
			t.Fatal(err)
		}
	}
}

func credentialCount(t *testing.T, path string) int {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow("SELECT count(*) FROM credential").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCopyGlobalProvidersMergesWithoutClobbering(t *testing.T) {
	testRoot(t)
	xdgCfg, xdgData := t.TempDir(), t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgCfg)
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("YWAI_PROFILE", "")
	t.Setenv("OPENCODE_CONFIG_DIR", "")

	mustWrite := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(filepath.Join(xdgCfg, "opencode", "opencode.json"),
		`{"model":"global/m","provider":{"harbor":{"options":{"apiKey":"g"}},"shared":{"options":{"apiKey":"global"}}}}`)
	makeCredentialDB(t, filepath.Join(xdgData, "opencode", "opencode.db"), [][]any{
		{"c1", "opencode-go", "default", `{"key":"k1"}`, nil, "key", 1, 1, 1},
		{"c2", "zai", "Z.AI", `{"key":"k2"}`, nil, "key", 1, 1, 1},
	})

	p, err := Create("dev", "dev")
	if err != nil {
		t.Fatal(err)
	}
	envCfg := filepath.Join(Dirs(p)["config"], "opencode", "opencode.json")
	mustWrite(envCfg, `{"model":"env/m","provider":{"shared":{"options":{"apiKey":"env"}}}}`)

	// Before the env ever started there is no database: providers copy, logins
	// are deferred with a note instead of failing.
	got, err := CopyGlobalProviders(p)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Providers, []string{"harbor"}) || len(got.Credentials) != 0 || got.Note == "" {
		t.Fatalf("first copy = %+v, want providers [harbor], no creds, a note", got)
	}
	env, _ := readJSONObject(envCfg)
	if env["model"] != "env/m" {
		t.Errorf("env model clobbered: %v", env["model"])
	}
	shared := env["provider"].(map[string]any)["shared"].(map[string]any)["options"].(map[string]any)
	if shared["apiKey"] != "env" {
		t.Errorf("env provider clobbered: %v", shared)
	}

	// Once opencode created the env database, logins copy; an existing
	// same-integration login in the env is kept.
	envDB := Env(p)["OPENCODE_DB"]
	makeCredentialDB(t, envDB, [][]any{{"mine", "zai", "Z.AI", `{"key":"env"}`, nil, "key", 1, 2, 2}})
	got, err = CopyGlobalProviders(p)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Credentials, []string{"opencode-go"}) || got.Note != "" {
		t.Fatalf("second copy = %+v, want creds [opencode-go]", got)
	}
	if n := credentialCount(t, envDB); n != 2 {
		t.Errorf("env credentials = %d, want 2", n)
	}
	// Idempotent.
	again, err := CopyGlobalProviders(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Providers)+len(again.Credentials) != 0 {
		t.Errorf("third copy = %+v, want nothing", again)
	}
}

func TestCopyGlobalProvidersRefusesInsideScope(t *testing.T) {
	testRoot(t)
	p, err := Create("dev", "dev")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("YWAI_PROFILE", "dev")
	if _, err := CopyGlobalProviders(p); err == nil {
		t.Error("copy inside a profile scope must fail")
	}
}

func TestPresetCopyProvidersDefaults(t *testing.T) {
	for name, want := range map[string]bool{"dev": true, "qa": true, "personal": false} {
		spec, err := Preset(name)
		if err != nil {
			t.Fatal(err)
		}
		if got := PresetCopyProviders(spec); got != want {
			t.Errorf("PresetCopyProviders(%s) = %v, want %v", name, got, want)
		}
	}
}
