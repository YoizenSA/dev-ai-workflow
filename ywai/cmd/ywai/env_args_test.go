package main

import (
	"slices"
	"testing"
)

func TestParseEnvRunArgs(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantArgv []string
		wantErr  bool
	}{
		{"colon form", []string{"hola", "--model:opencode-go/glm-5.3-flash", "--agent:ask"},
			[]string{"run", "--model", "opencode-go/glm-5.3-flash", "--agent", "ask", "hola"}, false},
		{"space and equals, words joined", []string{"--model", "zai/glm-5.3", "-a=ask", "hola", "mundo"},
			[]string{"run", "--model", "zai/glm-5.3", "--agent", "ask", "hola mundo"}, false},
		{"short -m and --auto", []string{"-m", "x/y", "fix it", "--auto"},
			[]string{"run", "--model", "x/y", "--auto", "fix it"}, false},
		{"double dash keeps literal text", []string{"hola", "--", "--model", "literal"},
			[]string{"run", "hola --model literal"}, false},
		{"passthrough value flag", []string{"describe", "--file", "a.png"},
			[]string{"run", "--file", "a.png", "describe"}, false},
		{"tui continue", []string{"-c"}, []string{"-c"}, false},
		{"tui plain", nil, nil, false},
		{"model without prompt", []string{"--model:x/y"}, nil, true},
		{"missing value", []string{"hola", "--agent"}, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prompt, opts, err := parseEnvRunArgs(tc.args)
			if err == nil {
				var argv []string
				argv, err = envOpencodeArgv(prompt, opts)
				if err == nil && !slices.Equal(argv, tc.wantArgv) {
					t.Fatalf("argv = %q, want %q", argv, tc.wantArgv)
				}
			}
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestWantsHelp(t *testing.T) {
	if !wantsHelp([]string{"--help"}) || !wantsHelp([]string{"x", "-h"}) {
		t.Error("help flag not detected")
	}
	if wantsHelp([]string{"--", "--help"}) {
		t.Error("--help after -- is prompt text, not a help request")
	}
}
