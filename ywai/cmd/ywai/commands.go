package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	agentprofiles "github.com/Yoizen/dev-ai-workflow/ywai/internal/agents"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/autostart"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/control"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/fastfs"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/gentlai"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/kanban"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/mcp"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions/cli"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/plugins" // CodegraphInfo, install helpers
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/selfupdate"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/serverutil"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/skills"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/tokenbank"
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
		Sys:   sysProcAttr(),
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

// killPort kills any process listening on the given TCP port.
// Uses lsof (macOS/Linux), fuser (Linux), or netsh (Windows) to find the PID, then kills it.
func killPort(port int) error {
	portStr := fmt.Sprintf(":%d", port)

	// Try lsof -ti :<port> first (macOS + Linux)
	cmd := exec.Command("lsof", "-ti", portStr)
	out, err := cmd.Output()
	if err == nil {
		// lsof prints one PID per line when several processes hold the port
		// (e.g. parent + child, or a leftover plus the new server). parsePIDs
		// splits on any whitespace so we kill each one instead of handing the
		// whole blob to Atoi and failing with invalid PID "1040\n19768".
		if pids := parsePIDs(string(out)); len(pids) > 0 {
			return killPIDs(pids)
		}
	}

	// Fallback: fuser <port>/tcp (Linux)
	cmd = exec.Command("fuser", fmt.Sprintf("%d/tcp", port))
	out, err = cmd.Output()
	if err == nil {
		if pids := parsePIDs(string(out)); len(pids) > 0 {
			return killPIDs(pids)
		}
	}

	// Fallback: Windows netstat -ano (findstr filters to lines with the port)
	cmd = exec.Command("cmd", "/C",
		fmt.Sprintf("netstat -ano | findstr :%d", port))
	out, err = cmd.Output()
	if err == nil {
		// netstat -ano output: "  TCP    0.0.0.0:5768    0.0.0.0:0    LISTENING    1234"
		// The PID is the last whitespace-separated field on each line.
		for _, line := range strings.Split(string(out), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				if pids := parsePIDs(fields[len(fields)-1]); len(pids) > 0 {
					return killPIDs(pids)
				}
			}
		}
	}

	return fmt.Errorf("port %d is in use and no process could be killed", port)
}

// parsePIDs extracts every numeric PID from raw output, splitting on any
// whitespace or newline. It skips non-numeric tokens, so it tolerates both
// `lsof -ti` (one PID per line) and mixed text. Returns nil if no PID is found.
func parsePIDs(raw string) []int {
	var pids []int
	for _, tok := range strings.Fields(raw) {
		pid, err := strconv.Atoi(tok)
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}
	return pids
}

// killPIDs sends a termination signal to every PID and returns the last error
// encountered (nil if all succeeded). A process that already exited reports
// "process already finished", which we treat as success — the goal is that
// nothing is left listening on the port.
func killPIDs(pids []int) error {
	var lastErr error
	for _, pid := range pids {
		if e := killPIDInt(pid); e != nil {
			lastErr = e
		}
	}
	return lastErr
}

// readStopPIDFile reads the first valid PID from a PID file. Accepts a file
// with multiple PIDs (returns the first) but fails when no numeric PID is
// present at all, so garbage is never mistaken for PID 0.
func readStopPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pids := parsePIDs(string(data))
	if len(pids) == 0 {
		return 0, fmt.Errorf("invalid PID in %s: no numeric PID found", path)
	}
	return pids[0], nil
}

