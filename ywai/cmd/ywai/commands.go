package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	agentprofiles "github.com/Yoizen/dev-ai-workflow/ywai/internal/agents"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/autostart"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/gentlai"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/kanban"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions/cli"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/overrides"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/skills"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/tokenbank"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/control"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

// daemonize re-executes the current process without the -b flag and exits the parent.
// This detaches the server from the terminal so it keeps running in background.
func daemonize() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot find executable: %w", err)
	}

	// Remove -b/--background from args to avoid infinite recursion
	args := make([]string, 0, len(os.Args)-1)
	for _, a := range os.Args[1:] {
		if a == "-b" || a == "--background" {
			continue
		}
		args = append(args, a)
	}

	child, err := os.StartProcess(exe, append([]string{exe}, args...), &os.ProcAttr{
		Dir:   ".",
		Env:   os.Environ(),
		Files: []*os.File{nil, os.Stdout, os.Stderr},
		Sys:   &syscall.SysProcAttr{Setsid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to fork process: %w", err)
	}

	pidFile := filepath.Join(config.DataDir(), "serve.pid")
	_ = os.MkdirAll(filepath.Dir(pidFile), 0o755)
	_ = os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", child.Pid)), 0o644)

	fmt.Printf("Server started in background (PID %d)\n", child.Pid)
	fmt.Printf("PID file: %s\n", pidFile)
	_ = child.Release()
	os.Exit(0)
	return nil
}

// startOpencodeServe starts the opencode HTTP server in background if not already running.
func startOpencodeServe() {
	url := os.Getenv("OPENCODE_URL")
	if url == "" {
		url = "http://127.0.0.1:4096"
	}
	// Quick check: is it already running?
	resp, err := http.Get(url + "/health")
	if err == nil {
		resp.Body.Close()
		return // already running
	}
	cmd := exec.Command("opencode", "serve", "--port", "4096")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not start opencode serve: %v\n", err)
		return
	}
	fmt.Printf("opencode server starting on %s (PID %d)\n", url, cmd.Process.Pid)
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install gentle-ai + ecosystem + extra skills",
	Long:  "Full setup: gentle-ai, ecosystem, extra skills, and optional project init.",
	Run: func(cmd *cobra.Command, args []string) {
		agentFlag, _ := cmd.Flags().GetString("agent")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		tuiFlag, _ := cmd.Flags().GetBool("tui")
		mcpFlag, _ := cmd.Flags().GetBool("mcp")
		globalFlag, _ := cmd.Flags().GetBool("global")
		autostartFlag, _ := cmd.Flags().GetBool("autostart")

		agents := detectAgents(cmd)
		if agents == nil {
			os.Exit(1)
		}

		var installMCP bool
		var installADO bool
		var globalOnly bool
		var preset, scope, sddMode, persona string
		var groupFilter agentprofiles.GroupFilter
		overwriteAgents := true

		if tuiFlag || (agentFlag == "" && !dryRun && !globalFlag) {
			if !isInteractiveTerminal() {
				fmt.Fprintln(os.Stderr, "Error: install requires --agent or --dry-run when running non-interactively.")
				fmt.Fprintln(os.Stderr, "Run `ywai install --help` for flags, or run `ywai install` from an interactive terminal.")
				os.Exit(1)
			}
			result, err := runTUI(agents)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if result.Agent == "" {
				fmt.Println("Installation cancelled.")
				return
			}
			agentFlag = result.Agent
			installMCP = result.MCP
			installADO = result.ADO
			globalOnly = result.GlobalOnly
			overwriteAgents = result.OverwriteAgents
			preset = result.Preset
			scope = result.Scope
			sddMode = result.SDDMode
			persona = result.Persona
			groupFilter = result.GroupFilter
		} else {
			installMCP = mcpFlag
			globalOnly = globalFlag
			preset = getStringFlag(cmd, "preset")
			scope = getStringFlag(cmd, "scope")
			sddMode = getStringFlag(cmd, "sdd-mode")
			persona = getStringFlag(cmd, "persona")
			groups := getStringSliceFlag(cmd, "group")
			allGroups := getBoolFlag(cmd, "all-groups")
			groupFilter = agentprofiles.GroupFilter{
				Groups:    groups,
				AllGroups: allGroups,
			}
		}

		installOpts := gentlai.InstallOptions{
			AgentName: agentFlag,
			Preset:    preset,
			Scope:     scope,
			SDDMode:   sddMode,
			Persona:   persona,
			DryRun:    dryRun,
		}

		if !tuiFlag {
			installADO = getBoolFlag(cmd, "ado")
		}

		if !tuiFlag {
			fmt.Print("  Overwrite existing agent profiles? [Y/n] ")
			var response string
			fmt.Scanln(&response)
			overwriteAgents = response != "n" && response != "N"
		}

		executeInstall(installOpts, installMCP, globalOnly, installADO, groupFilter, overwriteAgents)

		// Configure autostart if requested
		if autostartFlag && !dryRun {
			fmt.Println("\n[4/4] Configuring autostart...")
			if err := configureAutostart(); err != nil {
				fmt.Printf("  Warning: failed to configure autostart: %v\n", err)
			} else {
				fmt.Println("  Autostart configured successfully")
			}
		}
	},
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Upgrade ywai + gentle-ai + re-seed + sync + copy skills",
	Run: func(cmd *cobra.Command, args []string) {
		var warnings []string
		warn := func(format string, args ...any) {
			msg := fmt.Sprintf(format, args...)
			warnings = append(warnings, msg)
			fmt.Printf("  Warning: %s\n", msg)
		}

		fmt.Println("=== ywai update ===")

		fmt.Println("\n[1/7] Self-updating ywai...")
		selfUpdate()

		fmt.Println("\n[2/7] Upgrading gentle-ai...")
		if !gentlai.IsInstalled() {
			fmt.Println("  gentle-ai not found, installing...")
			if err := gentlai.Install(); err != nil {
				warn("gentle-ai install failed: %v", err)
			}
		} else {
			if err := gentlai.Upgrade(); err != nil {
				warn("gentle-ai upgrade failed: %v", err)
			}
		}

		agents := agent.Resolve()

		fmt.Println("\n[3/7] Re-seeding skills cache...")
		reseedData()

		fmt.Println("\n[4/7] Cleaning stale legacy links + pre-copying extra skills...")
		if len(agents) > 0 {
			for _, a := range agents {
				if configDir := filepath.Dir(a.SkillsDir); skills.IsLinkOrJunction(configDir) {
					warn("[%s] skipped stale legacy-link cleanup because config dir is a symlink/junction: %s", a.Name, configDir)
				} else {
					if removed, err := skills.RemoveStaleYwaiSkillLinks(a.SkillsDir); err != nil {
						warn("[%s] stale legacy-link cleanup failed: %v", a.Name, err)
					} else if len(removed) > 0 {
						fmt.Printf("  [%s] Removed stale legacy skill links: %s\n", a.Name, strings.Join(removed, ", "))
					}
				}
				if err := skills.CopyTo(a.SkillsDir); err != nil {
					warn("[%s] pre-sync skill copy failed: %v", a.Name, err)
				}
			}
		} else {
			fmt.Println("  No supported agents detected; skipping pre-sync skill copy.")
		}

		fmt.Println("\n[5/7] Syncing ecosystem...")
		if gentlai.IsInstalled() {
			syncOpts := gentlai.SyncOptions{
				SDDMode:      getStringFlag(cmd, "sdd-mode"),
				StrictTDD:    getBoolFlag(cmd, "strict-tdd"),
				IncludePerms: getBoolFlag(cmd, "include-permissions"),
				IncludeTheme: getBoolFlag(cmd, "include-theme"),
			}
			if err := gentlai.Sync(syncOpts); err != nil {
				warn("gentle-ai sync failed: %v", err)
				fmt.Println("  Continuing with ywai cache, skill copies, and overrides.")
			}
		} else {
			fmt.Println("  Skipping sync (gentle-ai not installed).")
		}

		fmt.Println("\n[6/7] Copying extra skills...")
		if len(agents) == 0 {
			fmt.Fprintln(os.Stderr, "Error: no supported agents detected.")
			os.Exit(1)
		}

		for _, a := range agents {
			if err := skills.CopyTo(a.SkillsDir); err != nil {
				warn("[%s] re-copy skills failed: %v", a.Name, err)
				continue
			}
			fmt.Printf("  [%s] Copied skills\n", a.Name)
		}

		fmt.Println("\n[6/7] Re-applying ywai overrides...")
		agentDirs := make(map[string]string, len(agents))
		for _, a := range agents {
			agentDirs[a.Name] = a.SkillsDir
		}
		if err := overrides.ApplyOpenSpecToSDDOverride(agentDirs); err != nil {
			warn("failed to apply overrides: %v", err)
		}

		if len(warnings) > 0 {
			fmt.Println("\n=== Done with warnings ===")
			for _, msg := range warnings {
				fmt.Printf("  - %s\n", msg)
			}
			return
		}

		fmt.Println("\n=== Done! ===")
	},
}

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "List detected AI agents",
	Run: func(cmd *cobra.Command, args []string) {
		detected := agent.Resolve()
		if len(detected) == 0 {
			fmt.Println("No agents detected.")
			fmt.Println("\nSupported agents:")
			for _, name := range agent.AvailableNames() {
				fmt.Printf("  - %s\n", name)
			}
			return
		}
		fmt.Printf("Detected %d agent(s):\n", len(detected))
		for _, a := range detected {
			fmt.Printf("  - %s\n", a.Name)
			if a.BinaryName != "" {
				fmt.Printf("    binary: %s\n", a.BinaryName)
			}
			fmt.Printf("    skills: %s\n", a.SkillsDir)
		}
	},
}

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "List available extra skills",
	Run: func(cmd *cobra.Command, args []string) {
		available, err := skills.ListAvailable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Extra skills available:")
		for _, s := range available {
			fmt.Printf("  - %s\n", s)
		}

		fmt.Printf("\nTotal: %d skills\n", len(available))
	},
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run gentle-ai health check",
	Long:  "Read-only ecosystem health diagnostics — tool binaries, state.json, Engram, disk space.",
	Run: func(cmd *cobra.Command, args []string) {
		if err := gentlai.Doctor(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var skillRegistryCmd = &cobra.Command{
	Use:   "skill-registry",
	Short: "Refresh the project skill registry",
	Long:  "Scan project and global skills, build the registry used by SDD orchestrators.",
	Run: func(cmd *cobra.Command, args []string) {
		cwd, _ := cmd.Flags().GetString("cwd")
		if cwd == "" {
			cwd, _ = os.Getwd()
		}
		if err := gentlai.SkillRegistryRefresh(cwd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage ywai configuration",
	Long:  "View or edit ywai configuration stored in ~/.ywai/config.yaml",
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a configuration value",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(args) == 0 {
			// Show all config
			data, err := yaml.Marshal(cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(data))
			return
		}

		key := args[0]
		var value interface{}
		switch key {
		case "default_preset":
			value = cfg.DefaultPreset
		case "default_sdd_mode":
			value = cfg.DefaultSDDMode
		case "default_persona":
			value = cfg.DefaultPersona
		case "default_scope":
			value = cfg.DefaultScope
		case "default_tui":
			value = cfg.DefaultTUI
		case "default_mcp":
			value = cfg.DefaultMCP
		case "colored_output":
			value = cfg.ColoredOutput
		case "log_level":
			value = cfg.LogLevel
		case "server.port":
			value = cfg.Server.Port
		case "server.background":
			value = cfg.Server.Background
		case "server.mcp":
			value = cfg.Server.MCP
		case "server.autostart":
			value = cfg.Server.Autostart
		default:
			fmt.Fprintf(os.Stderr, "Error: unknown key %q\n", key)
			os.Exit(1)
		}

		fmt.Printf("%v\n", value)
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		key := args[0]
		value := args[1]

		switch key {
		case "default_preset":
			cfg.DefaultPreset = value
		case "default_sdd_mode":
			cfg.DefaultSDDMode = value
		case "default_persona":
			cfg.DefaultPersona = value
		case "default_scope":
			cfg.DefaultScope = value
		case "default_tui":
			if value == "true" {
				cfg.DefaultTUI = true
			} else if value == "false" {
				cfg.DefaultTUI = false
			} else {
				fmt.Fprintf(os.Stderr, "Error: value must be true or false\n")
				os.Exit(1)
			}
		case "default_mcp":
			if value == "true" {
				cfg.DefaultMCP = true
			} else if value == "false" {
				cfg.DefaultMCP = false
			} else {
				fmt.Fprintf(os.Stderr, "Error: value must be true or false\n")
				os.Exit(1)
			}
		case "colored_output":
			if value == "true" {
				b := true
				cfg.ColoredOutput = &b
			} else if value == "false" {
				b := false
				cfg.ColoredOutput = &b
			} else {
				fmt.Fprintf(os.Stderr, "Error: value must be true or false\n")
				os.Exit(1)
			}
		case "log_level":
			cfg.LogLevel = value
		case "agents":
			// Parse comma-separated agent names
			var agents []string
			for _, a := range strings.Split(value, ",") {
				trimmed := strings.TrimSpace(a)
				if trimmed != "" {
					agents = append(agents, trimmed)
				}
			}
			cfg.Agents = agents
		case "server.port":
			var port int
			if _, err := fmt.Sscanf(value, "%d", &port); err != nil {
				fmt.Fprintf(os.Stderr, "Error: port must be a number\n")
				os.Exit(1)
			}
			cfg.Server.Port = port
		case "server.background":
			if value == "true" {
				cfg.Server.Background = true
			} else if value == "false" {
				cfg.Server.Background = false
			} else {
				fmt.Fprintf(os.Stderr, "Error: value must be true or false\n")
				os.Exit(1)
			}
		case "server.mcp":
			if value == "true" {
				cfg.Server.MCP = true
			} else if value == "false" {
				cfg.Server.MCP = false
			} else {
				fmt.Fprintf(os.Stderr, "Error: value must be true or false\n")
				os.Exit(1)
			}
		case "server.autostart":
			if value == "true" {
				cfg.Server.Autostart = true
			} else if value == "false" {
				cfg.Server.Autostart = false
			} else {
				fmt.Fprintf(os.Stderr, "Error: value must be true or false\n")
				os.Exit(1)
			}
		default:
			fmt.Fprintf(os.Stderr, "Error: unknown key %q\n", key)
			os.Exit(1)
		}

		if err := config.SaveConfig(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Set %s = %s\n", key, value)
	},
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.DefaultConfig()
		if err := config.SaveConfig(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Configuration reset to defaults")
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show ywai installation status",
	Long:  "Display information about ywai, gentle-ai, and detected agents",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("=== ywai Status ===")

		// ywai version
		fmt.Printf("\nVersion: %s\n", version)

		// Config location
		fmt.Printf("Config: %s\n", config.ConfigPath())

		// Data directory
		fmt.Printf("Data dir: %s\n", config.DataDir())

		// gentle-ai status
		fmt.Println("\n=== gentle-ai ===")
		if gentlai.IsInstalled() {
			fmt.Println("Status: Installed")
		} else {
			fmt.Println("Status: Not installed")
		}

		// Detected agents
		fmt.Println("\n=== Detected Agents ===")
		agents := agent.Resolve()
		if len(agents) == 0 {
			fmt.Println("No agents detected")
		} else {
			for _, a := range agents {
				fmt.Printf("  - %s\n", a.Name)
				if a.BinaryName != "" {
					fmt.Printf("    Binary: %s\n", a.BinaryName)
				}
				fmt.Printf("    Skills: %s\n", a.SkillsDir)
			}
		}

		// Available skills
		fmt.Println("\n=== Available Skills ===")
		skills := config.AvailableSkills()
		if len(skills) == 0 {
			fmt.Println("No skills available (run ywai install)")
		} else {
			for _, s := range skills {
				fmt.Printf("  - %s\n", s)
			}
		}

		// User config
		fmt.Println("\n=== User Configuration ===")
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
		} else {
			fmt.Printf("Default preset: %s\n", cfg.DefaultPreset)
			fmt.Printf("Default SDD mode: %s\n", cfg.DefaultSDDMode)
			fmt.Printf("Default persona: %s\n", cfg.DefaultPersona)
		}
	},
}

var groupsCmd = &cobra.Command{
	Use:   "groups",
	Short: "List available agent groups",
	Long:  "List available agent groups from groups.json. Core group is always installed.",
	Run: func(cmd *cobra.Command, args []string) {
		if !config.IsDirPopulated(config.DataAgentsDir()) {
			if err := config.SeedAgentsFromEmbedded(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: no agent data available: %v\n", err)
				os.Exit(1)
			}
		}
		names, err := agentprofiles.ListGroups(config.DataAgentsDir())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if len(names) == 0 {
			fmt.Println("No groups found.")
			return
		}
		for _, name := range names {
			fmt.Println(name)
		}
	},
}

// daemonCmd starts the Kanban UI server (deprecated: use 'ywai serve' instead).
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start the Kanban UI server (deprecated: use 'ywai serve' instead)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stderr, "Warning: 'ywai daemon' is deprecated. Use 'ywai serve' instead.")
		mcpMode, _ := cmd.Flags().GetBool("mcp")
		if mcpMode {
			// Run as MCP adapter (stdio JSON-RPC)
			adapter := kanban.NewMCPAdapter()
			adapter.Run()
			return nil
		}

	// Normal HTTP server mode with auto-start and port resolution
	port, _ := cmd.Flags().GetInt("port")
	_, err := kanban.GetOrStart(port)
	if err != nil {
		return err
	}
	// Block forever (server runs in background from GetOrStart)
	select {}
	},
}

