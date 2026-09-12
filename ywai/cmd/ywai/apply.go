package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	agentprofiles "github.com/Yoizen/dev-ai-workflow/ywai/internal/agents"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/configapi"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/envprofile"
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
	SetDefaultAgent bool
	SetDefaultModel bool
	ApplyOverrides  bool
	RefreshVersion  bool
	ExportWorkflows bool
	// Reseed refreshes the shared ~/.ywai skills + agent profile cache.
	// Profile-scoped applies skip it: the cache is global state outside
	// the profile sandbox.
	Reseed bool
}

// planManaged returns the fixed ywai-managed work for install/update.
// The optional profile names a profile-scoped apply: with one set, the
// shared reseed is skipped. Variadic so existing callers keep compiling.
func planManaged(mode applyMode, profile ...string) managedPlan {
	_ = mode
	scoped := len(profile) > 0 && strings.TrimSpace(profile[0]) != ""
	return managedPlan{
		CopyExtraSkills: true,
		InstallProfiles: true,
		WriteAgentsMd:   true,
		InstallPlugins:  true,
		SetDefaultAgent: true,
		SetDefaultModel: true,
		ApplyOverrides:  true,
		RefreshVersion:  true,
		ExportWorkflows: true,
		Reseed:          !scoped,
	}
}