// acquirePort binds the given TCP port, killing any process that still holds it
// and retrying the bind with backoff until the OS releases the socket.
//
// `ywai serve` used to retry net.Listen exactly once after killPort. That raced
// against the old process releasing the socket (SIGTERM is delivered, but the
// TCP listener isn't gone yet), so the immediate retry failed with
// "still in use after cleanup" even though killPort had succeeded. We now poll
// the bind for a short window after killing so a slow shutdown doesn't abort
// the restart.
func acquirePort(port int) (net.Listener, error) {
	addr := fmt.Sprintf(":%d", port)

	// Fast path: the port is already free.
	if ln, err := net.Listen("tcp", addr); err == nil {
		return ln, nil
	}

	// Port is busy — try to free it, then poll the bind.
	fmt.Fprintf(os.Stderr, "Port %d is in use, attempting to free it...\n", port)
	if err := killPort(port); err != nil {
		return nil, fmt.Errorf("port %d is already in use and could not be freed: %w", port, err)
	}

	// Poll for the socket to be released. The OS keeps a listener in CLOSE_WAIT
	// / TIME_WAIT briefly after the owning process exits; a few hundred ms is
	// plenty in practice while still failing fast on a truly stuck port.
	const (
		totalWait = 2 * time.Second
		step      = 50 * time.Millisecond
	)
	deadline := time.Now().Add(totalWait)
	var lastErr error
	for {
		if ln, err := net.Listen("tcp", addr); err == nil {
			return ln, nil
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(step)
	}
	return nil, fmt.Errorf("port %d is still in use after cleanup: %w", port, lastErr)
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the background control server",
	Long:  "Reads the PID file and sends SIGTERM for graceful shutdown.",
	RunE: func(cmd *cobra.Command, args []string) error {
		pidFile := filepath.Join(config.DataDir(), "serve.pid")
		pid, err := readStopPIDFile(pidFile)
		if err != nil {
			if os.IsNotExist(err) {
				// No PID file — try killing by port
				port, _ := cmd.Flags().GetInt("port")
				if port == 0 {
					port = 5768
				}
				if serverutil.CheckHealth(port) {
					if err := killPort(port); err != nil {
						return fmt.Errorf("could not stop server on port %d: %w", port, err)
					}
					fmt.Printf("Server on port %d stopped.\n", port)
					return nil
				}
				return fmt.Errorf("no running server found (no PID file, port %d not responding)", port)
			}
			return fmt.Errorf("cannot read PID file: %w", err)
		}

		// Always remove the PID file: whether the signal landed or the process
		// had already exited, there is nothing left for a future `stop` to signal.
		defer func() { _ = os.Remove(pidFile) }()

		// killPIDInt treats "process already finished" as success, so a stale
		// PID file (server crashed/was killed out of band) cleans up instead of
		// erroring with "could not send SIGTERM".
		if err := killPIDInt(pid); err != nil {
			return fmt.Errorf("could not stop server (PID %d): %w", pid, err)
		}

		fmt.Printf("Server (PID %d) stopped.\n", pid)
		return nil
	},
}

// startOpencodeServe starts the opencode HTTP server in background if an
// opencode server is not already reachable at the configured URL.
//
// It resolves the opencode binary via agent.FindBinary (so binaries installed
// via nvm/asdf/etc. — not in the raw process PATH — are still found), and
// probes with opencode.ProbeServer (GET /status requiring 200) instead of a
// bare /health ping, so another server (e.g. Kilo Code) on the same port does
// not produce a false "already running" positive.
//
// If the default port (4096, or the one in OPENCODE_URL) is already taken by a
// non-opencode process, it walks up to find a free port and starts opencode
// there, exporting the chosen URL via OPENCODE_URL so the rest of ywai
// (chat proxy, missions) all point at the same instance.
func startOpencodeServe() {
	url := os.Getenv("OPENCODE_URL")
	if url == "" {
		url = "http://127.0.0.1:4096"
	}

	// Probe: is an opencode server already running at the URL?
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if ok, _ := opencode.ProbeServer(ctx, url); ok {
		return // a real opencode server is already reachable
	}

	// Resolve the opencode binary (PATH, well-known dirs, login-shell which).
	binPath := agent.FindBinary("opencode")
	if binPath == "" {
		fmt.Fprintln(os.Stderr, "Warning: opencode binary not found (looked in PATH, "+
			"~/.opencode/bin, ~/.local/bin, and login-shell which). "+
			"Install opencode or set OPENCODE_URL to point at a running server.")
		return
	}

	// Determine the starting port from the URL (default 4096).
	startPort := 4096
	if _, p, err := net.SplitHostPort(strings.TrimPrefix(strings.TrimPrefix(url, "http://"), "https://")); err == nil && p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			startPort = n
		}
	}

	// Find a free port. If the default (startPort) is open, use it; otherwise
	// walk up to 50 ports looking for one that binds. This handles the case
	// where another server (Kilo Code, etc.) occupies the default opencode port.
	port := serverutil.FindFreePort(startPort)
	if port != startPort {
		fmt.Fprintf(os.Stderr, "Warning: port %d is busy; using %d instead.\n", startPort, port)
	}

	chosenURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	cmd := exec.Command(binPath, "serve", "--port", strconv.Itoa(port))
	cmd.SysProcAttr = sysProcAttr()
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not start opencode serve (%s): %v\n", binPath, err)
		return
	}
	// Export the chosen URL so detectOpenCodeURL (and every other consumer of
	// OPENCODE_URL in this process) proxies to the instance we just started.
	os.Setenv("OPENCODE_URL", chosenURL)
	fmt.Printf("opencode server starting on %s (PID %d)\n", chosenURL, cmd.Process.Pid)

	// Wait briefly for opencode to bind its port, so the control server's chat
	// route registration (which runs right after) sees it. Poll /status up to
	// ~5s; opencode usually binds in under a second.
	for i := 0; i < 25; i++ {
		pctx, pcancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		ok, _ := opencode.ProbeServer(pctx, chosenURL)
		pcancel()
		if ok {
			fmt.Printf("opencode server ready on %s\n", chosenURL)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Fprintf(os.Stderr, "Warning: opencode was started but did not become reachable on %s within 5s — chat may need a moment.\n", chosenURL)
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
		var globalOnly bool
		var preset, scope string
		var groupFilter agentprofiles.GroupFilter
		overwriteAgents := true
		ranTUI := false
		installSDD := false
		sddMode := ""

		if shouldRunInstallTUI(tuiFlag, agentFlag, dryRun, cmd.Flags().Changed("global")) {
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
			// "all" means install for every detected agent -- clear the flag so
			// executeInstall resolves all agents instead of looking up "all".
			if result.Agent == "all" {
				agentFlag = ""
			} else {
				agentFlag = result.Agent
			}
			installMCP = result.MCP
			globalOnly = result.GlobalOnly
			overwriteAgents = result.OverwriteAgents
			preset = result.Preset
			scope = result.Scope
			groupFilter = result.GroupFilter
			autostartFlag = result.Autostart
			installSDD = result.InstallSDD
			sddMode = result.SDDMode
			ranTUI = true
		} else {
			installMCP = mcpFlag
			globalOnly = globalFlag
			preset = getStringFlag(cmd, "preset")
			scope = getStringFlag(cmd, "scope")
			groups := getStringSliceFlag(cmd, "group")
			allGroups := getBoolFlag(cmd, "all-groups")
			groupFilter = agentprofiles.GroupFilter{
				Groups:    groups,
				AllGroups: allGroups,
			}
			// CLI: --sdd-mode single|multi enables optional SDD. Persona is never installed.
			sddMode = strings.ToLower(strings.TrimSpace(getStringFlag(cmd, "sdd-mode")))
			if sddMode == "single" || sddMode == "multi" {
				installSDD = true
			} else if sddMode != "" {
				fmt.Fprintf(os.Stderr, "Error: --sdd-mode must be single or multi (got %q)\n", sddMode)
				os.Exit(1)
			}
		}

		installOpts := gentlai.InstallOptions{
			AgentName:  agentFlag,
			Preset:     preset,
			Scope:      scope,
			DryRun:     dryRun,
			InstallSDD: installSDD,
			SDDMode:    sddMode,
		}

		// Only prompt for overwrite when the TUI did not collect it.
		if !ranTUI {
			fmt.Print("  Overwrite existing agent profiles? [Y/n] ")
			var response string
			_, _ = fmt.Scanln(&response)
			overwriteAgents = response != "n" && response != "N"
		}

		result := executeInstall(installOpts, installMCP, globalOnly, groupFilter, overwriteAgents, autostartFlag)
		result.printFooter(applyInstall)
		if code := result.exitCode(); code != 0 {
			os.Exit(code)
		}
	},
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "One-command upgrade: binary, skills, profiles, plugins, control server",
	Long: `Upgrade ywai and re-apply everything it manages so you usually do not
need a separate install after updating:

  - self-update ywai
  - upgrade gentle-ai
  - re-seed skills + agent profiles
  - re-install gentle-ai ecosystem components
  - copy ywai extra skills into detected agents
  - re-install agent profiles + curated AGENTS.md
  - re-wire OpenCode plugins (vision-bridge, background-agents, kanban MCP)
  - upgrade companion CLIs (ado, codegraph) via the plugins step
  - restart the control server only if it was already running

Channels:
  default          stable only (GitHub /releases/latest — ignores prereleases)
  --beta           newest prerelease (e.g. vX.Y.Z-beta.N)

Flags:
  --agent          limit re-apply to one agent
  --dry-run        preview without writing

After update, restart OpenCode once so it reloads plugins.`,
	Run: func(cmd *cobra.Command, args []string) {
		beta, _ := cmd.Flags().GetBool("beta")
		agentFlag, _ := cmd.Flags().GetString("agent")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		if beta {
			fmt.Println("=== ywai update (beta channel) ===")
		} else {
			fmt.Println("=== ywai update ===")
		}
		fmt.Println("  (binary + full managed re-apply — one command is enough)")

		fmt.Println("\n[pre] Self-updating ywai...")
		if dryRun {
			fmt.Println("  Would self-update ywai binary.")
		} else {
			selfUpdate(beta)
		}

		// gentle-ai binary is upgraded here; applyManaged skips a second pass
		// via SkipGentleAIBinary and still runs ecosystem/profiles/plugins.
		fmt.Println("\n[pre] Upgrading gentle-ai binary...")
		if dryRun {
			fmt.Println("  Would install or upgrade gentle-ai.")
		} else if !gentlai.IsInstalled() {
			fmt.Println("  gentle-ai not found, installing...")
			if err := gentlai.Install(); err != nil {
				fmt.Printf("  Warning: gentle-ai install failed: %v\n", err)
			}
		} else if err := gentlai.Upgrade(); err != nil {
			fmt.Printf("  Warning: gentle-ai upgrade failed: %v\n", err)
		}

		result := applyManaged(applyOpts{
			Mode: applyUpdate,
			Opts: gentlai.InstallOptions{
				AgentName: agentFlag,
				Preset:    "full-gentleman",
				DryRun:    dryRun,
			},
			GlobalOnly:            true,
			OverwriteAgents:       true,
			SkipGentleAIBinary:    true,
			RestartServeIfRunning: true,
		})
		// Surface binary-phase soft failures into the summary when we only printed them.
		result.printFooter(applyUpdate)
		if code := result.exitCode(); code != 0 {
			os.Exit(code)
		}
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
	RunE: func(cmd *cobra.Command, args []string) error {
		available, err := skills.ListAvailable()
		if err != nil {
			return err
		}

		fmt.Println("Extra skills available:")
		for _, s := range available {
			fmt.Printf("  - %s\n", s)
		}

		fmt.Printf("\nTotal: %d skills\n", len(available))
		return nil
	},
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run gentle-ai health check",
	Long:  "Read-only ecosystem health diagnostics — tool binaries, state.json, Engram, disk space, CodeGraph, ywai-fastfs.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := gentlai.Doctor(); err != nil {
			return err
		}
		printFastPathDoctor()
		return nil
	},
}

