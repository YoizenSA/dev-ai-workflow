package agents

import (
	"fmt"
	"os"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// rootShellRules are the shell permissions for the primary (unnamed) agent.
//
// opencode resolves a request with findLast over the rule list and falls back
// to "ask" when nothing matches. A TUI session shows that prompt to a human; a
// headless one (orca, `ywai serve`, background delegations) has nobody to
// answer, so an unmatched action is effectively denied. Every ywai agent
// carries its own `- action: shell` rule in its markdown frontmatter, but the
// primary agent has no markdown file: its only source is the root `permission`
// block of opencode.json. With no rule there, every command it ran was denied
// — including read-only ones like grep.
//
// So the baseline is "*": "allow", matching what opencode itself ships as the
// default primary agent, and the destructive commands are walked back to "ask".
//
// Ordering is load-bearing. opencode turns this object into rules with
// Object.entries and then picks the LAST match, not the most specific one, so
// the broad "*" has to be serialized before the narrow patterns. Go marshals
// map keys sorted, and "*" (0x2A) sorts below every character a command starts
// with, which puts it first. TestRootShellRulesSerializeWildcardFirst pins
// that down.
//
// ponytail: "ask" is the strongest gate the config format has — the effect enum
// is exactly allow/deny/ask, with no repeat-confirmation step. It means one
// prompt in the TUI and a refusal in headless. If a command must never run even
// with a human watching, change its effect to "deny" rather than looking for a
// second confirmation that does not exist.
var rootShellRules = map[string]any{
	"*": "allow",

	// Rewriting or discarding history that is already pushed.
	"git push*--force*":     "ask",
	"git push*-f*":          "ask",
	"git reset*--hard*":     "ask",
	"git clean*-*f*":        "ask",
	"git rebase*":           "ask",
	"git filter-branch*":    "ask",
	"git branch*-D*":        "ask",
	"git tag*-d*":           "ask",
	"git push*--delete*":    "ask",
	"git push*--mirror*":    "ask",
	"git checkout*--force*": "ask",

	// Destroying files or infrastructure outside git's reach.
	"rm -rf*":              "ask",
	"rm -fr*":              "ask",
	"sudo*":                "ask",
	"chmod -R*":            "ask",
	"chown -R*":            "ask",
	"mkfs*":                "ask",
	"dd if=*":              "ask",
	"docker system prune*": "ask",
	"docker volume rm*":    "ask",
	"kubectl delete*":      "ask",
	"terraform destroy*":   "ask",
	"terraform apply*":     "ask",

	// Publishing: irreversible once it leaves the machine.
	"npm publish*":       "ask",
	"pnpm publish*":      "ask",
	"yarn publish*":      "ask",
	"gh release create*": "ask",
	"dotnet nuget push*": "ask",

	// Piping a remote script straight into a shell.
	"curl*|*sh*":   "ask",
	"curl*|*bash*": "ask",
	"wget*|*sh*":   "ask",
	"wget*|*bash*": "ask",
}

// EnsureRootShellPermission fills in the root `permission.shell` block of
// opencode.json. Existing patterns are never overwritten, so a rule the user
// narrowed or denied by hand survives a re-install, and a blanket scalar
// ("shell": "allow") is left alone entirely.
//
// The key is "shell", opencode v2's action name. ywai's internal vocabulary
// calls the same bucket "bash" (see permissions_v2.go); writing that key here
// would do nothing, because v2 does not know it.
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
	if _, isScalar := perm["shell"].(string); isScalar {
		return nil // a blanket decision, made deliberately: leave it
	}
	shell, ok := perm["shell"].(map[string]any)
	if !ok {
		shell = map[string]any{}
	}

	added := 0
	for pattern, effect := range rootShellRules {
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
	fmt.Printf("  Granted %d shell rule(s) to the primary agent\n", added)
	return nil
}
