package envprofile

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// An env is a fresh opencode home: its opencode.json is built by the scoped
// apply, so `ado` profiles configured globally (`ado init`) are invisible
// inside the env when the CLI resolves them under the profile's config dir.
// CopyAdoConfig seeds the env's ado.json from the global one, mirroring
// CopyGlobalProviders. Entries the env already has win, so re-running never
// clobbers env-specific setup. It must run outside a profile sandbox (the
// global paths would resolve to the env itself).
//
// Only ado.json is copied: the PAT itself lives in ~/.azure-devops-cli/pat
// (or AZURE_DEVOPS_PAT), both HOME-based and therefore already visible
// inside envs — HOME is never redirected by the sandbox. No secret is
// duplicated by this copy.

// CopiedAdo reports what CopyAdoConfig added to an env.
type CopiedAdo struct {
	// Profiles are the profile names added from the global ado.json.
	Profiles []string `json:"profiles"`
	// DefaultProfile is true when the global default was adopted because the
	// env had none.
	DefaultProfile bool `json:"defaultProfile,omitempty"`
}

// CopyAdoConfig merges the global ado.json profiles into the env's ado.json.
func CopyAdoConfig(p Profile) (CopiedAdo, error) {
	var out CopiedAdo
	if InProfileScope() {
		return out, fmt.Errorf("copy ado config must run outside a profile scope")
	}
	dirs := Dirs(p)
	globalCfg := filepath.Join(config.OpenCodeUserConfigDir(), "ado.json")
	envCfg := filepath.Join(dirs["config"], "opencode", "ado.json")
	if filepath.Clean(globalCfg) == filepath.Clean(envCfg) {
		return out, fmt.Errorf("global and env ado config resolve to the same file")
	}

	global, err := readJSONObject(globalCfg)
	if err != nil {
		return out, err
	}
	if len(global) == 0 {
		return out, nil // no global ado config to copy
	}
	env, err := readJSONObject(envCfg)
	if err != nil {
		return out, err
	}

	gp, _ := global["profiles"].(map[string]any)
	if len(gp) > 0 {
		ep, _ := env["profiles"].(map[string]any)
		if ep == nil {
			ep = map[string]any{}
		}
		for name, def := range gp {
			if _, exists := ep[name]; !exists {
				ep[name] = def
				out.Profiles = append(out.Profiles, name)
			}
		}
		if len(out.Profiles) > 0 {
			env["profiles"] = ep
		}
	}
	if _, ok := env["defaultProfile"]; !ok {
		if def, ok := global["defaultProfile"].(string); ok && def != "" {
			env["defaultProfile"] = def
			out.DefaultProfile = true
		}
	}
	if len(out.Profiles) > 0 || out.DefaultProfile {
		// 0600: profiles name the PAT env var, never the PAT itself, but
		// keep the file private like the providers copy does.
		if err := writeJSONObject(envCfg, env, 0o600); err != nil {
			return out, err
		}
	}
	sort.Strings(out.Profiles)
	return out, nil
}