// printFastPathDoctor reports CodeGraph + ywai-fastfs readiness (Layer A/B).
func printFastPathDoctor() {
	fmt.Println("\n── ywai fast path ──")
	if v, ok := plugins.CodegraphInfo(); ok {
		if v != "" {
			fmt.Printf("  codegraph: installed (%s)\n", v)
		} else {
			fmt.Println("  codegraph: installed (version unknown)")
		}
	} else {
		fmt.Println("  codegraph: NOT on PATH (run ywai install / install codegraph)")
	}
	if _, err := exec.LookPath("ywai"); err != nil {
		// Still ok when running from a built binary not named on PATH.
		fmt.Println("  ywai-fastfs: use `ywai mcp fastfs` from this binary")
	} else {
		fmt.Println("  ywai-fastfs: `ywai mcp fastfs` available (register via ywai install)")
	}
	// Smoke: can construct a service for cwd.
	if svc, err := fastfs.NewService(""); err != nil {
		fmt.Printf("  fastfs service: ERROR %v\n", err)
	} else {
		fmt.Printf("  fastfs workspace: %s\n", svc.RootAbs())
	}
}

var skillRegistryCmd = &cobra.Command{
	Use:   "skill-registry",
	Short: "Refresh the project skill registry",
	Long:  "Scan project and global skills, build the registry used by SDD orchestrators.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, _ := cmd.Flags().GetString("cwd")
		if cwd == "" {
			cwd, _ = os.Getwd()
		}
		return gentlai.SkillRegistryRefresh(cwd)
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage ywai configuration",
	Long:  "View or edit ywai configuration stored in ~/.ywai/config.yaml",
}

