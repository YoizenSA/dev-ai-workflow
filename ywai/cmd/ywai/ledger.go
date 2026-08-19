package main

import (
	"fmt"
	"os"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/ledger"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(ledgerCmd)
	ledgerCmd.AddCommand(ledgerNoteCmd)
	ledgerCmd.AddCommand(ledgerSeamCmd)
	ledgerCmd.AddCommand(ledgerShipCmd)
	ledgerCmd.AddCommand(ledgerResumeCmd)

	ledgerNoteCmd.Flags().String("goal", "", "What done means")
	ledgerNoteCmd.Flags().String("next", "", "The single next action")
	ledgerNoteCmd.Flags().String("core", "", "A live constraint (max 2)")
	ledgerNoteCmd.Flags().Int("core-slot", 0, "1-based core slot to swap (default: append)")
	ledgerNoteCmd.Flags().String("check", "", "What now holds")
	ledgerNoteCmd.Flags().String("by", "", "Verifier and coverage (required with --check / --close)")
	ledgerNoteCmd.Flags().String("open", "", "Open a question")
	ledgerNoteCmd.Flags().String("settled-by", "", "What would settle the open question")
	ledgerNoteCmd.Flags().Int("close", 0, "Close question N against a new --check/--by")
}

var ledgerCmd = &cobra.Command{
	Use:   "ledger",
	Short: "Task-local work ledger",
	Long: `Record long-work state in .ywai/ledger.json of the current directory.

The ledger records; it does not choose solutions. Short work should not use it.

Examples:
  ywai ledger note --goal "what done means" --next "first action"
  ywai ledger note --check "tests pass" --by "go test ./internal/ledger/"
  ywai ledger seam
  ywai ledger ship handoff.md
  ywai ledger resume`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var ledgerNoteCmd = &cobra.Command{
	Use:   "note",
	Short: "Open or update the ledger",
	RunE:  runLedgerNote,
}

var ledgerSeamCmd = &cobra.Command{
	Use:   "seam",
	Short: "Print the current ledger",
	RunE: func(cmd *cobra.Command, args []string) error {
		l, err := loadCwdLedger()
		if err != nil {
			return err
		}
		fmt.Fprint(cmd.OutOrStdout(), ledger.RenderSeam(l))
		return nil
	},
}

var ledgerResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Reload the ledger after a gap",
	RunE: func(cmd *cobra.Command, args []string) error {
		l, err := loadCwdLedger()
		if err != nil {
			return err
		}
		fmt.Fprint(cmd.OutOrStdout(), ledger.RenderResume(l))
		return nil
	},
}

var ledgerShipCmd = &cobra.Command{
	Use:   "ship FILE",
	Short: "Refuse a file that leaked inner-register notes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ledger.ShipFile(args[0])
	},
}

func runLedgerNote(cmd *cobra.Command, args []string) error {
	n := ledger.Note{
		Goal:      flagString(cmd, "goal"),
		Next:      flagString(cmd, "next"),
		Core:      flagString(cmd, "core"),
		CoreSlot:  flagInt(cmd, "core-slot"),
		Check:     flagString(cmd, "check"),
		By:        flagString(cmd, "by"),
		Open:      flagString(cmd, "open"),
		SettledBy: flagString(cmd, "settled-by"),
		Close:     flagInt(cmd, "close"),
	}
	if n.Goal == "" && n.Next == "" && n.Core == "" && n.Check == "" && n.Open == "" && n.Close == 0 {
		return fmt.Errorf("nothing to record: pass --goal, --next, --core, --check, --open, or --close")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	l, err := ledger.Load(cwd)
	if err != nil {
		return err
	}
	if err := ledger.ApplyNote(l, n); err != nil {
		return err
	}
	if err := ledger.Save(cwd, l); err != nil {
		return err
	}
	fmt.Fprint(cmd.OutOrStdout(), ledger.RenderSeam(l))
	return nil
}

func loadCwdLedger() (*ledger.Ledger, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return ledger.Load(cwd)
}

func flagString(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}

func flagInt(cmd *cobra.Command, name string) int {
	v, _ := cmd.Flags().GetInt(name)
	return v
}
