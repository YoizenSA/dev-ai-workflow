package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/envprofile"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/gentlai"
	"github.com/spf13/cobra"
)

var (
	envPreset string
	envInit   bool
	envYes    bool

	envBootstrapPresets string
)

var envBootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Create and install the default environments (dev, qa, personal) from zero",
	Long: `Zero-to-configured environments: creates every missing default profile
with its preset and always runs a full install inside each one (personal is
bare, so its install is nearly a no-op). Existing profiles are re-installed,
never deleted. Safe to re-run.

Examples:
  ywai env bootstrap
  ywai env bootstrap --presets dev,qa`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runEnvBootstrap()
	},
}

func runEnvBootstrap() error {
	var presets []string
	for _, p := range strings.Split(envBootstrapPresets, ",") {
		if p = strings.TrimSpace(p); p != "" {
			presets = append(presets, p)
		}
	}
	if len(presets) == 0 {
		presets = envprofile.Presets()
	}
	ready := make([]envprofile.Profile, 0, len(presets))
	for _, name := range presets {
		if _, err := envprofile.Preset(name); err != nil {
			return fmt.Errorf("unknown preset %q", name)
		}
		if envprofile.Exists(name) {
			fmt.Printf("Environment %q already exists — re-installing\n", name)
			p, err := envprofile.Get(name)
			if err != nil {
				return err
			}
			ready = append(ready, p)
			continue
		}
		p, err := envprofile.Create(name, name)
		if err != nil {
			return err
		}
		fmt.Printf("Created environment %q (preset %s, port %d)\n", p.Name, p.Preset, p.Port)
		ready = append(ready, p)
	}
	failed := 0
	for _, p := range ready {
		fmt.Printf("\n=== Installing into environment %q ===\n", p.Name)
		r := applyManagedScoped(applyOpts{
			Mode:            applyInstall,
			Profile:         p.Name,
			Opts:            gentlai.InstallOptions{AgentName: "opencode"},
			OverwriteAgents: true,
		})
		r.printFooter(applyInstall)
		if code := r.exitCode(); code != 0 {
			fmt.Fprintf(os.Stderr, "  install into %q failed (exit %d), continuing\n", p.Name, code)
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d environment install(s) failed", failed)
	}
	fmt.Println("\nAll default environments configured.")
	return nil
}

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage isolated opencode2 environments",
	Long: `Each environment is a full isolated opencode2 instance: its own config,
database, background service and auth, stored under ~/.ywai/profiles/<name>/.
The global install is never touched by profile commands.

Shortcuts: every profile also registers as a top-level command
(ywai dev, ywai qa, ...) equivalent to 'ywai env start <name>'.

Examples:
  ywai env list
  ywai env create qa --preset qa
  ywai env start dev
  ywai dev "run the tests"`,
}

var envListCmd = &cobra.Command{
	Use:   "list",
	Short: "List isolated environments",
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles, err := envprofile.List()
		if err != nil {
			return err
		}
		if len(profiles) == 0 {
			fmt.Println("No environments. Create one: ywai env create dev --preset dev")
			return nil
		}
		fmt.Printf("%-12s %-8s %-6s %s\n", "NAME", "PRESET", "PORT", "STATUS")
		for _, p := range profiles {
			status := "stopped"
			if running, err := envprofile.Status(p); err == nil && running {
				status = "healthy"
			}
			fmt.Printf("%-12s %-8s %-6d %s\n", p.Name, p.Preset, p.Port, status)
		}
		return nil
	},
}

var envCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create an isolated environment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		preset := envPreset
		if preset == "" {
			preset = "dev"
		}
		if _, err := envprofile.Preset(preset); err != nil {
			return err
		}
		p, err := envprofile.Create(name, preset)
		if err != nil {
			return err
		}
		if envInit {
			if err := copyGlobalConfigInto(p); err != nil {
				return err
			}
		}
		fmt.Printf("Created environment %q (preset %s, port %d)\n", p.Name, p.Preset, p.Port)
		fmt.Printf("Start it: ywai %s\n", p.Name)
		return nil
	},
}