// configField defines a gettable/settable config key.
type configField struct {
	Get func(*config.UserConfig) interface{}
	Set func(*config.UserConfig, string) error
}

var configFields = map[string]configField{
	"default_preset": {
		Get: func(c *config.UserConfig) interface{} { return c.DefaultPreset },
		Set: func(c *config.UserConfig, v string) error { c.DefaultPreset = v; return nil },
	},
	"default_scope": {
		Get: func(c *config.UserConfig) interface{} { return c.DefaultScope },
		Set: func(c *config.UserConfig, v string) error { c.DefaultScope = v; return nil },
	},
	"default_tui": {
		Get: func(c *config.UserConfig) interface{} { return c.DefaultTUI },
		Set: func(c *config.UserConfig, v string) error {
			b, err := parseBool(v)
			if err != nil {
				return err
			}
			c.DefaultTUI = b
			return nil
		},
	},
	"default_mcp": {
		Get: func(c *config.UserConfig) interface{} { return c.DefaultMCP },
		Set: func(c *config.UserConfig, v string) error {
			b, err := parseBool(v)
			if err != nil {
				return err
			}
			c.DefaultMCP = b
			return nil
		},
	},
	"colored_output": {
		Get: func(c *config.UserConfig) interface{} {
			if c.ColoredOutput != nil {
				return *c.ColoredOutput
			}
			return nil
		},
		Set: func(c *config.UserConfig, v string) error {
			b, err := parseBool(v)
			if err != nil {
				return err
			}
			c.ColoredOutput = &b
			return nil
		},
	},
	"log_level": {
		Get: func(c *config.UserConfig) interface{} { return c.LogLevel },
		Set: func(c *config.UserConfig, v string) error { c.LogLevel = v; return nil },
	},
	"agents": {
		Get: func(c *config.UserConfig) interface{} { return c.Agents },
		Set: func(c *config.UserConfig, v string) error {
			var agents []string
			for _, a := range strings.Split(v, ",") {
				if trimmed := strings.TrimSpace(a); trimmed != "" {
					agents = append(agents, trimmed)
				}
			}
			c.Agents = agents
			return nil
		},
	},
	"server.port": {
		Get: func(c *config.UserConfig) interface{} { return c.Server.Port },
		Set: func(c *config.UserConfig, v string) error {
			port, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("port must be a number")
			}
			c.Server.Port = port
			return nil
		},
	},
	"server.background": {
		Get: func(c *config.UserConfig) interface{} { return c.Server.Background },
		Set: func(c *config.UserConfig, v string) error {
			b, err := parseBool(v)
			if err != nil {
				return err
			}
			c.Server.Background = b
			return nil
		},
	},
	"server.mcp": {
		Get: func(c *config.UserConfig) interface{} { return c.Server.MCP },
		Set: func(c *config.UserConfig, v string) error {
			b, err := parseBool(v)
			if err != nil {
				return err
			}
			c.Server.MCP = b
			return nil
		},
	},
	"server.autostart": {
		Get: func(c *config.UserConfig) interface{} { return c.Server.Autostart },
		Set: func(c *config.UserConfig, v string) error {
			b, err := parseBool(v)
			if err != nil {
				return err
			}
			c.Server.Autostart = b
			return nil
		},
	},
}

