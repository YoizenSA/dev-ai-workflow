package plugins

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// publishedSlots is the slot tree OpenCode 2 exposes to TUI plugins, read out
// of the shipped binary. A claim on any other path is discarded in silence —
// that is how the logo spent releases targeting "home.logo", rendering nothing
// and reporting nothing.
var publishedSlots = map[string]bool{
	"app":                     true,
	"home.footer":             true,
	"prompt.footer":           true,
	"prompt.footer.file":      true,
	"prompt.footer.location":  true,
	"prompt.footer.status":    true,
	"session.composer.top":    true,
	"session.panel":           true,
	"sidebar.content":         true,
	"sidebar.context":         true,
	"sidebar.footer":          true,
	"sidebar.footer.location": true,
	"sidebar.mcp":             true,
}

// Placement keys a claim may use to name its target slot.
var slotClaimRe = regexp.MustCompile(`(?m)^\s*(replace|prepend|append|before|after):\s*"([^"]+)"`)

func TestTuiPluginsClaimPublishedSlots(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	tuiDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "plugins", "tui")

	entries, err := os.ReadDir(tuiDir)
	if err != nil {
		t.Fatalf("read tui plugins dir: %v", err)
	}

	checked := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".tsx") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(tuiDir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		claims := slotClaimRe.FindAllStringSubmatch(string(data), -1)
		if len(claims) == 0 {
			t.Errorf("%s: no slot claim found; the plugin renders nowhere", e.Name())
			continue
		}
		for _, c := range claims {
			checked++
			if !publishedSlots[c[2]] {
				t.Errorf("%s: claims %q via %s, which the host does not publish — the claim is discarded silently",
					e.Name(), c[2], c[1])
			}
		}
	}
	if checked == 0 {
		t.Fatal("no slot claims checked; the test is not looking where it thinks")
	}
}
