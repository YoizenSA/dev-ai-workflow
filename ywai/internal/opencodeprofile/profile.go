// Package opencodeprofile isolates OpenCode into role trees (dev, qa, infra).
// Each tree is a snapshot of ~/.config/opencode with a filtered agent set.
// Launch sets OPENCODE_CONFIG_DIR and XDG_DATA_HOME — same trick as opencode-multi.
package opencodeprofile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	agentprofiles "github.com/Yoizen/dev-ai-workflow/ywai/internal/agents"
)

const (
	NameDev   = "dev"
	NameQA    = "qa"
	NameInfra = "infra"
)

// Names is the fixed set ywai seeds and accepts on `ywai run`.
func Names() []string {
	return []string{NameDev, NameQA, NameInfra}
}

func ParseName(s string) (string, error) {
	n := strings.ToLower(strings.TrimSpace(s))
	switch n {
	case NameDev, NameQA, NameInfra:
		return n, nil
	default:
		return "", fmt.Errorf("unknown OpenCode profile %q (want %s)", s, strings.Join(Names(), ", "))
	}
}

// Dirs is one isolated OpenCode tree.
type Dirs struct {
	Config string // OPENCODE_CONFIG_DIR
	Data   string // XDG_DATA_HOME
}

func DirsFor(home, name string) (Dirs, error) {
	n, err := ParseName(name)
	if err != nil {
		return Dirs{}, err
	}
	root := filepath.Join(home, ".ywai", "opencode-profiles", n)
	return Dirs{
		Config: filepath.Join(root, "config"),
		Data:   filepath.Join(root, "data"),
	}, nil
}

func LaunchEnv(d Dirs, name string) []string {
	return []string{
		"OPENCODE_CONFIG_DIR=" + d.Config,
		"XDG_DATA_HOME=" + d.Data,
		"OPENCODE_PROFILE=" + name,
	}
}

// KeepAgent decides whether an installed agent belongs in a role tree.
func KeepAgent(profile, agent, group string) bool {
	base := filepath.Base(agent)
	switch profile {
	case NameDev:
		return group == "core" && base != "devops"
	case NameQA:
		return group == "qa-automation" || base == "orchestrator"
	case NameInfra:
		return base == "orchestrator" || base == "devops" || group == "experiment"
	default:
		return false
	}
}

func FilterProfiles(all map[string]agentprofiles.AgentProfile, profile string) map[string]agentprofiles.AgentProfile {
	out := make(map[string]agentprofiles.AgentProfile, len(all))
	for name, p := range all {
		if KeepAgent(profile, name, p.Group) {
			out[name] = p
		}
	}
	return out
}

// CopySharedConfig copies the kitchen-sink OpenCode config (plugins, MCP)
// into a profile tree. Agent markdown is written separately, filtered.
func CopySharedConfig(srcConfigDir, destConfigDir string) error {
	if err := os.MkdirAll(destConfigDir, 0o755); err != nil {
		return err
	}
	for _, name := range []string{"opencode.json", "opencode.jsonc", "AGENTS.md"} {
		src := filepath.Join(srcConfigDir, name)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := copyFile(src, filepath.Join(destConfigDir, name)); err != nil {
			return fmt.Errorf("copy %s: %w", name, err)
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