func parseBool(v string) (bool, error) {
	switch strings.ToLower(v) {
	case "true", "1", "yes":
		return true, nil
	case "false", "0", "no":
		return false, nil
	default:
		return false, fmt.Errorf("value must be true or false")
	}
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a configuration value",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		if len(args) == 0 {
			data, err := yaml.Marshal(cfg)
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		key := args[0]
		f, ok := configFields[key]
		if !ok {
			return fmt.Errorf("unknown key %q (run 'ywai config list' for available keys)", key)
		}

		fmt.Printf("%v\n", f.Get(cfg))
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		key, value := args[0], args[1]
		f, ok := configFields[key]
		if !ok {
			return fmt.Errorf("unknown key %q (run 'ywai config list' for available keys)", key)
		}

		if err := f.Set(cfg, value); err != nil {
			return fmt.Errorf("invalid value for %q: %w", key, err)
		}

		if err := config.SaveConfig(cfg); err != nil {
			return err
		}

		fmt.Printf("Set %s = %s\n", key, value)
		return nil
	},
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available configuration keys",
	Run: func(cmd *cobra.Command, args []string) {
		keys := make([]string, 0, len(configFields))
		for k := range configFields {
			keys = append(keys, k)
		}
		// Sort for consistent output
		for i := 0; i < len(keys); i++ {
			for j := i + 1; j < len(keys); j++ {
				if keys[i] > keys[j] {
					keys[i], keys[j] = keys[j], keys[i]
				}
			}
		}
		for _, k := range keys {
			fmt.Println(k)
		}
	},
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.DefaultConfig()
		if err := config.SaveConfig(cfg); err != nil {
			return err
		}
		fmt.Println("Configuration reset to defaults")
		return nil
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
		}
	},
}