var envStartCmd = &cobra.Command{
	Use:   "start <name> [prompt...]",
	Short: "Start (and enter) an isolated environment",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return envStart(args[0], args[1:])
	},
}

var envStopCmd = &cobra.Command{
	Use:   "stop <name>",
	Short: "Stop an environment server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := envprofile.Get(args[0])
		if err != nil {
			return err
		}
		if err := envprofile.Stop(p); err != nil {
			return err
		}
		fmt.Printf("Environment %q stopped\n", p.Name)
		return nil
	},
}

var envStatusCmd = &cobra.Command{
	Use:   "status <name>",
	Short: "Show an environment (read-only)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := envprofile.Get(args[0])
		if err != nil {
			return err
		}
		dirs := envprofile.Dirs(p)
		running, _ := envprofile.Status(p)
		state := "stopped"
		if running {
			state = "healthy"
		}
		fmt.Printf("profile:  %s (preset %s, created %s)\n", p.Name, p.Preset, p.CreatedAt)
		fmt.Printf("config:   %s\n", filepath.Join(dirs["config"], "opencode"))
		fmt.Printf("database: %s\n", filepath.Join(dirs["data"], "opencode", "opencode.db"))
		fmt.Printf("service:  %s (%s)\n", envprofile.Env(p)["OPENCODE_URL"], state)
		fmt.Printf("log:      %s\n", envprofile.ServerLog(p))
		return nil
	},
}

var envDoctorCmd = &cobra.Command{
	Use:   "doctor [name]",
	Short: "Check environments (read-only)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := agent.GateOpenCodeV2(); err != nil {
			return err
		}
		bin, _ := agent.FindOpenCode()
		if bin == "" {
			return fmt.Errorf("opencode2 binary not found in PATH")
		}
		fmt.Printf("[ok] opencode2 at %s\n", bin)
		names := args
		if len(names) == 0 {
			profiles, err := envprofile.List()
			if err != nil {
				return err
			}
			for _, p := range profiles {
				names = append(names, p.Name)
			}
		}
		for _, name := range names {
			p, err := envprofile.Get(name)
			if err != nil {
				return err
			}
			running, _ := envprofile.Status(p)
			svc := "stopped"
			if running {
				svc = "healthy"
			}
			fmt.Printf("[ok] profile %q service %s port %d\n", p.Name, svc, p.Port)
		}
		return nil
	},
}

var envRemoveCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Stop and delete an environment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := envprofile.Get(args[0])
		if err != nil {
			return err
		}
		if !envYes {
			return fmt.Errorf("refusing to remove %q without --yes", p.Name)
		}
		if err := envprofile.Stop(p); err != nil {
			return err
		}
		if err := envprofile.Delete(p.Name); err != nil {
			return err
		}
		fmt.Printf("Environment %q removed\n", p.Name)
		return nil
	},
}

var envCloneCmd = &cobra.Command{
	Use:   "clone <src> <dst>",
	Short: "Clone an environment config into a fresh one (fresh DB)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		src, err := envprofile.Get(args[0])
		if err != nil {
			return err
		}
		dst, err := envprofile.Create(args[1], src.Preset)
		if err != nil {
			return err
		}
		// Never copy a hot database or service state: config tree only,
		// skipping the private service file.
		srcCfg := filepath.Join(envprofile.Dirs(src)["config"], "opencode")
		dstCfg := filepath.Join(envprofile.Dirs(dst)["config"], "opencode")
		if err := copyTreeFiltered(srcCfg, dstCfg, map[string]bool{"service.json": true}); err != nil {
			return err
		}
		fmt.Printf("Cloned %q -> %q (port %d, fresh database)\n", src.Name, dst.Name, dst.Port)
		return nil
	},
}

