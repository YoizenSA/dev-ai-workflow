package main

import "testing"

func TestShouldRunInstallTUI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		tuiFlag   bool
		agentFlag string
		dryRun    bool
		want      bool
	}{
		{
			name: "bare install opens TUI even when global defaults true",
			want: true,
		},
		{
			name:    "force --tui",
			tuiFlag: true,
			want:    true,
		},
		{
			name:      "agent flag skips TUI",
			agentFlag: "opencode",
			want:      false,
		},
		{
			name:   "dry-run skips TUI",
			dryRun: true,
			want:   false,
		},
		{
			name:      "--tui wins over agent flag",
			tuiFlag:   true,
			agentFlag: "opencode",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := shouldRunInstallTUI(tt.tuiFlag, tt.agentFlag, tt.dryRun)
			if got != tt.want {
				t.Fatalf("shouldRunInstallTUI(%v, %q, %v) = %v, want %v",
					tt.tuiFlag, tt.agentFlag, tt.dryRun, got, tt.want)
			}
		})
	}
}
