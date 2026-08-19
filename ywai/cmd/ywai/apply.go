package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	agentprofiles "github.com/Yoizen/dev-ai-workflow/ywai/internal/agents"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/configapi"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/gentlai"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/overrides"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/plugins"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/selfupdate"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/serverutil"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/versionfile"
)

// applyActiveModelProfile writes balanced/fast/deep (active) models into all host agent files.
func applyActiveModelProfile() (int, error) {
	return configapi.ApplyActiveOrchestratorProfile()
}

// applyMode selects which managed phases run after binary upgrades.
type applyMode int

const (
	applyInstall applyMode = iota
	applyUpdate
)

// managedPlan is the ywai-side work: everything ywai owns and always applies.
type managedPlan struct {
	CopyExtraSkills bool
	InstallProfiles bool
	WriteAgentsMd   bool
	InstallPlugins  bool
	RemoveQuota     bool
	SetDefaultAgent bool
	ApplyOverrides  bool
	RefreshVersion  bool
}

// planManaged returns the fixed ywai-managed work for install/update.
func planManaged(mode applyMode) managedPlan {
	_ = mode
	return managedPlan{
		CopyExtraSkills: true,
		InstallProfiles: true,
		WriteAgentsMd:   true,
		InstallPlugins:  true,
		RemoveQuota:     true,
		SetDefaultAgent: true,
		ApplyOverrides:  true,
		RefreshVersion:  true,
	}
}

// applyOpts drives the shared install/update pipeline.
type applyOpts struct {
	Mode applyMode

	Opts            gentlai.InstallOptions
	InstallMCP      bool
	InstallPonytail bool
	GroupFilter     agentprofiles.GroupFilter
	OverwriteAgents bool
	Autostart       bool

	// SkipGentleAIBinary skips gentlai.Install/Upgrade (caller already did it).
	SkipGentleAIBinary bool
	// RestartServeIfRunning restarts the control server only when it was up.
	RestartServeIfRunning bool
}

// applyResult collects non-fatal warnings; Fatal is a hard stop (e.g. no agents).
type applyResult struct {
	Warnings []string
	Fatal    error
}

func (r *applyResult) warnf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	r.Warnings = append(r.Warnings, msg)
	fmt.Printf("  Warning: %s\n", msg)
}

func (r *applyResult) exitCode() int {
	if r.Fatal != nil {
		return 1
	}
	if len(r.Warnings) > 0 {
		return 1
	}
	return 0
}

func (r *applyResult) printFooter(mode applyMode) {
	if r.Fatal != nil {
		fmt.Fprintf(os.Stderr, "\n=== Failed: %v ===\n", r.Fatal)
		return
	}
	if len(r.Warnings) > 0 {
		fmt.Println("\n=== Done with warnings ===")
		for _, msg := range r.Warnings {
			fmt.Printf("  - %s\n", msg)
		}
	} else {
		fmt.Println("\n=== Done! ===")
	}
	fmt.Println()
	if mode == applyUpdate {
		fmt.Println("Next step (once):")
		fmt.Println("  Restart OpenCode so it reloads plugins (vision-bridge, etc.).")
		fmt.Println("  Optional: open ywai Settings → Vision bridge to pick the vision model.")
		return
	}
	fmt.Println("Next steps:")
	fmt.Println("  1. Open your AI agent")
	fmt.Println("  2. Run `ywai skills` to see available skills")
}

type stepCounter struct {
	cur, total int
}

func (s *stepCounter) next(title string) {
	s.cur++
	fmt.Printf("\n[%d/%d] %s...\n", s.cur, s.total, title)
}

func countApplySteps(plan managedPlan, o applyOpts) int {
	n := 0
	n++ // reseed
	n++ // list agents
	n++ // ecosystem
	if plan.CopyExtraSkills {
		n++
	}
	if plan.InstallProfiles {
		n++ // Installing agent profiles
		n++ // Applying orchestrator model profile
		n++ // Configuring TokenBank providers
	}
	if plan.ApplyOverrides {
		n++
	}
	if plan.WriteAgentsMd {
		n++
	}
	if plan.InstallPlugins {
		n++
	}
	if plan.RemoveQuota {
		n++
	}
	if plan.SetDefaultAgent {
		n++
	}
	if plan.RefreshVersion {
		n++
	}
	if o.Autostart {
		n++
	}
	if o.RestartServeIfRunning {
		n++
	}
	if o.Mode == applyInstall {
		n++ // ensure control server running
	}
	return n
}

