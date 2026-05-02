package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage ywai configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.LoadConfig()
		fmt.Printf("Profiles:\n")
		for name, p := range cfg.Profiles {
			fmt.Printf("  %s: %s\n", name, p.Description)
			if p.Skills != nil {
				fmt.Printf("    skills: %v\n", p.Skills)
			} else {
				fmt.Printf("    skills: all\n")
			}
		}
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a .ywai.yaml in current directory",
	Run: func(cmd *cobra.Command, args []string) {
		cfgPath := ".ywai.yaml"
		if _, err := os.Stat(cfgPath); err == nil {
			fmt.Printf("%s already exists. Use --force to overwrite.\n", cfgPath)
			return
		}

		force, _ := cmd.Flags().GetBool("force")
		if !force {
			cfg := config.LoadConfig()
			defaultCfg := map[string]interface{}{
				"profiles": cfg.Profiles,
			}
			data, _ := yaml.Marshal(defaultCfg)
			if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
				fmt.Fprintf(cmd.OutOrStderr(), "Error: %v\n", err)
				return
			}
		}

		abs, _ := filepath.Abs(cfgPath)
		fmt.Printf("Created %s\n", abs)
	},
}

func init() {
	configInitCmd.Flags().Bool("force", false, "Overwrite existing config")
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configInitCmd)
	rootCmd.AddCommand(configCmd)
}
