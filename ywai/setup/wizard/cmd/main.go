package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/setup/wizard/pkg/installer"
	versionresolver "github.com/Yoizen/dev-ai-workflow/ywai/setup/wizard/pkg/installer/version"
)

// Version information (set during build)
var buildVersion = "dev"

func main() {
	flags := parseFlags()

	// Launch TUI when no arguments and not in non-interactive mode
	if !flags.NonInteractive && len(os.Args) == 1 {
		if _, err := runInteractive(flags); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle version flag
	if flags.Help {
		showHelp()
		os.Exit(0)
	}

	if flags.VersionFlag && !hasInstallIntent(flags) {
		fmt.Printf("YWAI Setup Wizard %s\n", formatDisplayVersion(buildVersion))
		os.Exit(0)
	}

	// Handle self-update
	if flags.SelfUpdate {
		if err := runSelfUpdate(flags); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle SDD profiles
	if flags.SDDProfiles {
		if err := runSDDProfiles(flags); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle global agents update
	if flags.UpdateGlobalAgents {
		if err := runUpdateGlobalAgents(flags); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	checkForUpdates(flags)

	inst := installer.New(flags)

	if err := inst.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if shouldShowNextSteps(flags) {
		inst.ShowNextSteps()
	}
}

func hasInstallIntent(flags *installer.Flags) bool {
	return flags.All ||
		flags.InstallGA ||
		flags.InstallSDD ||
		flags.InstallVSCode ||
		flags.InstallGlobal ||
		flags.InstallExt ||
		flags.SkipGA ||
		flags.SkipSDD ||
		flags.SkipVSCode ||
		flags.UpdateAll ||
		flags.SelfUpdate ||
		flags.SDDProfiles ||
		flags.UpdateGlobalAgents ||
		flags.DryRun ||
		flags.Force ||
		flags.ListTypes ||
		flags.ListExtensions ||
		flags.ListInstallableSkills ||
		flags.Sync ||
		flags.InstallSkill != "" ||
		len(flags.InstallSkills) > 0 ||
		flags.Target != "" ||
		flags.ProjectType != "" ||
		flags.Version != "" ||
		flags.Channel != "stable"
}

func shouldShowNextSteps(flags *installer.Flags) bool {
	return !flags.Help &&
		!flags.VersionFlag &&
		!flags.ListTypes &&
		!flags.ListExtensions &&
		!flags.ListInstallableSkills &&
		!flags.Sync &&
		flags.InstallSkill == "" &&
		len(flags.InstallSkills) == 0
}

func parseFlags() *installer.Flags {
	flags := &installer.Flags{}

	flag.BoolVar(&flags.All, "all", false, "Install everything")
	flag.BoolVar(&flags.InstallGA, "install-ga", false, "Install GA")
	flag.BoolVar(&flags.InstallSDD, "install-sdd", false, "Install SDD")
	flag.BoolVar(&flags.InstallVSCode, "install-vscode", false, "Install VS Code")
	flag.BoolVar(&flags.InstallGlobal, "global-skills", false, "Install global agents")
	flag.BoolVar(&flags.InstallExt, "extensions", false, "Install extensions")
	flag.BoolVar(&flags.SkipGA, "skip-ga", false, "Skip GA")
	flag.BoolVar(&flags.SkipSDD, "skip-sdd", false, "Skip SDD")
	flag.BoolVar(&flags.SkipVSCode, "skip-vscode", false, "Skip VS Code")
	flag.BoolVar(&flags.UpdateAll, "update-all", false, "Update all")
	flag.BoolVar(&flags.SelfUpdate, "self-update", false, "Update ywai to latest version")
	flag.BoolVar(&flags.SDDProfiles, "sdd-profiles", false, "Manage SDD profiles for per-phase model assignment")
	flag.BoolVar(&flags.UpdateGlobalAgents, "update-global-agents", false, "Update global agents to latest version")
	flag.BoolVar(&flags.SkipGlobalAgentsUpdate, "skip-global-agents-update", false, "Skip global agents update during self-update")
	flag.BoolVar(&flags.Force, "force", false, "Force reinstall")
	flag.BoolVar(&flags.Silent, "silent", false, "Minimal output")
	flag.BoolVar(&flags.DryRun, "dry-run", false, "Show what would happen")
	flag.BoolVar(&flags.Help, "h", false, "Show help")
	flag.BoolVar(&flags.Help, "help", false, "Show help")
	flag.BoolVar(&flags.NonInteractive, "no-interactive", false, "Disable interactive mode")

	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "Show version")

	flag.BoolVar(&flags.ListTypes, "list-types", false, "List project types")
	flag.BoolVar(&flags.ListExtensions, "list-extensions", false, "List extensions for a project type")
	flag.BoolVar(&flags.ListInstallableSkills, "list-installable-skills", false, "List skills available to install in this repo")
	flag.BoolVar(&flags.Sync, "sync", false, "Generate sync report for LLM")
	flag.StringVar(&flags.InstallSkill, "install-skill", "", "Install specific skill (e.g., angular/signals)")
	var installSkillsCSV string
	flag.StringVar(&installSkillsCSV, "install-skills", "", "Install multiple skills separated by commas")
	flag.StringVar(&flags.Provider, "provider", "opencode", "LLM provider")
	flag.StringVar(&flags.Target, "target", "", "Target directory")
	flag.StringVar(&flags.ProjectType, "type", "", "Project type")
	flag.StringVar(&flags.Preset, "preset", "", "Install preset: minimal | standard | full (default: standard)")
	flag.StringVar(&flags.Version, "install-version", "", "Specific version to install")
	flag.StringVar(&flags.Channel, "channel", "stable", "Release channel")

	flag.Parse()

	// Handle environment variable overrides
	if envVersion := os.Getenv("YWAI_VERSION"); envVersion != "" {
		flags.Version = envVersion
	}
	if envChannel := os.Getenv("YWAI_CHANNEL"); envChannel != "" {
		flags.Channel = envChannel
	}

	if flag.NArg() > 0 && flags.Target == "" {
		flags.Target = flag.Arg(0)
	}
	if installSkillsCSV != "" {
		for _, part := range strings.Split(installSkillsCSV, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				flags.InstallSkills = append(flags.InstallSkills, part)
			}
		}
	}

	// Set version flag for main function
	flags.VersionFlag = showVersion
	flags.BuildVersion = normalizeBuildVersion(buildVersion)

	return flags
}

func showHelp() {
	fmt.Println(`YWAI Setup Wizard - Go Binary Version

USAGE:
    ywai [OPTIONS] [target-directory]

OPTIONS:
    --all               Install the full recommended setup
    --install-ga        Install GA (Guardian Agent)
    --install-sdd       Install SDD Orchestrator
    --install-vscode    Install VS Code / Copilot extensions
    --global-skills     Install global agents
    --extensions       Install project extensions
    --update-all        Refresh an existing YWAI installation
    --self-update       Update ywai to latest version
    --sdd-profiles      Manage SDD profiles for per-phase model assignment
    --update-global-agents Update global agents to latest version
    --sync              Generate sync report for LLM (no changes made)
    --install-skill SKILL   Install one specific skill (e.g. angular/signals)
    --install-skills A,B    Install multiple skills at once
    --force             Force reinstall / overwrite managed files
    --silent            Minimal output
    --dry-run           Preview changes without writing anything
    --version           Show version information
    --list-types        List available project types
    --list-extensions   List extensions for a project type
    --list-installable-skills  List installable skills missing from this repo
    --provider PROVIDER Main AI provider (default: opencode)
    --type TYPE         Project type
    --preset NAME       Install preset: minimal | standard | full (default: standard)
    --install-version VERSION   Specific version to install
    --channel CHANNEL   Release channel (stable/latest)
    --help, -h          Show this help
    --no-interactive    Disable interactive mode (CLI only)

EXAMPLES:
    ywai                                   # Interactive guided setup
    ywai --all --type=nest                 # Full install in current repo
    ywai --update-all                      # Refresh an existing setup
    ywai --self-update                     # Update ywai to latest version
    ywai --sync                            # Generate sync report
    ywai --list-installable-skills         # Show missing installable skills
    ywai --install-skills typescript,biome # Install multiple skills
    ywai --sync --type=nest-angular        # Sync with specific type
    ywai --install-skill angular/signals   # Install one skill
    ywai --dry-run --all                   # Preview full install
    ywai --version

ENVIRONMENT VARIABLES:
    YWAI_VERSION        Specific version (overrides --install-version)
    YWAI_CHANNEL        Release channel (overrides --channel)

For more information, visit: https://github.com/Yoizen/dev-ai-workflow`)
}

func checkForUpdates(flags *installer.Flags) {
	if shouldSkipUpdateCheck() {
		return
	}

	// Check if we should do periodic update check
	if !shouldCheckForPeriodicUpdate() {
		return
	}

	current := normalizeBuildVersion(buildVersion)
	if current == "" {
		return
	}

	resolver := versionresolver.NewResolver("Yoizen/dev-ai-workflow")
	hasUpdate, latest, err := resolver.CheckForUpdates(current, flags.Channel)
	if err != nil || !hasUpdate {
		return
	}

	fmt.Printf("Update available: %s -> %s\n", current, latest)
	fmt.Println("   Run: ywai --self-update")

	// Auto-update if enabled
	if os.Getenv("YWAI_AUTO_UPDATE") == "true" {
		fmt.Println("   Auto-update enabled, running update...")
		if err := runSelfUpdate(flags); err != nil {
			fmt.Fprintf(os.Stderr, "Auto-update failed: %v\n", err)
		}
	} else {
		fmt.Println("   To enable auto-update, set YWAI_AUTO_UPDATE=true")
	}
	fmt.Println("")
}

func shouldCheckForPeriodicUpdate() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return true // Fallback to always check if we can't determine home dir
	}

	ywaiDir := filepath.Join(homeDir, ".ywai")
	timestampFile := filepath.Join(ywaiDir, "update-check-timestamp")

	// Create ywai dir if it doesn't exist
	if err := os.MkdirAll(ywaiDir, 0755); err != nil {
		return true
	}

	// Read timestamp
	content, err := os.ReadFile(timestampFile)
	if err != nil {
		// File doesn't exist, create it and allow check
		writeTimestamp(timestampFile)
		return true
	}

	// Parse timestamp
	lastCheckStr := strings.TrimSpace(string(content))
	lastCheck, err := time.Parse(time.RFC3339, lastCheckStr)
	if err != nil {
		// Invalid timestamp, recreate and allow check
		writeTimestamp(timestampFile)
		return true
	}

	// Check interval (default 7 days, configurable via env)
	intervalStr := os.Getenv("YWAI_UPDATE_INTERVAL")
	intervalDays := 7
	if intervalStr != "" {
		if d, err := strconv.Atoi(intervalStr); err == nil && d > 0 {
			intervalDays = d
		}
	}

	// Check if enough time has passed
	if time.Since(lastCheck) < time.Duration(intervalDays)*24*time.Hour {
		return false
	}

	// Update timestamp
	writeTimestamp(timestampFile)
	return true
}

func writeTimestamp(timestampFile string) {
	now := time.Now().Format(time.RFC3339)
	os.WriteFile(timestampFile, []byte(now), 0644)
}

func shouldSkipUpdateCheck() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("YWAI_SKIP_UPDATE_CHECK")))
	return v == "1" || v == "true" || v == "yes"
}

func normalizeBuildVersion(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" || v == "dev" {
		return ""
	}
	if idx := strings.Index(v, " "); idx > 0 {
		v = v[:idx]
	}
	if strings.HasPrefix(v, "main") || strings.HasPrefix(v, "master") {
		return v
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return v
}

func formatDisplayVersion(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "vdev"
	}
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}