var groupsCmd = &cobra.Command{
	Use:   "groups",
	Short: "List available agent groups",
	Long:  "List available agent groups from groups.json. Core group is always installed.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !config.IsDirPopulated(config.DataAgentsDir()) {
			if err := config.SeedAgentsFromEmbedded(); err != nil {
				return fmt.Errorf("no agent data available: %w", err)
			}
		}
		var names []string
		var err error
		// Try source dir first (has latest groups when running from source checkout),
		// fall back to data dir (seeded/embedded).
		names, err = agentprofiles.ListGroups(config.AgentsSourceDir())
		if err != nil {
			names, err = agentprofiles.ListGroups(config.DataAgentsDir())
		}
		if err != nil {
			return err
		}
		if len(names) == 0 {
			fmt.Println("No groups found.")
			return nil
		}
		for _, name := range names {
			fmt.Println(name)
		}
		return nil
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
		noUpdate, _ := cmd.Flags().GetBool("no-update")

		// MCP-only mode: talk over stdio, do NOT touch the HTTP port.
		// NewMCPAdapter will reuse the control server if it is already
		// running, or start a standalone one if needed.
		if mcpOnly {
			adapter := kanban.NewMCPAdapter()
			adapter.Run()
			return nil
		}

		// Ensure the port is available — kill any process holding it and wait
		// for the OS to release the socket before we try to bind our own server.
		ln, err := acquirePort(port)
		if err != nil {
			return err
		}
		_ = ln.Close()

		// Fork to background before doing any work
		if background {
			if err := daemonize(); err != nil {
				return err
			}
		}

		// Auto-update before starting (skip in MCP-only mode or if --no-update)
		if !noUpdate {
			if newVer, err := selfupdate.Run(version); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: auto-update failed: %v\n", err)
			} else if newVer != "" {
				// Binary was replaced — re-exec with the new version
				exe, err := selfupdate.ResolvedExecutable()
				if err != nil {
					return fmt.Errorf("failed to resolve ywai binary after update: %w", err)
				}
				fmt.Printf("Updated %s → %s, restarting...\n", version, newVer)
				reexecSelf(exe)
			}
		}

		// Auto-start opencode serve BEFORE the control server, so that when the
		// control server registers its chat routes (which probe for opencode),
		// opencode is already binding its port.
		startOpencodeServe()

		// Start control server
		control.AppVersion = version
		s, err := control.GetOrStart(port)
		if err != nil {
			return fmt.Errorf("failed to start control server: %w", err)
		}

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
	RunE: func(cmd *cobra.Command, args []string) error {
		url := getStringFlag(cmd, "url")
		key := getStringFlag(cmd, "key")

		if url == "" {
			return fmt.Errorf("--url is required")
		}
		if key == "" {
			return fmt.Errorf("--key is required")
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		cfg.TokenBankURL = url
		cfg.TokenBankAPIKey = key

		if err := config.SaveConfig(cfg); err != nil {
			return err
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
			return nil
		}
		fmt.Printf("  ✓ Connected! %d models available (default: %s)\n", len(models.Models), models.DefaultModel)
		return nil
	},
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP servers and authentication",
}