// serveCmd starts the control ywai server (Kanban + Missions).
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the control ywai server (Kanban + Missions)",
	Long:  "Start the control ywai server combining Kanban and Missions on a single port.",
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		background, _ := cmd.Flags().GetBool("background")
		noMCP, _ := cmd.Flags().GetBool("no-mcp")
		mcpOnly, _ := cmd.Flags().GetBool("mcp-only")

		// Fork to background before doing any work
		if background {
			if err := daemonize(); err != nil {
				return err
			}
		}

		if mcpOnly {
			adapter := kanban.NewMCPAdapter()
			adapter.Run()
			return nil
		}

		// Start control server
		s, err := control.GetOrStart(port)
		if err != nil {
			return fmt.Errorf("failed to start control server: %w", err)
		}

		// Auto-start opencode serve if not already running
		startOpencodeServe()

		// Start MCP adapter in background if not disabled
		if !noMCP {
			go func() {
				adapter := kanban.NewMCPAdapter()
				adapter.Run()
			}()
		}

		fmt.Printf("Server running on port %d\n", s.Port())
		fmt.Printf("Control UI: http://localhost:%d/\n", s.Port())
		fmt.Printf("Health check: http://localhost:%d/health\n", s.Port())
		fmt.Printf("Kanban UI: http://localhost:%d/\n", s.Port())
		fmt.Printf("Missions UI: http://localhost:%d/missions/\n", s.Port())

		// Block forever
		select {}
	},
}