func init() {
	envCmd.AddCommand(envListCmd)
	envCmd.AddCommand(envCreateCmd)
	envCmd.AddCommand(envStartCmd)
	envCmd.AddCommand(envStopCmd)
	envCmd.AddCommand(envStatusCmd)
	envCmd.AddCommand(envDoctorCmd)
	envCmd.AddCommand(envRemoveCmd)
	envCmd.AddCommand(envCloneCmd)
	envBootstrapCmd.Flags().StringVar(&envBootstrapPresets, "presets", "", "Comma-separated presets to bootstrap (default: all embedded presets)")
	envCmd.AddCommand(envBootstrapCmd)
	envCreateCmd.Flags().StringVar(&envPreset, "preset", "dev", "Preset to stamp (dev, qa, personal)")
	envCreateCmd.Flags().BoolVar(&envInit, "init", false, "Seed from the current global opencode config (config files only, never DB/service)")
	envRemoveCmd.Flags().BoolVar(&envYes, "yes", false, "Confirm deletion")
	rootCmd.AddCommand(envCmd)

	// Dynamic shortcuts: ywai <profile> == ywai env start <profile>.
	// Stored profiles were validated at create time; re-guard anyway so a
	// hand-placed directory can never shadow a real command.
	if profiles, err := envprofile.List(); err == nil {
		for _, p := range profiles {
			if err := envprofile.ValidateName(p.Name); err != nil {
				continue
			}
			name := p.Name
			rootCmd.AddCommand(&cobra.Command{
				Use:   name,
				Short: "Enter isolated environment " + name,
				Args:  cobra.ArbitraryArgs,
				RunE: func(cmd *cobra.Command, args []string) error {
					return envStart(name, args)
				},
			})
		}
	}
}

// envStart ensures the profile server runs, then execs opencode2 under the
// profile environment: bare for the interactive TUI, with args as a headless
// `opencode2 run` prompt.
func envStart(name string, promptArgs []string) error {
	if err := agent.GateOpenCodeV2(); err != nil {
		return err
	}
	bin, _ := agent.FindOpenCode()
	if bin == "" {
		return fmt.Errorf("opencode2 binary not found in PATH")
	}
	p, err := envprofile.Get(name)
	if err != nil {
		return err
	}
	running, err := envprofile.Status(p)
	if err != nil {
		return err
	}
	if !running {
		if err := envprofile.Start(context.Background(), p, bin); err != nil {
			return err
		}
	}
	argv := []string{bin}
	if len(promptArgs) > 0 {
		argv = append(argv, "run")
		argv = append(argv, promptArgs...)
	}
	return runUnderProfileEnv(p, argv)
}

// runUnderProfileEnv runs argv[0] with the profile environment applied,
// inheriting stdio. It replaces runUnderProfileEnv's caller semantics with a
// plain child process (portable; no syscall.Exec) so Windows behaves.
func runUnderProfileEnv(p envprofile.Profile, argv []string) error {
	env := os.Environ()
	for k, v := range envprofile.Env(p) {
		env = append(env, k+"="+v)
	}
	user, pass, err := envprofile.EnsureCreds(p)
	if err != nil {
		return err
	}
	env = append(env, "OPENCODE_SERVER_USERNAME="+user, "OPENCODE_SERVER_PASSWORD="+pass)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// copyGlobalConfigInto seeds a profile from the user's current global config.
// Config files only: service.json and any database stay behind by construction
// (the global config dir holds no database).
func copyGlobalConfigInto(p envprofile.Profile) error {
	src := config.OpenCodeUserConfigDir()
	dst := filepath.Join(envprofile.Dirs(p)["config"], "opencode")
	return copyTreeFiltered(src, dst, map[string]bool{"service.json": true})
}

// copyTreeFiltered copies src into dst, skipping excluded base names.
func copyTreeFiltered(src, dst string, exclude map[string]bool) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if exclude[d.Name()] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// Follow-up wiring (no edits made here): add "env" to skipDataSeeding in
// root.go so read-only env commands (list, status, doctor) skip data seeding.
