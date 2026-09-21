package agents

import (
	"fmt"
	"os"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// headlessShellAllows are the shell patterns the primary (unnamed) agent must
// be able to run without a prompt.
//
// opencode's default for an action with no rule is "ask". In a TUI session a
// human answers it; in a headless session (orca, `ywai serve`, background
// delegations) nobody does and the request is auto-denied. Every ywai agent
// carries its own `- action: shell` rule in its markdown frontmatter, but the
// primary agent has no markdown file: its only permission source is the root
// `permission` block of opencode.json, which ywai never wrote. The result was a
// headless session that could commit but never push.
//
// ponytail: only the delivery commands observed to be auto-denied. If another
// command starts failing headless the same way, add its pattern here rather
// than widening the key to a blanket "allow" — that would also become the
// fallback for any agent whose markdown rule is missing.
var headlessShellAllows = map[string]any{
	"git push*": "allow",
}

// EnsureRootShellPermission adds the missing shell rules to the root
// `permission` block of opencode.json. Existing keys are never overwritten, so
// a user who denied or narrowed a pattern by hand keeps their decision.
//
// "shell" is opencode v2's action name for running commands (ywai's internal
// vocabulary calls the same bucket "bash"; see permissions_v2.go). Writing
// "bash" here does nothing — v2 does not know that key.
func EnsureRootShellPermission(configPath string) error {
	if _, err := os.Stat(configPath); err != nil {
		return nil // no config yet: install writes it later
	}
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return err
	}

	perm, _ := root["permission"].(map[string]any)
	if perm == nil {
		perm = map[string]any{}
	}
	// A scalar ("allow"/"deny"/"ask") is a deliberate blanket decision: leave it.
	shell, isMap := perm["shell"].(map[string]any)
	if _, isScalar := perm["shell"].(string); isScalar {
		return nil
	}
	if !isMap {
		shell = map[string]any{}
	}

	added := 0
	for pattern, effect := range headlessShellAllows {
		if _, exists := shell[pattern]; exists {
			continue
		}
		shell[pattern] = effect
		added++
	}
	if added == 0 {
		return nil
	}

	perm["shell"] = shell
	root["permission"] = perm
	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	fmt.Printf("  Granted %d shell pattern(s) to the primary agent\n", added)
	return nil
}