// applyOpts drives the shared install/update pipeline.
type applyOpts struct {
	Mode applyMode

	// Profile names an isolated environment (`ywai env`). When set, the
	// pipeline runs inside the profile sandbox and applies only there;
	// the global install is left alone.
	Profile string

	Opts            gentlai.InstallOptions
	InstallMCP      bool
	InstallMetaMCP  bool
	InstallPonytail bool
	GroupFilter     agentprofiles.GroupFilter
	OverwriteAgents bool
	Autostart       bool

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
		fmt.Println("  Reopen any OpenCode session listed above so it reloads plugins and agents.")
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
	n++ // reseed (profile scope prints a skip line, so the count is stable)
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
	if plan.ExportWorkflows {
		n++
	}
	if plan.WriteAgentsMd {
		n++
	}
	if plan.InstallPlugins {
		n++
	}
	if plan.SetDefaultAgent {
		n++
	}
	if plan.SetDefaultModel {
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
	if o.Mode == applyInstall || o.Mode == applyUpdate {
		n++ // restart OpenCode
	}
	return n
}

// presetApplyContext carries preset enforcement values for one apply run.
// InScope is false for global applies, where every other field stays zero
// and behavior is bit-identical to before presets existed.
type presetApplyContext struct {
	InScope      bool
	Bare         bool
	DefaultAgent string
	DefaultModel string
	DenyBash     []string
}

// applyPresetHook enforces the profile preset for scoped applies. A bare
// preset disables the install steps entirely (agents, skills, MCP/plugins,
// defaults, AGENTS.md); other presets only contribute
// default_agent/default_model/deny_bash here — groups/skills/MCP filtering
// lives in root.go via envprofile helpers. Global applies return zero.
// DB/service/pids are untouched by every branch below by design.
func applyPresetHook(plan *managedPlan, o *applyOpts) presetApplyContext {
	var ctx presetApplyContext
	name := strings.TrimSpace(o.Profile)
	if name == "" {
		name = envprofile.ScopedProfileName()
	}
	if name == "" {
		return ctx
	}
	p, err := envprofile.Get(name)
	if err != nil {
		return ctx
	}
	spec, err := envprofile.PresetSpec(p)
	if err != nil {
		return ctx
	}
	ctx.InScope = true
	if envprofile.IsBareSpec(spec) {
		ctx.Bare = true
		plan.CopyExtraSkills = false
		plan.InstallProfiles = false
		plan.InstallPlugins = false
		plan.SetDefaultAgent = false
		plan.SetDefaultModel = false
		plan.WriteAgentsMd = false
		plan.ExportWorkflows = false
		plan.ApplyOverrides = false
		fmt.Printf("  Bare preset %q: skipping agents, skills, MCP/plugins, defaults, AGENTS.md, workflows and overrides.\n", p.Preset)
		fmt.Println("  Skipping agent profiles install for bare preset.")
		fmt.Println("  Skipping skills copy for bare preset.")
		fmt.Println("  Skipping MCP/plugin wiring for bare preset.")
		fmt.Println("  Skipping default_agent/default_model writes for bare preset.")
		fmt.Println("  Skipping AGENTS.md write for bare preset.")
		fmt.Println("  Skipping workflows export and overrides for bare preset.")
		return ctx
	}
	ctx.DefaultAgent = envprofile.PresetDefaultAgent(spec)
	ctx.DefaultModel = envprofile.PresetDefaultModel(spec)
	ctx.DenyBash = envprofile.PresetDenyBash(spec)
	fmt.Printf("  Enforcing preset %q.\n", p.Preset)
	return ctx
}

// applyManagedScoped runs the shared pipeline inside the profile sandbox
// when o.Profile names an environment, and runs it directly otherwise.
// The sandbox redirects every XDG/OPENCODE path the pipeline writes
// through, so the step bodies need no profile branches.
func applyManagedScoped(o applyOpts) applyResult {
	name := strings.TrimSpace(o.Profile)
	if name == "" {
		return applyManaged(o)
	}
	p, err := envprofile.Get(name)
	if err != nil {
		var r applyResult
		r.Fatal = fmt.Errorf("unknown environment %q: %w (create it with `ywai env create %s`)", name, err, name)
		fmt.Fprintf(os.Stderr, "Error: %v\n", r.Fatal)
		return r
	}
	// Envs created before the managed-service port fix have no service.json:
	// their TUI/debug would collide with the global service on the default
	// port. Pin it on every apply so re-applying fixes old envs too.
	if err := envprofile.EnsureServicePort(p); err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: could not pin env service port: %v\n", err)
	}
	fmt.Printf("Applying inside environment %q (global install untouched)...\n", p.Name)
	var r applyResult
	if err := envprofile.WithProfileEnv(p, func() error {
		r = applyManaged(o)
		return nil
	}); err != nil {
		r.Fatal = err
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	return r
}

// applyManaged is the shared body for `ywai install` and `ywai update`.
func applyManaged(o applyOpts) applyResult {
	var r applyResult
	o.Profile = strings.TrimSpace(o.Profile)
	if o.Profile != "" {
		// Profile-scoped applies run inside the profile sandbox and must
		// not touch shared machine state: no control-server restart, no
		// autostart registration (the machine has one control server).
		o.RestartServeIfRunning = false
		o.Autostart = false
	}
	plan := planManaged(o.Mode, o.Profile)
	// Preset enforcement hook (profile scope only; global bit-identical).
	preset := applyPresetHook(&plan, &o)
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

	// Gate: OpenCode 2 is the minimum supported host. Runs before any write so
	// a machine with only the retired v1 `opencode` binary fails fast with the
	// withdrawal notice instead of receiving v2-shaped config it rejects.
	for _, a := range agents {
		if a.Name != "opencode" {
			continue
		}
		if err := agent.GateOpenCodeV2(); err != nil {
			r.Fatal = err
			fmt.Fprintf(os.Stderr, "Error: %v.\n", err)
			return r
		}
		break
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
	switch {
	case !plan.Reseed:
		fmt.Println("  Skipping global re-seed for profile-scoped apply.")
	case o.Opts.DryRun:
		fmt.Println("  Would re-seed ~/.ywai skills and agent profiles.")
	default:
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

	// ── workflows ─────────────────────────────────────────────────────────
	// MUST run after the profile install: that step prunes every agent .md
	// outside the installed profile set, which takes the workflow sub-agents
	// with it. Re-exporting also rewrites commands whose frontmatter came from
	// an older ywai.
	if plan.ExportWorkflows {
		steps.next("Re-exporting workflows")
		n, err := exportInstalledWorkflows(o.Opts.DryRun)
		switch {
		case err != nil:
			r.warnf("failed to export workflows: %v", err)
		case n == 0:
			fmt.Println("  No workflows to export")
		case o.Opts.DryRun:
			fmt.Printf("  Would re-export %d workflow(s)\n", n)
		default:
			fmt.Printf("  ✓ %d workflow(s) re-exported\n", n)
		}
	}

	// Preset enforcement: append preset deny_bash patterns to every agent's
	// shell rules. Runs after the workflow export so workflow sub-agents are
	// covered too (they are written by that step, after the profile install).
	// The agents dir is already sandbox-resolved: global is never touched.
	if plan.InstallProfiles && preset.InScope && !preset.Bare && len(preset.DenyBash) > 0 {
		if o.Opts.DryRun {
			fmt.Printf("  Would append %d preset shell-deny rule(s)\n", len(preset.DenyBash))
		} else if n, err := envprofile.AppendDenyBashToAgents(config.OpenCodeAgentsDir(), preset.DenyBash); err != nil {
			r.warnf("failed to append preset shell-deny rules: %v", err)
		} else if n > 0 {
			fmt.Printf("  ✓ %d agent file(s) gained preset shell-deny rules\n", n)
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
		installPluginsForAgents(agents, o.Opts.DryRun, o.InstallMCP, o.InstallMetaMCP, o.InstallPonytail)
	}

	// ── default agent ─────────────────────────────────────────────────────
	if plan.SetDefaultAgent {
		steps.next("Setting default_agent")
		// Under preset scope the preset IS the choice (same rule as the
		// default model below): switching an env to another preset must move
		// its default agent too. Globally a user pick is never overwritten.
		if preset.DefaultAgent != "" {
			if err := setDefaultAgentForced(preset.DefaultAgent, o.Opts.DryRun); err != nil {
				r.warnf("failed to set default_agent: %v", err)
			}
		} else if err := setDefaultAgent("orchestrator", o.Opts.DryRun); err != nil {
			r.warnf("failed to set default_agent: %v", err)
		}
	}

	// ── default model ─────────────────────────────────────────────────────
	// Root `model` is the default for a new session. Globally it is written
	// only when absent or empty; a model the user picked stays untouched.
	// Under preset scope the preset IS the choice, so it overwrites.
	if plan.SetDefaultModel {
		steps.next("Setting default model")
		wantModel := defaultRootModel()
		if preset.DefaultModel != "" {
			wantModel = preset.DefaultModel
			if err := setDefaultModelForced(wantModel, o.Opts.DryRun); err != nil {
				r.warnf("failed to set default model: %v", err)
			}
		} else if err := setDefaultModel(wantModel, o.Opts.DryRun); err != nil {
			r.warnf("failed to set default model: %v", err)
		}
	}

	// Follow-up (not this lane): wire preset cli_theme with
	// cfg["theme"] = theme via openCodeRootForWrite/writeOpenCodeRoot
	// (profile-scoped opencode.json).

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

	// ── OpenCode restart ──────────────────────────────────────────────────
	// Plugins, MCP servers and agent frontmatter are read once at startup, so
	// everything written above is inert in an already-running OpenCode. Servers
	// are stopped here; interactive sessions are only reported.
	if o.Mode == applyInstall || o.Mode == applyUpdate {
		steps.next("Restarting OpenCode so it reloads plugins and agents")
		restartOpenCodeServers(&r, o.Opts.DryRun)
	}

	// ── strip agent frontmatter keys opencode v2 rejects ──────────────────
	// Runs after every writer: several of them rebuild these files by keeping
	// existing frontmatter lines, so a stale key survives until swept.
	if !o.Opts.DryRun && (o.Mode == applyInstall || o.Mode == applyUpdate) {
		// OpenCodeAgentsDir may point at a host-managed location (e.g. Orca's
		// shared hooks dir). opencode itself always reads ~/.config/opencode/
		// agents, and other tooling syncs into it, so sweep both — except
		// under profile scope, where the canonical copy is global state.
		seen := map[string]bool{}
		dirs := []string{config.OpenCodeAgentsDir()}
		if os.Getenv("YWAI_PROFILE") == "" {
			if home, err := os.UserHomeDir(); err == nil {
				dirs = append(dirs, filepath.Join(home, ".config", "opencode", "agents"))
			}
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
	// Never under profile scope: the machine has one control server and the
	// scoped apply may itself run as its child (web Apply would orphan).
	if o.Mode == applyInstall && strings.TrimSpace(o.Profile) == "" && os.Getenv("YWAI_PROFILE") == "" {
		steps.next("Starting control server (if not running)")
		ensureControlServerRunning(&r, o.Opts.DryRun)
		// Last, so Orca gets everything the steps above wrote. Update mirrors
		// after its own final TokenBank pass instead.
		mirrorOpenCodeToOrca(o.Opts.DryRun)
	}

	return r
}

// mirrorOpenCodeToOrca copies the user's opencode.json over Orca's shared
// OpenCode config. Orca launches OpenCode with OPENCODE_CONFIG_DIR pointing
// there, so ywai runs from any other terminal left Orca on stale providers
// and agents. A full copy by design: Orca's config is meant to be the user's.
// The replaced file is kept as .bak. No-op when Orca is not installed.
func mirrorOpenCodeToOrca(dryRun bool) {
	// Profile-scoped applies run inside the profile sandbox: the global
	// Orca mirror is shared state and stays untouched.
	if strings.TrimSpace(os.Getenv("YWAI_PROFILE")) != "" {
		return
	}
	userDir := config.OpenCodeUserConfigDir()
	orcaDir := filepath.Join(filepath.Dir(userDir), "orca", "opencode-hooks", "shared")
	if info, err := os.Stat(orcaDir); err != nil || !info.IsDir() {
		return
	}
	src := filepath.Join(userDir, "opencode.json")
	dst := filepath.Join(orcaDir, "opencode.json")
	data, err := os.ReadFile(src)
	if err != nil {
		fmt.Printf("  Warning: Orca OpenCode config not synced: %v\n", err)
		return
	}
	old, readErr := os.ReadFile(dst)
	if readErr == nil && string(old) == string(data) {
		return
	}
	if dryRun {
		fmt.Printf("  Would sync %s → %s\n", src, dst)
		return
	}
	if readErr == nil {
		if err := os.WriteFile(dst+".bak", old, 0o644); err != nil {
			fmt.Printf("  Warning: Orca OpenCode config not synced: %v\n", err)
			return
		}
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		fmt.Printf("  Warning: Orca OpenCode config not synced: %v\n", err)
		return
	}
	fmt.Printf("  ✓ Orca OpenCode config synced: %s\n", dst)
}

// resolveApplyAgents returns the agents to install for this run.
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
		// The kill can fail for reasons the update cannot fix — on Windows
		// taskkill exits 1 when the process belongs to another session or is
		// elevated. Say what that costs: the old server keeps the port, so it
		// serves the previous build's UI and API until someone restarts it.
		// Reporting only the taskkill error made that invisible.
		if serverutil.GetRunningPort() == port {
			r.warnf("could not stop the control server on port %d (%v); it keeps running the previous version "+
				"and will serve the old UI until you stop it yourself and run `ywai serve`", port, err)
			return
		}
		fmt.Printf("  Server on port %d exited despite: %v\n", port, err)
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
