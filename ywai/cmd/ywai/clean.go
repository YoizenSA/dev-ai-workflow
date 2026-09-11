package main

import (
	"fmt"
	"os"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/cleanup"
	"github.com/spf13/cobra"
)

var (
	cleanApply bool
	cleanHome  string
	cleanRepo  string
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove artifacts retired by old ywai installs",
	Long: `One-shot cleanup for machines upgrading from an old ywai release.

Removes: the retired skill-registry and stray .atl indexes, the qa-finder
agent, leftover *.bak agent files (moved to ~/.ywai/agent-backups), broken
legacy plugin bundles, the opencode-quota plugin, orphaned ponytail entries,
subagent-statusline bundles, the retired 'skill-registry refresh' hook, and
the CodeGraph CLI plus its index.

Dry run by default: nothing is modified until --apply is passed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		home := cleanHome
		if home == "" {
			home, _ = os.UserHomeDir()
		}
		repo := cleanRepo
		if repo == "" {
			repo, _ = os.Getwd()
		}
		actions := cleanup.Run(cleanup.Options{Home: home, Repo: repo, Apply: cleanApply})
		for _, a := range actions {
			fmt.Println(a)
		}
		if cleanApply {
			fmt.Printf("\nDone. %d action(s) applied.\n", len(actions))
		} else {
			fmt.Printf("\nDry run: %d action(s) would be applied. Run 'ywai clean --apply' to execute.\n", len(actions))
		}
		return nil
	},
}

func init() {
	cleanCmd.Flags().BoolVar(&cleanApply, "apply", false, "actually remove/move instead of only reporting")
	cleanCmd.Flags().StringVar(&cleanHome, "home", "", "home directory to clean (default: OS home)")
	cleanCmd.Flags().StringVar(&cleanRepo, "repo", "", "repo walked for stray .atl dirs (default: cwd)")
	rootCmd.AddCommand(cleanCmd)
}
