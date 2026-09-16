package control

import "testing"

func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v8.8.6", "8.8.8", false},     // the reported bug: older published release
		{"v8.8.8", "8.8.8", false},     // equal
		{"v8.9.0", "8.8.8", true},      // newer minor
		{"v8.8.9", "8.8.8", true},      // newer patch
		{"v9.0.0", "8.8.8", true},      // newer major
		{"8.8.8", "v8.8.8", false},     // mixed prefix, equal
		{"v8.8.10", "8.8.9", true},     // numeric (not lexical) compare
		{"v8.8.8-rc1", "8.8.8", false}, // prerelease is older than its release
		{"v8.24.8", "8.26.0-beta.17", false},
		{"v8.26.0-beta.18", "8.26.0-beta.17", true},
		{"v8.26.0", "8.26.0-beta.17", true},
	}
	for _, c := range cases {
		if got := isNewerVersion(c.latest, c.current); got != c.want {
			t.Errorf("isNewerVersion(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestUpdateArgsFollowsChannel(t *testing.T) {
	if got := updateArgs("8.24.8"); len(got) != 1 || got[0] != "update" {
		t.Errorf("stable args = %v, want [update]", got)
	}
	if got := updateArgs("8.26.0-beta.17"); len(got) != 2 || got[0] != "update" || got[1] != "--beta" {
		t.Errorf("beta args = %v, want [update --beta]", got)
	}
	if got := updateArgs("v8.26.0-rc.1"); len(got) != 2 || got[1] != "--beta" {
		t.Errorf("rc args = %v, want --beta", got)
	}
}
