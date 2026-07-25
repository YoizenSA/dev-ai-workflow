package main

import (
	"fmt"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/spf13/cobra"
)

// The advisor is a second model that reviews each turn the main agent takes and
// injects a note when it has something concrete. It is off by default: every
// reviewed turn is an extra model call, so turning it on has to be a decision.
var advisorCmd = &cobra.Command{
	Use:   "advisor",
	Short: "Second-model review of each agent turn",
	Long: `A second model reads each turn the main agent takes and, when it sees
something concrete, injects a note the agent reads on its next turn.

Notes appear as a toast and as a block in the transcript, tagged with a
severity and with explicit guidance to weigh them rather than obey them.

The advisor stays off until both a model and the enable flag are set, because
each reviewed turn costs a second model call.

Project-specific review priorities go in WATCHDOG.md at the repo root (or
.ywai/WATCHDOG.md): things worth flagging to a reviewer but too noisy to put in
the executing agent's own prompt.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return advisorStatus()
	},
}

var advisorStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show whether the advisor is on and which model it uses",
	RunE: func(cmd *cobra.Command, args []string) error {
		return advisorStatus()
	},
}

var advisorModelCmd = &cobra.Command{
	Use:   "model <provider/model>",
	Short: "Set the model the advisor runs on",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		model := strings.TrimSpace(args[0])
		// A bare id cannot be resolved without the catalog. Guessing a provider
		// would review on an unintended model and bill for it silently.
		if i := strings.Index(model, "/"); i <= 0 || i == len(model)-1 {
			return fmt.Errorf("model must be \"provider/model\" (got %q)", model)
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		cfg.AdvisorModel = model
		if err := config.SaveConfig(cfg); err != nil {
			return err
		}

		fmt.Printf("Advisor model set to %s\n", model)
		if !cfg.AdvisorEnabled {
			fmt.Println("The advisor is still off — run `ywai advisor on` to enable it.")
		}
		return nil
	},
}

var advisorOnCmd = &cobra.Command{
	Use:   "on",
	Short: "Enable the advisor",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		if strings.TrimSpace(cfg.AdvisorModel) == "" {
			return fmt.Errorf("no advisor model set — run `ywai advisor model <provider/model>` first")
		}
		cfg.AdvisorEnabled = true
		if err := config.SaveConfig(cfg); err != nil {
			return err
		}
		fmt.Printf("Advisor enabled on %s.\n", cfg.AdvisorModel)
		fmt.Println("Restart OpenCode so it reloads the plugin.")
		return nil
	},
}

var advisorOffCmd = &cobra.Command{
	Use:   "off",
	Short: "Disable the advisor",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		cfg.AdvisorEnabled = false
		if err := config.SaveConfig(cfg); err != nil {
			return err
		}
		fmt.Println("Advisor disabled. Restart OpenCode so it reloads the plugin.")
		return nil
	},
}

func advisorStatus() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	model := strings.TrimSpace(cfg.AdvisorModel)
	switch {
	case model == "":
		fmt.Println("Advisor: off (no model set)")
		fmt.Println("  Set one with: ywai advisor model <provider/model>")
	case !cfg.AdvisorEnabled:
		fmt.Printf("Advisor: off (model %s is configured)\n", model)
		fmt.Println("  Enable with: ywai advisor on")
	default:
		fmt.Printf("Advisor: on, reviewing with %s\n", model)
	}
	return nil
}

func init() {
	advisorCmd.AddCommand(advisorStatusCmd)
	advisorCmd.AddCommand(advisorModelCmd)
	advisorCmd.AddCommand(advisorOnCmd)
	advisorCmd.AddCommand(advisorOffCmd)
	rootCmd.AddCommand(advisorCmd)
}
