package cmd

import (
	"fmt"
	"os"

	"github.com/yoizen/ga/internal/cache"
	"github.com/yoizen/ga/internal/config"
	"github.com/yoizen/ga/internal/git"
	"github.com/yoizen/ga/internal/providers"
	"github.com/yoizen/ga/internal/review"
	"github.com/yoizen/ga/internal/ui"

	"github.com/spf13/cobra"
)

var (
	noCache       bool
	ciMode        bool
	prMode        bool
	diffOnly      bool
	commitMsgFile string
	phase         string
)

var runCmd = &cobra.Command{
	Use:   "run [commit-msg-file]",
	Short: "Run code review on staged files",
	Long: `Run code review on staged files using the configured AI provider.
Supports multiple modes: staged (default), CI, and PR mode.`,
	Args: cobra.MaximumNArgs(1),
	Aliases: []string{"review"},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 && commitMsgFile == "" {
			commitMsgFile = args[0]
		}
		return runReview(ReviewConfig{
			NoCache:       noCache,
			CIMode:        ciMode,
			PRMode:        prMode,
			DiffOnly:      diffOnly,
			CommitMsgFile: commitMsgFile,
			Phase:         phase,
		})
	},
}

func init() {
	runCmd.Flags().BoolVar(&noCache, "no-cache", false, "Force review all files, ignoring cache")
	runCmd.Flags().BoolVar(&ciMode, "ci", false, "CI mode: review files changed in last commit")
	runCmd.Flags().BoolVar(&prMode, "pr-mode", false, "PR mode: review all files changed in the full PR")
	runCmd.Flags().BoolVar(&diffOnly, "diff-only", false, "With --pr-mode: send only diffs (faster, cheaper)")
	runCmd.Flags().StringVar(&commitMsgFile, "commit-msg-file", "", "Path to commit message file (for commit-msg hook)")
	runCmd.Flags().StringVar(
		&phase,
		"phase",
		"",
		"SDD phase for model resolution (e.g., sdd-apply, sdd-design). "+
			"Reads from .ywai/sdd-models.json",
	)
	_ = runCmd.Flags().MarkHidden("commit-msg-file")
	rootCmd.AddCommand(runCmd)
}

// ReviewConfig holds configuration for review execution
type ReviewConfig struct {
	NoCache       bool
	CIMode        bool
	PRMode        bool
	DiffOnly      bool
	CommitMsgFile string
	Phase         string
}

