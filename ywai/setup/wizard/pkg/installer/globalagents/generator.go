package globalagents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Logger is the minimal logging surface used by the generator. Matches the
// subset of *ui.Logger used by the rest of the installer.
type Logger interface {
	LogInfo(msg string)
	LogSuccess(msg string)
	LogWarning(msg string)
}

type noopLogger struct{}

func (noopLogger) LogInfo(string)    {}
func (noopLogger) LogSuccess(string) {}
func (noopLogger) LogWarning(string) {}

// Generator installs global agent files across all supported AI platforms.
type Generator struct {
	// ExtensionDir points to .../ywai/extensions/install-steps/global-agents.
	ExtensionDir string
	// SkillsDir points to .../ywai/skills (used to extract auto_invoke patterns).
	SkillsDir string
	// TypesJSON points to .../ywai/types/types.json (used to resolve
	// global_agents[type]). Optional; falls back to a hardcoded default list.
	TypesJSON string
	// ProjectType (nest, dotnet, generic, ...). Normalized internally.
	ProjectType string
	// Logger is optional.
	Logger Logger
}

type installPlan struct {
	dir         string
	target      Target
	fileFor     func(agent string) string
	displayName string
}

// InstallAll generates and writes the global agent files for the configured
// project type. Returns the total number of files written.
func (g Generator) InstallAll() (int, error) {
	logger := g.logger()

	projectType := normalizeProjectType(g.ProjectType)
	agents, err := g.resolveAgents(projectType)
	if err != nil {
		return 0, err
	}
	if len(agents) == 0 {
		return 0, fmt.Errorf("no global agents configured for project type %q", projectType)
	}

	bundles, err := LoadBundleConfig(filepath.Join(g.ExtensionDir, "bundles.json"))
	if err != nil {
		return 0, fmt.Errorf("load bundles.json: %w", err)
	}

	templatesDir := filepath.Join(g.ExtensionDir, "templates")

	plans := g.installPlans()

	managedNames := g.managedBasenames(agents, plans)
	written := 0

	for _, plan := range plans {
		if err := os.MkdirAll(plan.dir, 0o755); err != nil {
			logger.LogWarning(fmt.Sprintf("Failed to create %s: %v", plan.dir, err))
			continue
		}
		if err := removeManagedFiles(plan.dir, managedNames[plan.displayName]); err != nil {
			logger.LogWarning(fmt.Sprintf("Cleanup failed for %s: %v", plan.dir, err))
		}
		for _, agent := range agents {
			tmplBytes, err := os.ReadFile(filepath.Join(templatesDir, agent+".md"))
			if err != nil {
				if os.IsNotExist(err) {
					logger.LogWarning(fmt.Sprintf("Template missing for agent %q (project type %q)", agent, projectType))
					continue
				}
				return written, err
			}

			bundle := bundles.Bundle(projectType, agent)

			triggers := make(map[string]string, len(bundle))
			for _, skill := range bundle {
				patterns := AutoInvokePatterns(g.SkillsDir, skill)
				triggers[skill] = joinTriggers(patterns)
			}

			content := Render(RenderInput{
				AgentName:      agent,
				ProjectType:    projectType,
				Target:         plan.target,
				Template:       string(tmplBytes),
				Bundle:         bundle,
				SkillsTriggers: triggers,
			})

			dest := filepath.Join(plan.dir, plan.fileFor(agent))
			if err := os.WriteFile(dest, content, 0o644); err != nil {
				logger.LogWarning(fmt.Sprintf("Failed to write %s: %v", dest, err))
				continue
			}
			written++
		}
		logger.LogInfo(fmt.Sprintf("Installed %d global agent(s) -> %s", len(agents), plan.dir))
	}

	return written, nil
}

func (g Generator) logger() Logger {
	if g.Logger == nil {
		return noopLogger{}
	}
	return g.Logger
}