var mcpFastfsCmd = &cobra.Command{
	Use:   "fastfs",
	Short: "Run ywai-fastfs MCP server on stdio (search/read with mtime cache)",
	Long: `Starts the ywai-fastfs MCP server on stdin/stdout.

Long-lived process: the mtime file cache is shared across tool calls in the
same session (no per-call rg/cat fork). Register with:

  ywai install   # wires ywai-fastfs into opencode/claude/pi configs

Tools: fastfs_find, fastfs_search, fastfs_read_outline, fastfs_read_slice, fastfs_stat.
Prefer codegraph_explore for structural questions; use fastfs for text search.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, _ := cmd.Flags().GetString("cwd")
		adapter, err := fastfs.NewMCPAdapter(cwd)
		if err != nil {
			return err
		}
		adapter.Run()
		return nil
	},
}

var mcpAuthCmd = &cobra.Command{
	Use:   "auth <server-id>",
	Short: "Authenticate an MCP server via OAuth",
	Long: `Starts the OAuth Authorization Code flow (with PKCE) for a remote MCP server.

Opens your browser to the server's authorization URL, starts a local
callback server on a random port, exchanges the code for tokens, and
persists them in ~/.ywai/mcp-tokens/.

Supported servers:
  figma, github, gitlab, (any catalog entry with AuthType="oauth")

Example:
  ywai mcp auth figma
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		entry, ok := mcp.CatalogByID(id)
		if !ok {
			return fmt.Errorf("unknown MCP server %q. Run 'ywai mcp auth' to see supported servers", id)
		}
		if !entry.HasOAuth() {
			return fmt.Errorf("server %q does not require OAuth (AuthType=%q)", id, entry.AuthType)
		}
		return mcp.StartOAuthFlow(cmd.Context(), entry)
	},
}

var tokenbankConfigureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure agents to use TokenBank proxy",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		if cfg.TokenBankURL == "" || cfg.TokenBankAPIKey == "" {
			return fmt.Errorf("TokenBank not configured. Run 'ywai tokenbank setup --url <url> --key <key>' first")
		}

		agentFlag := getStringFlag(cmd, "agent")
		fmt.Println("=== TokenBank Configure ===")
		fmt.Printf("  Server: %s\n\n", cfg.TokenBankURL)

		if agentFlag != "" {
			// Configure a single agent
			switch agentFlag {
			case "opencode":
				if err := tokenbank.ConfigureOpenCode(cfg.TokenBankURL, cfg.TokenBankAPIKey); err != nil {
					return fmt.Errorf("error configuring opencode: %w", err)
				}
			case "pi":
				if err := tokenbank.ConfigurePi(cfg.TokenBankURL, cfg.TokenBankAPIKey); err != nil {
					return fmt.Errorf("error configuring pi: %w", err)
				}
			case "copilot":
				if err := tokenbank.ConfigureCopilot(cfg.TokenBankURL, cfg.TokenBankAPIKey); err != nil {
					return fmt.Errorf("error configuring copilot: %w", err)
				}
			default:
				return fmt.Errorf("unknown agent %q. Use: opencode, pi, copilot", agentFlag)
			}
		} else {
			// Configure all agents
			errors := tokenbank.ConfigureAll(cfg.TokenBankURL, cfg.TokenBankAPIKey)
			if len(errors) > 0 {
				var msgs []string
				for _, e := range errors {
					msgs = append(msgs, fmt.Sprintf("  ✗ %v", e))
				}
				return fmt.Errorf("some agents failed:\n%s", strings.Join(msgs, "\n"))
			}
		}

		fmt.Println("\nDone! Restart your agents to pick up the new configuration.")
		return nil
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

	serveCmd.Flags().IntP("port", "p", 5768, "Port for control server")
	serveCmd.Flags().BoolP("background", "b", false, "Run in background (detach from terminal)")
	serveCmd.Flags().Bool("no-mcp", false, "Don't start MCP adapter")
	serveCmd.Flags().Bool("mcp-only", false, "Run as MCP adapter only (stdio, no HTTP)")
	serveCmd.Flags().Bool("no-update", false, "Skip auto-update before starting")

	stopCmd.Flags().IntP("port", "p", 5768, "Port to stop (fallback if no PID file)")

	uiCmd.Flags().IntP("port", "p", kanban.DefaultUIPort, "Port for Kanban UI server")

	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(uiCmd)

	installCmd.Flags().StringP("agent", "a", "", "Specific agent to install for")
	installCmd.Flags().Bool("dry-run", false, "Preview changes without applying")
	installCmd.Flags().Bool("tui", false, "Force TUI mode")
	installCmd.Flags().Bool("mcp", false, "Install Microsoft Learn MCP (for opencode)")
	installCmd.Flags().Bool("global", true, "Run gentle-ai from a neutral dir so it does not write into the current project (default true; pass --global explicitly to skip the install TUI)")
	installCmd.Flags().String("preset", "full-gentleman", "gentle-ai component preset only (ywai skills always install): full-gentleman, ecosystem-only, minimal")
	installCmd.Flags().String("scope", "", "gentle-ai --scope: global (default) or workspace")
	installCmd.Flags().String("sdd-mode", "", "Optional gentle-ai SDD: single or multi (omit to skip SDD; persona is never installed)")
	installCmd.Flags().Bool("autostart", true, "Configure control server to start automatically on system boot")
	installCmd.Flags().StringSlice("group", []string{}, "Agent groups to install (repeatable, e.g., --group social-refactor)")
	installCmd.Flags().Bool("all-groups", false, "Install all agent groups")

	rootCmd.AddCommand(installCmd)

	updateCmd.Flags().Bool("beta", false, "Upgrade to the newest prerelease (beta) instead of stable latest")
	updateCmd.Flags().StringP("agent", "a", "", "Limit re-apply to one agent (default: all detected)")
	updateCmd.Flags().Bool("dry-run", false, "Preview changes without applying")
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(agentsCmd)
	rootCmd.AddCommand(skillsCmd)
	rootCmd.AddCommand(doctorCmd)

	skillRegistryCmd.Flags().String("cwd", "", "Project directory (defaults to current)")
	rootCmd.AddCommand(skillRegistryCmd)

	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configListCmd)
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

	// MCP commands
	mcpFastfsCmd.Flags().String("cwd", "", "Workspace root (default: process cwd)")
	mcpCmd.AddCommand(mcpFastfsCmd)
	mcpCmd.AddCommand(mcpAuthCmd)
	rootCmd.AddCommand(mcpCmd)
}

func isInteractiveTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// shouldRunInstallTUI decides whether `ywai install` opens the Bubbletea wizard.
//
// --global defaults to true (install scope). Its value must not suppress auto-TUI;
// only an explicitly changed --global, a set --agent, or --dry-run means flag-driven
// install. --tui always forces the wizard.
func shouldRunInstallTUI(tuiFlag bool, agentFlag string, dryRun, globalChanged bool) bool {
	flagDriven := agentFlag != "" || dryRun || globalChanged
	return tuiFlag || !flagDriven
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