// applyManaged is the shared body for `ywai install` and `ywai update`.
func applyManaged(o applyOpts) applyResult {
	var r applyResult
	plan := planManaged(o.Mode)
	// Update always re-applies managed state with overwrite so new profiles land.
	if o.Mode == applyUpdate {
		o.OverwriteAgents = true
	}

	agents, err := resolveApplyAgents(o.Opts.AgentName)
	if err != nil {
		r.Fatal = err
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return r
	}

	// Update already printed its own banner + [pre] steps; just open the
	// managed phase. Install has no prior banner, so print the full one.
	if o.Mode == applyUpdate {
		fmt.Println("\n[apply] Re-applying managed state...")
	} else {
		fmt.Println("=== ywai install ===")
	}
	if o.Opts.DryRun {
		fmt.Println("\n[DRY RUN] No changes will be made.")
	}
	steps := stepCounter{total: countApplySteps(plan, o)}

	// ── reseed ────────────────────────────────────────────────────────────
	steps.next("Re-seeding skills + agent profile cache")
	if o.Opts.DryRun {
		fmt.Println("  Would re-seed ~/.ywai skills and agent profiles.")
	} else {
		reseedData()
	}

	// ── agents ────────────────────────────────────────────────────────────
	steps.next("Detecting agents")
	for _, a := range agents {
		fmt.Printf("  Found: %s (%s)\n", a.Name, a.BinaryName)
	}

	// ── ecosystem ─────────────────────────────────────────────────────────
	steps.next("Installing Engram")
	installEcosystem(agents, o.Opts.DryRun, o.Opts)

	// ── extra skills (always) ─────────────────────────────────────────────
	if plan.CopyExtraSkills {
		steps.next("Copying ywai extra skills")
		copySkillsForAgents(agents, o.Opts.DryRun)
		if o.Opts.DryRun {
			fmt.Println("  Would install /learn-ywai slash command")
		} else if err := plugins.InstallLearnYwaiCommand(plugins.DefaultLearnYwaiCommandDirs()...); err != nil {
			fmt.Printf("  Warning: failed to install /learn-ywai: %v\n", err)
		}
	}

	agentDirs := make(map[string]string, len(agents))
	for _, a := range agents {
		agentDirs[a.Name] = a.SkillsDir
	}

	// ── profiles ──────────────────────────────────────────────────────────
	if plan.InstallProfiles {
		steps.next("Installing agent profiles")
		installAgentProfiles(agents, o.Opts.DryRun, o.GroupFilter, o.OverwriteAgents)

		// Write active model profile (balanced/fast/deep) into each agent .md
		// on every host we just installed.
		steps.next("Applying orchestrator model profile")
		if o.Opts.DryRun {
			fmt.Println("  Would apply active orchestrator model profile to installed agents")
		} else {
			n, err := applyActiveModelProfile()
			if err != nil {
				r.warnf("model profile apply: %v", err)
			} else {
				fmt.Printf("  ✓ model profile applied to %d agent role(s)\n", n)
			}
		}

		// TokenBank proxy into opencode / pi / omp / copilot when credentials exist.
		steps.next("Configuring TokenBank providers")
		reapplyTokenBank(o.Opts.DryRun)
	}

	// ── overrides ─────────────────────────────────────────────────────────
	if plan.ApplyOverrides {
		steps.next("Applying ywai overrides")
		if o.Opts.DryRun {
			fmt.Println("  Would apply OpenSpec→SDD overrides.")
		} else if err := overrides.ApplyOpenSpecToSDDOverride(agentDirs); err != nil {
			r.warnf("failed to apply overrides: %v", err)
		} else {
			fmt.Println("  ✓ overrides applied")
		}
	}

	// ── AGENTS.md ─────────────────────────────────────────────────────────
	// MUST run before plugins so Graft can append its marker section.
	// Also runs BEFORE optional SDD so it can re-inject marker blocks.
	if plan.WriteAgentsMd {
		steps.next("Writing curated AGENTS.md")
		if o.Opts.DryRun {
			fmt.Println("  Would write curated AGENTS.md (engram + sub-agents + graft)")
		} else {
			agentsMdPath := filepath.Join(config.OpenCodeConfigDir(), "AGENTS.md")
			if err := agentprofiles.WriteAgentsMd(agentsMdPath); err != nil {
				r.warnf("failed to write AGENTS.md: %v", err)
			} else {
				fmt.Println("  ✓ AGENTS.md written (engram + sub-agents + graft)")
			}
		}
	}

	// ── plugins + CLIs ────────────────────────────────────────────────────
	if plan.InstallPlugins {
		steps.next("Installing plugins + MCP + companion CLIs")
		installPluginsForAgents(agents, o.Opts.DryRun, o.InstallMCP, o.InstallPonytail)
	}

	// ── cleanup ───────────────────────────────────────────────────────────
	if plan.RemoveQuota {
		steps.next("Removing deprecated opencode-quota plugin")
		removeQuotaForAgents(agents, o.Opts.DryRun)
	}

	// ── default agent ─────────────────────────────────────────────────────
	if plan.SetDefaultAgent {
		steps.next("Setting default_agent")
		if err := setDefaultAgent("orchestrator", o.Opts.DryRun); err != nil {
			r.warnf("failed to set default_agent: %v", err)
		}
	}

	// ── version file ──────────────────────────────────────────────────────
	if plan.RefreshVersion {
		steps.next("Refreshing version info")
		if o.Opts.DryRun {
			fmt.Println("  Would refresh ~/.ywai/version.json")
		} else if err := versionfile.Refresh(version, 24*time.Hour); err != nil {
			r.warnf("failed to write version info: %v", err)
		} else {
			fmt.Println("  ✓ version info refreshed")
		}
	}

	// ── autostart ─────────────────────────────────────────────────────────
	if o.Autostart {
		steps.next("Configuring control server autostart")
		if o.Opts.DryRun {
			fmt.Println("  Would configure control server autostart")
		} else if err := configureAutostart(); err != nil {
			r.warnf("failed to configure autostart: %v", err)
		} else {
			fmt.Println("  ✓ autostart configured")
		}
	}

	// ── control server restart (only if it was running) ───────────────────
	if o.RestartServeIfRunning {
		steps.next("Restarting control server (if running)")
		restartControlServerIfRunning(&r, o.Opts.DryRun)
	}

	// ── strip agent frontmatter keys opencode v2 rejects ──────────────────
	// Runs after every writer: several of them rebuild these files by keeping
	// existing frontmatter lines, so a stale key survives until swept.
	if o.Mode == applyInstall && !o.Opts.DryRun {
		// OpenCodeAgentsDir may point at a host-managed location (e.g. Orca's
		// shared hooks dir). opencode itself always reads ~/.config/opencode/
		// agents, and other tooling syncs into it, so sweep both.
		seen := map[string]bool{}
		dirs := []string{config.OpenCodeAgentsDir()}
		if home, err := os.UserHomeDir(); err == nil {
			dirs = append(dirs, filepath.Join(home, ".config", "opencode", "agents"))
		}
		cleaned := 0
		for _, dir := range dirs {
			if dir == "" || seen[dir] {
				continue
			}
			seen[dir] = true
			cleaned += agentprofiles.StripLegacyAgentKeys(dir)
		}
		if cleaned > 0 {
			fmt.Printf("  Cleaned legacy frontmatter keys in %d agent files\n", cleaned)
		}
	}

	// ── control server start (install: ensure it is running) ──────────────
	if o.Mode == applyInstall {
		steps.next("Starting control server (if not running)")
		ensureControlServerRunning(&r, o.Opts.DryRun)
	}

	return r
}

