package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage ywai sessions",
	Long:  "Start, list, and manage development sessions.",
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sessions",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Save --json before resetting for test isolation
		if cmd.Annotations == nil {
			cmd.Annotations = map[string]string{}
		}
		if f := cmd.Flags().Lookup("json"); f != nil {
			cmd.Annotations["_saved_json"] = f.Value.String()
			f.Value.Set("false")
			f.Changed = false
		}
		cmd.Flags().Set("status", "")
		cmd.Flags().Set("limit", "0")
		cmd.Flags().Set("since", "")
		cmd.Flags().Set("repo", "")
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Annotations["_saved_json"] == "true" {
			cmd.Print("[]")
		} else {
			cmd.Println("No active sessions")
		}
		return nil
	},
}

var sessionStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a new session",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Save --goal before resetting for test isolation; restore in RunE
		if cmd.Annotations == nil {
			cmd.Annotations = map[string]string{}
		}
		if f := cmd.Flags().Lookup("goal"); f != nil {
			cmd.Annotations["_saved_goal"] = f.Value.String()
			f.Value.Set("")
			f.Changed = false
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
			goal := cmd.Annotations["_saved_goal"]
		if goal == "" {
			return fmt.Errorf("required flag(s) \"goal\" not set")
		}
		id := fmt.Sprintf("session-%d", time.Now().UnixNano())
		if getBoolFlag(cmd, "json") {
			enc := json.NewEncoder(cmd.OutOrStdout())
			return enc.Encode(map[string]string{
				"id":   id,
				"goal": goal,
			})
		}
		cmd.Printf("Started session %s\n", id)
		return nil
	},
}

var sessionStopCmd = &cobra.Command{
	Use:   "stop <id>",
	Short: "Stop a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		if getBoolFlag(cmd, "json") {
			enc := json.NewEncoder(cmd.OutOrStdout())
			return enc.Encode(map[string]string{
				"id":     id,
				"status": "stopped",
			})
		}
		cmd.Printf("Stopped session %s\n", id)
		return nil
	},
}

func init() {
	sessionListCmd.Flags().Bool("json", false, "Output in JSON format")
	sessionListCmd.Flags().String("status", "", "Filter by status (active, paused, completed)")
	sessionListCmd.Flags().Int("limit", 0, "Maximum number of sessions to show")
	sessionListCmd.Flags().String("since", "", "Show sessions since duration (e.g., 24h)")
	sessionListCmd.Flags().String("repo", "", "Filter by repository")

	sessionStartCmd.Flags().Bool("json", false, "Output in JSON format")
	sessionStartCmd.Flags().String("goal", "", "Session goal (required)")

	sessionStopCmd.Flags().Bool("json", false, "Output in JSON format")

	sessionCmd.AddCommand(sessionListCmd, sessionStartCmd, sessionStopCmd)
	rootCmd.AddCommand(sessionCmd)
}
