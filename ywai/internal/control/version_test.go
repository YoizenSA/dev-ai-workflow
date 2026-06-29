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
		{"v8.8.8-rc1", "8.8.8", false}, // suffix ignored → equal
	}
	for _, c := range cases {
		if got := isNewerVersion(c.latest, c.current); got != c.want {
			t.Errorf("isNewerVersion(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}
