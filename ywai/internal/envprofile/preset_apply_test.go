package envprofile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPresetSpecEmpty(t *testing.T) {
	spec, err := PresetSpec(Profile{Name: "x"})
	if err != nil {
		t.Fatalf("PresetSpec empty = %v, want nil", err)
	}
	if len(spec) != 0 {
		t.Errorf("PresetSpec empty = %v, want empty", spec)
	}
	if IsBare(Profile{Name: "x"}) {
		t.Errorf("IsBare empty preset = true, want false")
	}
}

func TestPresetSpecKnown(t *testing.T) {
	for _, name := range []string{"dev", "qa", "personal"} {
		spec, err := PresetSpec(Profile{Name: "x", Preset: name})
		if err != nil {
			t.Fatalf("PresetSpec(%q) = %v", name, err)
		}
		if len(spec) == 0 {
			t.Errorf("PresetSpec(%q) empty, want content", name)
		}
	}
	if !IsBare(Profile{Name: "x", Preset: "personal"}) {
		t.Errorf("IsBare(personal) = false, want true")
	}
	if IsBare(Profile{Name: "x", Preset: "dev"}) {
		t.Errorf("IsBare(dev) = true, want false")
	}
	if _, err := PresetSpec(Profile{Name: "x", Preset: "nope"}); err == nil {
		t.Errorf("PresetSpec(nope) = nil, want error")
	}
}

func TestPresetGetters(t *testing.T) {
	spec, err := PresetSpec(Profile{Name: "x", Preset: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if groups := PresetGroups(spec); len(groups) != 2 || groups[0] != "core" {
		t.Errorf("PresetGroups(dev) = %v", groups)
	}
	// dev/qa install ALL skills like a global install: empty skills allowlist
	// means no filter (keep current behavior). MCPs are an explicit
	// allowlist per user request (graft + grafana + chrome-devtools).
	if skills := PresetSkills(spec); skills != nil {
		t.Errorf("PresetSkills(dev) = %v, want nil (all)", skills)
	}
	if mcp := PresetMCPIDs(spec); len(mcp) != 3 || mcp[0] != "graft" || mcp[1] != "grafana" || mcp[2] != "chrome-devtools" {
		t.Errorf("PresetMCPIDs(dev) = %v, want [graft grafana chrome-devtools]", mcp)
	}
	if deny := PresetDenyBash(spec); deny != nil {
		t.Errorf("PresetDenyBash(dev) = %v, want nil (commit/push is each agent's bash permission)", deny)
	}
	if !ShouldStripCommitDenies("orchestrator") {
		t.Error("daily-dev must strip stale git commit/push denials")
	}
	if !ShouldStripCommitDenies("qa-orchestrator") {
		t.Error("QA lane must strip stale git commit/push denials")
	}
	if ShouldStripCommitDenies("reviewer") {
		t.Error("code-review must keep the commit/push lane pack")
	}
	qaSpec, err := PresetSpec(Profile{Name: "x", Preset: "qa"})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range PresetDenyBash(qaSpec) {
		if strings.Contains(p, "git commit") || strings.Contains(p, "git push") {
			t.Errorf("qa deny_bash still has %q; commit/push is each agent's bash permission", p)
		}
	}
	crSpec, err := PresetSpec(Profile{Name: "x", Preset: "code-review"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(PresetDenyBash(crSpec), " "); !strings.Contains(got, "git commit") {
		t.Errorf("code-review deny_bash missing git commit, got %v", PresetDenyBash(crSpec))
	}
	if got := PresetDefaultAgent(spec); got != "orchestrator" {
		t.Errorf("PresetDefaultAgent(dev) = %q", got)
	}
	if got := PresetDefaultModel(spec); got == "" {
		t.Errorf("PresetDefaultModel(dev) empty, want value")
	}
	if got := PresetCliTheme(spec); got == "" {
		t.Errorf("PresetCliTheme(dev) empty, want value")
	}
	if got := PresetGroups(map[string]any{}); got != nil {
		t.Errorf("PresetGroups(empty) = %v, want nil", got)
	}
}

// The devops lane pack is a curated blocklist: every entry must be a glob
// (trailing *) so it matches with arguments, and no entry may repeat.
func TestDevopsDenyBashNormalized(t *testing.T) {
	spec, err := PresetSpec(Profile{Name: "x", Preset: "devops"})
	if err != nil {
		t.Fatal(err)
	}
	deny := PresetDenyBash(spec)
	if len(deny) == 0 {
		t.Fatal("devops deny_bash empty, want the lane pack")
	}
	seen := map[string]bool{}
	for _, d := range deny {
		if !strings.HasSuffix(d, "*") {
			t.Errorf("devops deny_bash entry %q has no trailing glob", d)
		}
		if seen[d] {
			t.Errorf("devops deny_bash entry %q duplicated", d)
		}
		seen[d] = true
	}
	for _, want := range []string{"kubectl delete*", "helm uninstall*", "helm update*", "helm install*", "terraform destroy*", "terraform apply*", "git push --force*", "az group delete*", "az aks get-credentials*", "Remove-Az*"} {
		if !seen[want] {
			t.Errorf("devops deny_bash missing %q", want)
		}
	}
}

func TestShouldInstallMCP(t *testing.T) {
	if !ShouldInstallMCP("graft", nil) {
		t.Errorf("empty allowlist must allow")
	}
	allow := []string{"graft", "context7"}
	if !ShouldInstallMCP("graft", allow) {
		t.Errorf("graft should install")
	}
	if ShouldInstallMCP("chrome-devtools", allow) {
		t.Errorf("chrome-devtools should skip")
	}
	if !IsMCPServerManifestID("chrome-devtools") {
		t.Errorf("chrome-devtools must be an MCP manifest id")
	}
	if IsMCPServerManifestID("background-agents") {
		t.Errorf("background-agents is a plugin, not an MCP server")
	}
}

func TestAppendDenyBashToAgents(t *testing.T) {
	dir := t.TempDir()
	body := "---\ndescription: dev\nmode: primary\npermissions:\n  - action: shell\n    resource: \"*\"\n    effect: allow\n---\n\nPrompt.\n"
	path := filepath.Join(dir, "dev.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := AppendDenyBashToAgents(dir, []string{"git commit*", "git push*"}, nil)
	if err != nil {
		t.Fatalf("AppendDenyBashToAgents: %v", err)
	}
	if n != 1 {
		t.Fatalf("changed = %d, want 1", n)
	}
	data, _ := os.ReadFile(path)
	for _, p := range []string{"git commit*", "git push*"} {
		if !strings.Contains(string(data), p) {
			t.Errorf("pattern %q missing after append", p)
		}
	}
	// Idempotent: second run changes nothing.
	n, err = AppendDenyBashToAgents(dir, []string{"git commit*"}, nil)
	if err != nil || n != 0 {
		t.Errorf("second run = (%d, %v), want (0, nil)", n, err)
	}
}

func TestAppendDenyBashNoPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plain.md")
	if err := os.WriteFile(path, []byte("---\ndescription: x\nmode: all\n---\n\nBody.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := AppendDenyBashToAgents(dir, []string{"git commit*"}, nil)
	if err != nil || n != 0 {
		t.Errorf("no-permissions file = (%d, %v), want (0, nil)", n, err)
	}
}
