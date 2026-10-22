package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/plugins"
	"github.com/spf13/cobra"
)

var compactTokensCmd = &cobra.Command{
	Use:   "compact-tokens [tokens]",
	Short: "Show or set the jev-compaction token threshold",
	Long: `Show or set the history size, in estimated tokens, above which the
jev-compaction plugin asks Jev to drop stale tool calls (default 200000).

Stored as compactionThresholdTokens in ~/.ywai/jev-gate.json; the API key in
that file is kept. A project .opencode/jev-gate.json or the
JEV_COMPACTION_THRESHOLD_TOKENS env var still take precedence.

  ywai compact-tokens            # show
  ywai compact-tokens 1000000    # set (1m and 500k also work)`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			n, set := plugins.CompactionThreshold()
			src := "default"
			if set {
				src = plugins.JevKeyPath()
			}
			fmt.Printf("%d (%s)\n", n, src)
			return nil
		}
		n, err := parseTokens(args[0])
		if err != nil {
			return err
		}
		if err := plugins.SetCompactionThreshold(n); err != nil {
			return err
		}
		fmt.Printf("compaction threshold set to %d in %s\n", n, plugins.JevKeyPath())
		return nil
	},
}

// parseTokens accepts 1000000, 1_000_000, 500k or 1m.
func parseTokens(raw string) (int, error) {
	s := strings.ToLower(strings.ReplaceAll(raw, "_", ""))
	mult := 1
	switch {
	case strings.HasSuffix(s, "k"):
		mult, s = 1000, strings.TrimSuffix(s, "k")
	case strings.HasSuffix(s, "m"):
		mult, s = 1000000, strings.TrimSuffix(s, "m")
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid token count %q (try 1000000, 500k or 1m)", raw)
	}
	return int(n * float64(mult)), nil
}

func init() {
	rootCmd.AddCommand(compactTokensCmd)
}
