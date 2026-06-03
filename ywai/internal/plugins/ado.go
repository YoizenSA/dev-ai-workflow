package plugins

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ADOProfile defines a single Azure DevOps profile configuration.
type ADOProfile struct {
	Org       string   `json:"org"`
	PATEnvVar string   `json:"patEnvVar"`
	Project   string   `json:"project"`
	Repos     []string `json:"repos"`
	Default   bool     `json:"default,omitempty"`
}

// ADOPluginConfig is the config passed as the second element in the plugin array.
type ADOPluginConfig struct {
	DefaultProfile string                `json:"defaultProfile"`
	Profiles       map[string]ADOProfile `json:"profiles"`
}

// ─── Shared config paths ─────────────────────────────────────────────────

func adoConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".azure-devops-cli")
}

func adoSharedConfigPath() string {
	dir := adoConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "config.json")
}

func adoPATPath() string {
	dir := adoConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "pat")
}

func ADODefaultConfigExists() bool {
	p := adoSharedConfigPath()
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

// ─── Interactive setup ────────────────────────────────────────────────────

// ADOSetup holds the collected answers from the interactive setup.
type ADOSetup struct {
	ProfileName string
	Org         string
	Project     string
	Repos       []string
	PAT         string
	PATEnvVar   string
}

// RunADOSetup runs an interactive prompt to configure the ADO plugin.
// Returns the setup data or an error.
func RunADOSetup() (*ADOSetup, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n=== Azure DevOps Plugin Setup ===")
	fmt.Println()

	// 1. Profile name
	fmt.Print("  Profile name (e.g. work, myorg): ")
	profileName, _ := reader.ReadString('\n')
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		profileName = "default"
	}

	// 2. Organization
	fmt.Println()
	fmt.Println("  Your Azure DevOps organization URL.")
	fmt.Println("  Examples: https://dev.azure.com/myorg  or  myorg (shorthand)")
	fmt.Print("  Organization: ")
	org, _ := reader.ReadString('\n')
	org = strings.TrimSpace(org)
	if org == "" {
		return nil, fmt.Errorf("organization is required")
	}
	// Normalize: if no https:// prefix, add shorthand form
	if !strings.HasPrefix(org, "http") && !strings.HasPrefix(org, "https://dev.azure.com/") {
		// Keep as shorthand — the plugin supports both
	}

	// 3. Project
	fmt.Print("  Project name: ")
	project, _ := reader.ReadString('\n')
	project = strings.TrimSpace(project)
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}

	// 4. Repos
	fmt.Print("  Repositories (comma-separated, or Enter for all): ")
	reposStr, _ := reader.ReadString('\n')
	reposStr = strings.TrimSpace(reposStr)
	var repos []string
	if reposStr != "" {
		for _, r := range strings.Split(reposStr, ",") {
			r = strings.TrimSpace(r)
			if r != "" {
				repos = append(repos, r)
			}
		}
	}

	// 5. PAT
	patEnvVar := "AZURE_DEVOPS_PAT"
	fmt.Println()
	fmt.Println("  You need a Personal Access Token (PAT) with scopes:")
	fmt.Println("    • Code (Read & Write)")
	fmt.Println("    • Pull Request Contribute (Read & Write)")
	fmt.Println("    • Work Items (Read)")
	fmt.Println()
	fmt.Println("  Generate one at:")
	fmt.Printf("    https://dev.azure.com/%s/_usersSettings/tokens\n", org)
	fmt.Println()
	fmt.Print("  Paste your PAT (hidden): ")
	pat, _ := reader.ReadString('\n')
	pat = strings.TrimSpace(pat)
	if pat == "" {
		return nil, fmt.Errorf("PAT is required")
	}

	return &ADOSetup{
		ProfileName: profileName,
		Org:         org,
		Project:     project,
		Repos:       repos,
		PAT:         pat,
		PATEnvVar:   patEnvVar,
	}, nil
}