func resolveApplyAgents(agentName string) ([]agent.Agent, error) {
	if agentName != "" {
		a, err := agent.FindByName(agentName)
		if err != nil {
			return nil, err
		}
		return []agent.Agent{*a}, nil
	}
	agents := agent.Resolve()
	if len(agents) == 0 {
		return nil, fmt.Errorf("no supported agents detected")
	}
	return agents, nil
}

func restartControlServerIfRunning(r *applyResult, dryRun bool) {
	port := serverutil.GetRunningPort()
	if port <= 0 {
		fmt.Println("  Control server was not running; leaving it stopped.")
		return
	}
	if dryRun {
		fmt.Printf("  Would restart control server on port %d\n", port)
		return
	}
	fmt.Printf("  Stopping server on port %d...\n", port)
	if err := killPort(port); err != nil {
		r.warnf("could not kill server on port %d: %v", port, err)
		return
	}
	fmt.Println("  Server stopped.")
	exe, err := selfupdate.ResolvedExecutable()
	if err != nil {
		r.warnf("could not find ywai binary: %v", err)
		return
	}
	serveCmd := exec.Command(exe, "serve", "--background", "--no-update")
	serveCmd.SysProcAttr = sysProcAttr()
	serveCmd.Stdout = os.Stdout
	serveCmd.Stderr = os.Stderr
	if err := serveCmd.Start(); err != nil {
		r.warnf("could not restart server: %v", err)
		return
	}
	fmt.Printf("  Server restarted in background (PID %d)\n", serveCmd.Process.Pid)
}

// ensureControlServerRunning starts the control server in the background when
// it is not already healthy (mirrors `ywai serve -b`). Install uses this so a
// successful install leaves the control server up; dry runs only preview.
func ensureControlServerRunning(r *applyResult, dryRun bool) {
	port := detectRunningServer()
	if port > 0 {
		fmt.Printf("  Control server already running on port %d\n", port)
		return
	}
	if dryRun {
		fmt.Println("  Would start control server in background")
		return
	}
	if err := launchControlServer(); err != nil {
		r.warnf("could not start control server: %v", err)
	}
}

// startControlServer launches the current binary with `serve --background`,
// detaching it from the terminal just like `ywai serve -b`.
func startControlServer() error {
	exe, err := selfupdate.ResolvedExecutable()
	if err != nil {
		return err
	}
	serveCmd := exec.Command(exe, "serve", "--background", "--no-update")
	serveCmd.SysProcAttr = sysProcAttr()
	serveCmd.Stdout = os.Stdout
	serveCmd.Stderr = os.Stderr
	if err := serveCmd.Start(); err != nil {
		return err
	}
	fmt.Printf("  Control server started in background (PID %d)\n", serveCmd.Process.Pid)
	return nil
}

// Injectable seams so tests can stub the health check and the launch without
// touching a real server or spawning a process.
var (
	detectRunningServer = serverutil.GetRunningPort
	launchControlServer = startControlServer
)