func runReview(reviewCfg ReviewConfig) error {
	ui.PrintBanner("dev")

	cfg, err := config.Load()
	if err != nil {
		ui.Error("Failed to load config: %v", err)
		return err
	}

	if cfg.Provider == "" {
		ui.Error("No provider configured")
		ui.Info("Configure a provider in .ga or set GA_PROVIDER")
		ui.Info("Run 'ga init' to create a config file")
		return nil
	}

	if err := providers.ValidateProvider(cfg.Provider); err != nil {
		ui.Error("Provider validation failed: %v", err)
		return err
	}

	if _, err := os.Stat(cfg.RulesFile); os.IsNotExist(err) {
		ui.Error("Rules file not found: %s", cfg.RulesFile)
		ui.Info("Please create a %s file with your coding standards.", cfg.RulesFile)
		return err
	}

	ui.Info("Provider: %s", cfg.Provider)
	ui.Info("Rules file: %s", cfg.RulesFile)
	ui.Info("File patterns: %s", cfg.FilePatterns)
	if cfg.ExcludePatterns != "" {
		ui.Info("Exclude patterns: %s", cfg.ExcludePatterns)
	}

	files := []string{}
	var prRange string
	var baseBranch string

	if reviewCfg.PRMode {
		if reviewCfg.DiffOnly {
			ui.Info("Mode: PR (diff-only review)")
		} else {
			ui.Info("Mode: PR (full file review)")
		}

		baseBranch = cfg.PRBaseBranch
		if baseBranch == "" {
			var err error
			baseBranch, err = git.DetectBaseBranch()
			if err != nil {
				ui.Error("Failed to detect base branch: %v", err)
				ui.Info("Set PR_BASE_BRANCH in your .ga config")
				return err
			}
		}
		if baseBranch == "" {
			ui.Error("Could not detect base branch")
			ui.Info("Set PR_BASE_BRANCH in your .ga config")
			return fmt.Errorf("base branch not detected")
		}
		prRange = baseBranch + "...HEAD"
		ui.Info("PR range: %s", prRange)
		files, err = git.GetPRFiles(prRange, cfg.FilePatterns, cfg.ExcludePatterns)
		if err != nil {
			ui.Error("Failed to get PR files: %v", err)
			return err
		}
		return nil
	}

	if reviewCfg.CIMode {
		ui.Info("Mode: CI (reviewing last commit)")
		files, err = git.GetCIFiles(cfg.FilePatterns, cfg.ExcludePatterns)
		if err != nil {
			ui.Error("Failed to get CI files: %v", err)
			return err
		}
		return nil
	}

	ui.Info("Mode: Staged files")
	files, err = git.GetStagedFiles(cfg.FilePatterns, cfg.ExcludePatterns)
	if err != nil {
		ui.Error("Failed to get staged files: %v", err)
		return err
	}

	useCache := !reviewCfg.NoCache && !reviewCfg.CIMode && !reviewCfg.PRMode
	if useCache {
		ui.Info("Cache: enabled")
	} else {
		ui.Info("Cache: disabled")
	}

	if len(files) == 0 {
		ui.Success("No files to review")
		return nil
	}

	ui.Info("Files to review: %d", len(files))
	for _, f := range files {
		ui.Info("  - %s", f)
	}

	// Handle cache
	var filesToReview []string
	if useCache {
		// Check cache validity atomically
		if !cache.IsCacheValid(cfg.RulesFile, ".ga") {
			ui.Info("Cache invalidated (rules or config changed)")
			if err := cache.InvalidateCache(); err != nil {
				ui.Warning("Failed to invalidate cache: %v", err)
			}
		}

		if _, err := cache.InitCache(cfg.RulesFile, ".ga"); err != nil {
			ui.Warning("Failed to initialize cache: %v", err)
			// Continue without cache if initialization fails
			filesToReview = files
		} else {
			// Filter files that need review (not cached)
			for _, f := range files {
				if !cache.IsFileCached(f) {
					filesToReview = append(filesToReview, f)
				}
			}
			cachedCount := len(files) - len(filesToReview)
			if cachedCount > 0 {
				ui.Success("%d file(s) passed from cache", cachedCount)
			}
		}
	} else {
		filesToReview = files
	}

	if len(filesToReview) == 0 {
		ui.Success("All files passed from cache!")
		ui.Success("CODE REVIEW PASSED (cached)")
		return nil
	}

	// Build prompt
	var prompt string
	var commitMsg string

	if reviewCfg.CommitMsgFile != "" {
		if data, err := os.ReadFile(reviewCfg.CommitMsgFile); err == nil {
			commitMsg = string(data)
		}
	}

	if reviewCfg.PRMode {
		var prDiff string
		if reviewCfg.DiffOnly {
			var err error
			prDiff, err = git.GetPRDiff(prRange)
			if err != nil {
				ui.Warning("Failed to get PR diff: %v", err)
				ui.Info("Falling back to full file review")
				reviewCfg.DiffOnly = false
			}
		}
		prompt = review.BuildPRPrompt(review.PRPromptConfig{
			RulesFile:  cfg.RulesFile,
			PRDiff:     prDiff,
			Files:      filesToReview,
			DiffOnly:   reviewCfg.DiffOnly,
			BaseBranch: baseBranch,
		})
	} else {
		useStaged := !reviewCfg.CIMode
		prompt = review.BuildPrompt(cfg.RulesFile, commitMsg, filesToReview, useStaged)
	}

	// Execute provider
	ui.Info("Sending to %s for review (timeout: %ds)...", cfg.Provider, cfg.Timeout)

	provider, err := providers.NewProvider(&providers.Config{
		Provider: cfg.Provider,
		Model:    "",
		Timeout:  cfg.Timeout,
		Phase:    reviewCfg.Phase,
	})
	if err != nil {
		ui.Error("Failed to create provider: %v", err)
		return err
	}

	result, err := provider.Execute(prompt)
	if err != nil {
		ui.Error("Provider execution failed: %v", err)
		if cfg.StrictMode {
			return err
		}
		return nil
	}

	fmt.Println(result)
	fmt.Println("")

	// Parse result
	reviewResult := review.ParseResult(result)

	if reviewResult == review.ResultPassed {
		if useCache {
			for _, f := range filesToReview {
				if err := cache.CacheFileResult(f, "PASSED"); err != nil {
					ui.Warning("Failed to cache result for %s: %v", f, err)
				}
			}
		}
		ui.Success("CODE REVIEW PASSED")
		return nil
	} else if reviewResult == review.ResultFailed {
		ui.Error("CODE REVIEW FAILED")
		ui.Info("Fix the violations listed above before committing.")
		return fmt.Errorf("code review failed")
	} else {
		ui.Warning("Could not determine review status")
		if cfg.StrictMode {
			ui.Error("STRICT MODE: Failing due to ambiguous response")
			ui.Info("Expected 'STATUS: PASSED' or 'STATUS: FAILED' in the response")
			return fmt.Errorf("ambiguous response")
		}
		ui.Warning("Allowing commit (STRICT_MODE=false)")
		return nil
	}
}
