package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/project"
)

var initCmd = &cobra.Command{
	Use:   "init <type>",
	Short: "Initialize a project with AGENTS.md and REVIEW.md",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projectType := args[0]
		targetDir, _ := cmd.Flags().GetString("dir")
		if targetDir == "" {
			targetDir = "."
		}

		if err := project.Init(projectType, targetDir); err != nil {
			fmt.Fprintf(cmd.OutOrStderr(), "Error: %v\n", err)
		}
	},
}

func init() {
	initCmd.Flags().StringP("dir", "d", "", "Target directory (defaults to current directory)")
	rootCmd.AddCommand(initCmd)
}
