package gentlai

import "testing"

func TestParseVersion_Semver(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"v1.2.3", "1.2.3"},
		{"1.2.3", "1.2.3"},
		{"gentle-ai version v0.5.0", "0.5.0"},
		{"v1.0.0-beta.1", "1.0.0-beta.1"},
		{"no version here", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := parseVersion(tt.input)
		if got != tt.want {
			t.Errorf("parseVersion(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