// uiCmd opens the Kanban UI in the default browser.
var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Open Kanban UI in browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		url := fmt.Sprintf("http://localhost:%d", port)

		// Try to open browser
		browserCmd := exec.Command("xdg-open", url)
		if err := browserCmd.Start(); err != nil {
			fmt.Printf("Open %s in your browser\n", url)
			return nil
		}
		fmt.Printf("Opening %s ...\n", url)
		return nil
	},
}

// ---------------------------------------------------------------------------
// TokenBank commands
// ---------------------------------------------------------------------------

var tokenbankCmd = &cobra.Command{
	Use:   "tokenbank",
	Short: "Configure agents to use TokenBank proxy",
}

var tokenbankSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Save TokenBank URL and API key to ywai config",
	Run: func(cmd *cobra.Command, args []string) {
		url := getStringFlag(cmd, "url")
		key := getStringFlag(cmd, "key")

		if url == "" {
			fmt.Fprintln(os.Stderr, "Error: --url is required")
			os.Exit(1)
		}
		if key == "" {
			fmt.Fprintln(os.Stderr, "Error: --key is required")
			os.Exit(1)
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		cfg.TokenBankURL = url
		cfg.TokenBankAPIKey = key

		if err := config.SaveConfig(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("TokenBank configuration saved.")
		fmt.Printf("  URL: %s\n", url)
		fmt.Printf("  Key: %s***\n", maskKey(key))

		// Verify connection
		fmt.Println("\nVerifying connection...")
		models, err := tokenbank.FetchModels(url, key)
		if err != nil {
			fmt.Printf("  ⚠ Warning: could not connect to TokenBank: %v\n", err)
			fmt.Println("  Config saved anyway. Run 'ywai tokenbank configure' when the server is available.")
			return
		}
		fmt.Printf("  ✓ Connected! %d models available (default: %s)\n", len(models.Models), models.DefaultModel)
	},
}

var tokenbankConfigureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure agents to use TokenBank proxy",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		if cfg.TokenBankURL == "" || cfg.TokenBankAPIKey == "" {
			fmt.Fprintln(os.Stderr, "Error: TokenBank not configured. Run 'ywai tokenbank setup --url <url> --key <key>' first.")
			os.Exit(1)
		}

		agentFlag := getStringFlag(cmd, "agent")
		fmt.Println("=== TokenBank Configure ===")
		fmt.Printf("  Server: %s\n\n", cfg.TokenBankURL)

		if agentFlag != "" {
			// Configure a single agent
			switch agentFlag {
			case "opencode":
				if err := tokenbank.ConfigureOpenCode(cfg.TokenBankURL, cfg.TokenBankAPIKey); err != nil {
					fmt.Fprintf(os.Stderr, "Error configuring opencode: %v\n", err)
					os.Exit(1)
				}
			case "pi":
				if err := tokenbank.ConfigurePi(cfg.TokenBankURL, cfg.TokenBankAPIKey); err != nil {
					fmt.Fprintf(os.Stderr, "Error configuring pi: %v\n", err)
					os.Exit(1)
				}
			case "copilot":
				if err := tokenbank.ConfigureCopilot(cfg.TokenBankURL, cfg.TokenBankAPIKey); err != nil {
					fmt.Fprintf(os.Stderr, "Error configuring copilot: %v\n", err)
					os.Exit(1)
				}
			default:
				fmt.Fprintf(os.Stderr, "Error: unknown agent %q. Use: opencode, pi, copilot\n", agentFlag)
				os.Exit(1)
			}
		} else {
			// Configure all agents
			errors := tokenbank.ConfigureAll(cfg.TokenBankURL, cfg.TokenBankAPIKey)
			if len(errors) > 0 {
				fmt.Fprintln(os.Stderr, "\nSome agents failed:")
				for _, e := range errors {
					fmt.Fprintf(os.Stderr, "  ✗ %v\n", e)
				}
				os.Exit(1)
			}
		}

		fmt.Println("\nDone! Restart your agents to pick up the new configuration.")
	},
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + strings.Repeat("*", len(key)-4)
}