// SavePAT persists the PAT to disk and shell profile.
// It saves to:
//   - ~/.azure-devops-cli/pat  (used by the plugin)
//   - shell profile file (.bashrc, .zshrc, .profile, etc.) as export AZURE_DEVOPS_PAT=xxx
func SavePAT(pat string) error {
	// 1. Save to ~/.azure-devops-cli/pat
	configDir := adoConfigDir()
	if configDir == "" {
		return fmt.Errorf("cannot determine home directory")
	}

	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	patPath := adoPATPath()
	if err := os.WriteFile(patPath, []byte(pat+"\n"), 0o600); err != nil {
		return fmt.Errorf("failed to write PAT file: %w", err)
	}
	fmt.Printf("  ✓ PAT saved to %s\n", patPath)

	// 2. Persist to shell profile
	shellRC := detectShellProfile()
	if shellRC != "" {

		// Check if already exported in the profile
		content, err := os.ReadFile(shellRC)
		if err != nil {
			content = []byte{}
		}

		if strings.Contains(string(content), "AZURE_DEVOPS_PAT") {
			// Update existing — replace the line
			lines := strings.Split(string(content), "\n")
			var updated []string
			for _, line := range lines {
				if strings.HasPrefix(line, "export AZURE_DEVOPS_PAT=") {
					updated = append(updated, fmt.Sprintf("export AZURE_DEVOPS_PAT=\"%s\"", pat))
				} else {
					updated = append(updated, line)
				}
			}
			os.WriteFile(shellRC, []byte(strings.Join(updated, "\n")), 0o644)
		} else {
			// Append new export
			f, err := os.OpenFile(shellRC, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
			if err == nil {
				defer f.Close()
				exportLine := fmt.Sprintf("\n# Azure DevOps PAT (set by ywai)\nexport AZURE_DEVOPS_PAT=\"%s\"\n", pat)
				f.WriteString(exportLine)
			}
		}
		fmt.Printf("  ✓ AZURE_DEVOPS_PAT exported in %s\n", shellRC)
		fmt.Println("    Run 'source " + shellRC + "' or open a new terminal to load it.")
	} else {
		fmt.Println("  ⚠ Could not detect shell profile. Add this to your shell config manually:")
		fmt.Println("    export AZURE_DEVOPS_PAT=\"<your-pat>\"")
	}

	return nil
}

// detectShellProfile finds the appropriate shell profile file for the current OS.
func detectShellProfile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch runtime.GOOS {
	case "windows":
		// Windows: persist to PowerShell profile
		psProfile := filepath.Join(home, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1")
		if _, err := os.Stat(psProfile); err == nil {
			return psProfile
		}
		// Try PowerShell 7 profile
		ps7Profile := filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
		if _, err := os.Stat(ps7Profile); err == nil {
			return ps7Profile
		}
		return ""

	default: // linux, darwin
		// Detect current shell
		shell := os.Getenv("SHELL")

		candidates := []string{}

		if strings.Contains(shell, "zsh") {
			zshrc := filepath.Join(home, ".zshrc")
			zprofile := filepath.Join(home, ".zprofile")
			candidates = append(candidates, zshrc, zprofile)
		} else if strings.Contains(shell, "fish") {
			fishConfig := filepath.Join(home, ".config", "fish", "config.fish")
			candidates = append(candidates, fishConfig)
		} else {
			// bash or unknown
			bashrc := filepath.Join(home, ".bashrc")
			bashProfile := filepath.Join(home, ".bash_profile")
			profile := filepath.Join(home, ".profile")
			candidates = append(candidates, bashrc, bashProfile, profile)
		}

		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}

		// Fallback: create .bashrc or .zshrc based on shell
		if strings.Contains(shell, "zsh") {
			return filepath.Join(home, ".zshrc")
		}
		return filepath.Join(home, ".bashrc")
	}
}

// ─── Config writers ───────────────────────────────────────────────────────

// InstallADOFromSetup runs the full ADO setup interactively and installs everything.
func InstallADOFromSetup() error {
	setup, err := RunADOSetup()
	if err != nil {
		return err
	}

	// 1. Save PAT
	fmt.Println()
	if err := SavePAT(setup.PAT); err != nil {
		fmt.Printf("  Warning: PAT persistence failed: %v\n", err)
		fmt.Println("  The PAT file was saved but the shell export may need manual setup.")
	}

	// 2. Write shared config
	adoConfig := ADOPluginConfig{
		DefaultProfile: setup.ProfileName,
		Profiles: map[string]ADOProfile{
			setup.ProfileName: {
				Org:       setup.Org,
				PATEnvVar: setup.PATEnvVar,
				Project:   setup.Project,
				Repos:     setup.Repos,
				Default:   true,
			},
		},
	}

	if err := InstallADODefaultConfig(adoConfig); err != nil {
		return fmt.Errorf("failed to write ADO config: %w", err)
	}

	fmt.Println()
	fmt.Printf("  ✓ ADO profile '%s' configured:\n", setup.ProfileName)
	fmt.Printf("    org: %s | project: %s\n", setup.Org, setup.Project)
	if len(setup.Repos) > 0 {
		fmt.Printf("    repos: %s\n", strings.Join(setup.Repos, ", "))
	}

	return nil
}