func (g Generator) resolveAgents(projectType string) ([]string, error) {
	if g.TypesJSON != "" {
		if agents, err := readGlobalAgentsFromTypes(g.TypesJSON, projectType); err != nil {
			return nil, err
		} else if len(agents) > 0 {
			return agents, nil
		}
	}
	return defaultAgentsFor(projectType), nil
}

func (g Generator) installPlans() []installPlan {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}

	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		xdgConfig = filepath.Join(home, ".config")
	}

	vscodeUserDir := vscodeUserProfileDir(home, xdgConfig)

	// Only the officially supported AI assistants are written: OpenCode,
	// Copilot (both the agent and the VS Code prompt directory) and Claude.
	// Cursor, Gemini and Codex are intentionally not part of the managed
	// pipeline; writing to ~/.cursor or ~/.gemini is no longer attempted
	// here to honor the "don't touch Cursor / Gemini" policy.
	return []installPlan{
		{
			dir:         filepath.Join(xdgConfig, "opencode", "agent"),
			target:      TargetOpenCode,
			fileFor:     func(a string) string { return a + ".md" },
			displayName: "opencode",
		},
		{
			dir:         filepath.Join(home, ".copilot", "agents"),
			target:      TargetCopilotAgent,
			fileFor:     func(a string) string { return a + ".md" },
			displayName: "copilot",
		},
		{
			dir:         filepath.Join(vscodeUserDir, "prompts"),
			target:      TargetCopilotPrompt,
			fileFor:     func(a string) string { return a + ".instructions.md" },
			displayName: "copilot-prompt",
		},
		{
			dir:         filepath.Join(home, ".claude", "agents"),
			target:      TargetClaude,
			fileFor:     func(a string) string { return a + ".md" },
			displayName: "claude",
		},
	}
}

// managedBasenames returns, for each platform, the set of file basenames the
// generator owns. Any other file in the destination directory is preserved.
func (g Generator) managedBasenames(agents []string, plans []installPlan) map[string]map[string]struct{} {
	out := make(map[string]map[string]struct{}, len(plans))
	for _, plan := range plans {
		set := make(map[string]struct{}, len(agents))
		for _, agent := range agents {
			set[plan.fileFor(agent)] = struct{}{}
		}
		out[plan.displayName] = set
	}
	return out
}

func removeManagedFiles(dir string, managed map[string]struct{}) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if _, ok := managed[e.Name()]; !ok {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func vscodeUserProfileDir(home, xdgConfig string) string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "Code", "User")
	}
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "Code", "User")
		}
		return filepath.Join(home, "AppData", "Roaming", "Code", "User")
	}
	return filepath.Join(xdgConfig, "Code", "User")
}

func readGlobalAgentsFromTypes(path, projectType string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	type projectEntry struct {
		GlobalAgents []string `json:"global_agents"`
	}
	var parsed struct {
		Types map[string]projectEntry `json:"types"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if entry, ok := parsed.Types[projectType]; ok {
		return entry.GlobalAgents, nil
	}
	return nil, nil
}

func normalizeProjectType(pt string) string {
	pt = strings.TrimSpace(strings.ToLower(pt))
	switch pt {
	case "nest", "nest-angular", "nest-react", "python", "dotnet", "qa-playwright", "devops", "generic":
		return pt
	case "":
		return "generic"
	default:
		return "generic"
	}
}

func defaultAgentsFor(projectType string) []string {
	switch projectType {
	case "nest":
		return []string{"sdd-orchestator", "nest-engineer", "devops"}
	case "nest-angular", "nest-react":
		return []string{"sdd-orchestator", "fe-engineer", "devops"}
	case "dotnet":
		return []string{"sdd-orchestator", "dotnet-engineer", "devops"}
	case "qa-playwright":
		return []string{"sdd-orchestator", "qa-playwright", "devops"}
	default:
		return []string{"sdd-orchestator", "devops"}
	}
}