// configureAutostart sets up the control server to start automatically on system boot.
func configureAutostart() error {
	if err := autostart.Configure(); err != nil {
		return fmt.Errorf("failed to configure autostart: %w", err)
	}
	return nil
}

func init() {
	cli.RegisterCommands(rootCmd)

	daemonCmd.Flags().Bool("mcp", false, "Run as MCP stdio adapter")
	daemonCmd.Flags().IntP("port", "p", kanban.DefaultUIPort, "Port for Kanban UI server")

	serveCmd.Flags().IntP("port", "p", 5768, "Port for control server")
	serveCmd.Flags().BoolP("background", "b", false, "Run in background (detach from terminal)")
	serveCmd.Flags().Bool("no-mcp", false, "Don't start MCP adapter")
	serveCmd.Flags().Bool("mcp-only", false, "Run as MCP adapter only (stdio, no HTTP)")

	uiCmd.Flags().IntP("port", "p", kanban.DefaultUIPort, "Port for Kanban UI server")

	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(uiCmd)

	installCmd.Flags().StringP("agent", "a", "", "Specific agent to install for")
	installCmd.Flags().Bool("dry-run", false, "Preview changes without applying")
	installCmd.Flags().Bool("tui", false, "Force TUI mode")
	installCmd.Flags().Bool("mcp", false, "Install Microsoft Learn MCP (for opencode)")
	installCmd.Flags().Bool("global", false, "Install global skills only (skip AGENTS.md/REVIEW.md in project)")
	installCmd.Flags().String("preset", "full-gentleman", "Install preset: full-gentleman, ecosystem-only, minimal, custom")
	installCmd.Flags().String("scope", "", "Install scope: global (default) or workspace")
	installCmd.Flags().String("sdd-mode", "", "SDD orchestrator mode: single or multi")
	installCmd.Flags().String("persona", "", "Persona: gentleman, neutral, custom")
	installCmd.Flags().Bool("autostart", false, "Configure control server to start automatically on system boot")
	installCmd.Flags().Bool("ado", false, "Install Azure DevOps plugin (opencode + pi)")
	installCmd.Flags().StringSlice("group", []string{}, "Agent groups to install (repeatable, e.g., --group social-refactor)")
	installCmd.Flags().Bool("all-groups", false, "Install all agent groups")

	updateCmd.Flags().String("sdd-mode", "", "SDD orchestrator mode: single or multi")
	updateCmd.Flags().Bool("strict-tdd", false, "Enable Strict TDD Mode for SDD agents")
	updateCmd.Flags().Bool("include-permissions", false, "Include permissions in sync")
	updateCmd.Flags().Bool("include-theme", false, "Include theme in sync")

	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(agentsCmd)
	rootCmd.AddCommand(skillsCmd)
	rootCmd.AddCommand(doctorCmd)

	skillRegistryCmd.Flags().String("cwd", "", "Project directory (defaults to current)")
	rootCmd.AddCommand(skillRegistryCmd)

	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configResetCmd)
	rootCmd.AddCommand(configCmd)

	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(groupsCmd)

	// TokenBank commands
	tokenbankSetupCmd.Flags().String("url", "", "TokenBank instance URL (e.g. https://tokenbank.example.com)")
	tokenbankSetupCmd.Flags().String("key", "", "TokenBank proxy API key")
	tokenbankCmd.AddCommand(tokenbankSetupCmd)

	tokenbankConfigureCmd.Flags().String("agent", "", "Agent to configure: opencode, copilot, pi (default: all)")
	tokenbankCmd.AddCommand(tokenbankConfigureCmd)

	rootCmd.AddCommand(tokenbankCmd)
}

func isInteractiveTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func getStringFlag(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}

func getBoolFlag(cmd *cobra.Command, name string) bool {
	v, _ := cmd.Flags().GetBool(name)
	return v
}

func getStringSliceFlag(cmd *cobra.Command, name string) []string {
	v, _ := cmd.Flags().GetStringSlice(name)
	return v
}