// InstallADODefaultConfig writes the shared ~/.azure-devops-cli/config.json.
func InstallADODefaultConfig(config ADOPluginConfig) error {
	configPath := adoSharedConfigPath()
	if configPath == "" {
		return fmt.Errorf("cannot determine home directory")
	}

	configDir := adoConfigDir()
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("failed to create config dir %s: %w", configDir, err)
	}

	// Read existing config to merge
	existing := map[string]any{}
	if data, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(data, &existing)
	}

	if _, ok := existing["ado"]; !ok {
		existing["ado"] = config
	} else {
		if existingAdo, ok := existing["ado"].(map[string]any); ok {
			if profiles, ok := existingAdo["profiles"].(map[string]any); ok {
				for k, v := range config.Profiles {
					profiles[k] = v
				}
			} else {
				existingAdo["profiles"] = config.Profiles
			}
			if config.DefaultProfile != "" {
				existingAdo["defaultProfile"] = config.DefaultProfile
			}
		}
	}

	updated, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	updated = append(updated, '\n')

	if err := os.WriteFile(configPath, updated, 0o644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Printf("  ✓ Config saved to %s\n", configPath)
	return nil
}

// InstallADOOpenCode adds the @nahuelcio/opencode-ado plugin to opencode.json.
func InstallADOOpenCode(configPath string, adoConfig ADOPluginConfig) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	pluginsRaw, ok := root["plugin"]
	if !ok {
		pluginsRaw = []any{}
		root["plugin"] = pluginsRaw
	}

	plugins, ok := pluginsRaw.([]any)
	if !ok {
		plugins = []any{}
		root["plugin"] = plugins
	}

	pluginName := "@nahuelcio/opencode-ado"

	for i, p := range plugins {
		if arr, ok := p.([]any); ok && len(arr) >= 1 {
			if name, ok := arr[0].(string); ok && name == pluginName {
				plugins[i] = []any{pluginName, adoConfig}
				goto write
			}
		}
	}

	plugins = append(plugins, []any{pluginName, adoConfig})
	root["plugin"] = plugins

write:
	updated, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	updated = append(updated, '\n')

	if err := os.WriteFile(configPath, updated, 0o644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// InstallADOPi adds @nahuelcio/opencode-ado to Pi's packages list in settings.json.
func InstallADOPi(piSettingsPath string) error {
	data, err := os.ReadFile(piSettingsPath)
	if err != nil {
		root := map[string]any{
			"packages": []any{"npm:@nahuelcio/opencode-ado"},
		}
		updated, _ := json.MarshalIndent(root, "", "  ")
		updated = append(updated, '\n')
		return os.WriteFile(piSettingsPath, updated, 0o644)
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("failed to parse Pi settings: %w", err)
	}

	packagesRaw, ok := root["packages"]
	if !ok {
		packagesRaw = []any{}
		root["packages"] = packagesRaw
	}

	packages, ok := packagesRaw.([]any)
	if !ok {
		packages = []any{}
		root["packages"] = packages
	}

	pluginPkg := "npm:@nahuelcio/opencode-ado"
	for _, p := range packages {
		if s, ok := p.(string); ok && s == pluginPkg {
			return nil
		}
	}

	packages = append(packages, pluginPkg)
	root["packages"] = packages

	updated, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal Pi settings: %w", err)
	}
	updated = append(updated, '\n')

	if err := os.WriteFile(piSettingsPath, updated, 0o644); err != nil {
		return fmt.Errorf("failed to write Pi settings: %w", err)
	}

	return nil
}

// ReadExistingADOConfig reads the existing shared ADO config if it exists.
func ReadExistingADOConfig() (*ADOPluginConfig, error) {
	configPath := adoSharedConfigPath()
	if configPath == "" {
		return nil, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, nil // doesn't exist
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, nil
	}

	adoRaw, ok := root["ado"]
	if !ok {
		return nil, nil
	}

	// Re-marshal and unmarshal to get typed struct
	adoJSON, err := json.Marshal(adoRaw)
	if err != nil {
		return nil, nil
	}

	var config ADOPluginConfig
	if err := json.Unmarshal(adoJSON, &config); err != nil {
		return nil, nil
	}

	return &config, nil
}
