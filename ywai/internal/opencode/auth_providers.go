package opencode

import (
	"context"
	"encoding/json"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// authProviderEntry is one provider in `opencode auth list --format json`.
type authProviderEntry struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Connections []json.RawMessage `json:"connections"`
}

// AuthedProviders returns the provider ids that hold at least one stored
// credential, per the OpenCode CLI (`auth list --format json`). The CLI asks
// the same background service that `run` attaches to, so this reflects the
// credentials the CLI actually uses — unlike the on-disk auth.json, which only
// carries legacy direct-API keys.
//
// Best-effort: any failure (CLI missing, service down, unparsable output)
// yields an empty list so callers degrade to "no filtering" instead of hiding
// usable models.
func AuthedProviders(ctx context.Context) []string {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := runOpencodeAuthList(ctx)
	if err != nil {
		return nil
	}
	return parseAuthedProviders(out)
}

// runOpencodeAuthList runs `opencode auth list --format json` cross-platform,
// mirroring runOpencodeModels (a Windows .cmd/.bat shim needs `cmd /c`, a .ps1
// needs PowerShell).
func runOpencodeAuthList(ctx context.Context) ([]byte, error) {
	bin := resolveOpencodeBin()
	lower := strings.ToLower(bin)
	args := []string{"auth", "list", "--format", "json"}
	var cmd *exec.Cmd
	switch {
	case runtime.GOOS == "windows" && (strings.HasSuffix(lower, ".cmd") || strings.HasSuffix(lower, ".bat")):
		cmd = exec.CommandContext(ctx, "cmd", append([]string{"/c", bin}, args...)...)
	case runtime.GOOS == "windows" && strings.HasSuffix(lower, ".ps1"):
		cmd = exec.CommandContext(ctx, "powershell", append([]string{"-NoProfile", "-File", bin}, args...)...)
	default:
		cmd = exec.CommandContext(ctx, bin, args...)
	}
	cmd.Env = opencodeEnv()
	return cmd.Output()
}

// parseAuthedProviders extracts the ids of providers with at least one
// connection (stored credential) from `auth list --format json` output.
func parseAuthedProviders(data []byte) []string {
	var entries []authProviderEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.ID == "" || len(e.Connections) == 0 {
			continue
		}
		ids = append(ids, e.ID)
	}
	return ids
}
