package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultOrchestratorModelProfiles_SeedsThreeProfiles(t *testing.T) {
	profiles := DefaultOrchestratorModelProfiles()

	if len(profiles) != 4 {
		t.Fatalf("expected 4 seeded orchestrator profiles, got %d", len(profiles))
	}

	for _, name := range []string{"balanced", "fast", "deep"} {
		profile, ok := profiles[name]
		if !ok {
			t.Fatalf("expected seeded profile %q to exist; profiles=%v", name, profiles)
		}
		if profile.DisplayName == "" {
			t.Fatalf("expected profile %q to have a display name", name)
		}
		// Profiles are keyed by agent name; spot-check core agents.
		for _, agent := range []string{"dev", "qa", "reviewer"} {
			if profile.Agents[agent].Model == "" {
				t.Fatalf("expected profile %q to define a model for agent %q", name, agent)
			}
		}
	}
}

func TestInheritProfileHasNoPinnedModels(t *testing.T) {
	p, ok := DefaultOrchestratorModelProfiles()["inherit"]
	if !ok {
		t.Fatal("expected shipped profile inherit")
	}
	if p.DisplayName == "" {
		t.Fatal("inherit needs a display name")
	}
	for name, rd := range p.Agents {
		if strings.TrimSpace(rd.Model) != "" {
			t.Errorf("inherit must not pin %s to %q", name, rd.Model)
		}
	}
	for role, model := range p.OmpModelRoles {
		if strings.TrimSpace(model) != "" {
			t.Errorf("inherit must not pin omp role %s to %q", role, model)
		}
	}
}

func TestDefaultOrchestratorModelProfiles_FastUsesFlashEverywhere(t *testing.T) {
	profiles := DefaultOrchestratorModelProfiles()

	got := profiles["fast"].Agents["dev"]
	if got.Model != "opencode-admin/deepseek-v4-flash" {
		t.Fatalf("expected fast dev model opencode-admin/deepseek-v4-flash, got %q", got.Model)
	}
}

// TestSeedProfilesShipOrchestration pins the orchestration blocks of the three
// shipped profiles (design doc "orchestrator-execution-modes.md" §4).
func TestSeedProfilesShipOrchestration(t *testing.T) {
	profiles := DefaultOrchestratorModelProfiles()

	cases := []struct {
		name      string
		mode      string
		soloWrite bool
		review    string
		hops      int
	}{
		{"fast", "solo", true, "never", 1},
		{"balanced", "thin", true, "on_ship", 1},
		{"deep", "full", false, "always", 2},
	}
	for _, tc := range cases {
		p, ok := profiles[tc.name]
		if !ok {
			t.Fatalf("missing profile %q", tc.name)
		}
		pol := p.Orchestration
		if pol.DefaultMode != tc.mode {
			t.Errorf("%s default_mode = %q, want %q", tc.name, pol.DefaultMode, tc.mode)
		}
		if pol.SoloWriteAllowed() != tc.soloWrite {
			t.Errorf("%s allow_solo_write = %v, want %v", tc.name, pol.SoloWriteAllowed(), tc.soloWrite)
		}
		if pol.RequireReview != tc.review {
			t.Errorf("%s require_review = %q, want %q", tc.name, pol.RequireReview, tc.review)
		}
		if pol.MaxHopsBeforeEscalate == nil || *pol.MaxHopsBeforeEscalate != tc.hops {
			t.Errorf("%s max_hops_before_escalate = %v, want %d", tc.name, pol.MaxHopsBeforeEscalate, tc.hops)
		}
		if len(pol.EscalateOn) == 0 {
			t.Errorf("%s escalate_on must not be empty", tc.name)
		}
	}
}

// TestOrchestrationPolicyNormalize pins the fallback for profiles that ship no
// orchestration block (user-created profiles keep working unchanged).
func TestOrchestrationPolicyNormalize(t *testing.T) {
	got := (OrchestrationPolicy{}).Normalize()
	if got.DefaultMode != "thin" {
		t.Errorf("default_mode = %q, want thin", got.DefaultMode)
	}
	if !got.SoloWriteAllowed() {
		t.Error("allow_solo_write must default to true")
	}
	if got.MaxHopsBeforeEscalate == nil || *got.MaxHopsBeforeEscalate != 1 {
		t.Errorf("max_hops_before_escalate = %v, want 1", got.MaxHopsBeforeEscalate)
	}
	if got.RequireReview != "on_ship" {
		t.Errorf("require_review = %q, want on_ship", got.RequireReview)
	}
	if !reflect.DeepEqual(got.EscalateOn, DefaultEscalateOn) {
		t.Errorf("escalate_on = %v, want %v", got.EscalateOn, DefaultEscalateOn)
	}

	false_ := false
	deny := OrchestrationPolicy{DefaultMode: "full", AllowSoloWrite: &false_}.Normalize()
	if deny.SoloWriteAllowed() {
		t.Error("explicit allow_solo_write=false must stay false after Normalize")
	}
	if deny.RequireReview != "on_ship" {
		t.Errorf("require_review should still be defaulted, got %q", deny.RequireReview)
	}

	// An explicit 0 hops is a deliberate choice ("escalate on any delegation")
	// and must survive Normalize instead of being treated as unset.
	zero := 0
	explicitZero := OrchestrationPolicy{MaxHopsBeforeEscalate: &zero}.Normalize()
	if explicitZero.MaxHopsBeforeEscalate == nil || *explicitZero.MaxHopsBeforeEscalate != 0 {
		t.Errorf("explicit max_hops_before_escalate=0 must survive Normalize, got %v", explicitZero.MaxHopsBeforeEscalate)
	}
}

// TestOrchestrationPolicyCloneDeepCopies guards against aliasing: mutating a
// clone's EscalateOn / AllowSoloWrite / MaxHopsBeforeEscalate must not touch
// the original.
func TestOrchestrationPolicyCloneDeepCopies(t *testing.T) {
	p := DefaultOrchestrationPolicy()
	c := p.Clone()
	c.EscalateOn[0] = "changed"
	c.AllowSoloWrite = nil
	c.MaxHopsBeforeEscalate = nil

	if p.EscalateOn[0] == "changed" {
		t.Error("clone EscalateOn aliases the original slice")
	}
	if p.AllowSoloWrite == nil {
		t.Error("clone AllowSoloWrite aliases the original pointer")
	}
	if p.MaxHopsBeforeEscalate == nil {
		t.Error("clone MaxHopsBeforeEscalate aliases the original pointer")
	}
}

// The shipped profiles (balanced, fast, deep) are product defaults, not user
// data: they gain agents as agents are added, so an install must restore them.
// A shipped profile is rebuilt from the seed (so new agents reach it) but the
// user's per-agent model overrides and OMP modelRoles survive. A setup you
// want fully your own lives under a name we do not ship, and passes through
// untouched.
func TestShippedProfilesMergeUserOverrides_NewAgentsAppear(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))

	seed := DefaultOrchestratorModelProfiles()

	cfg := DefaultConfig()
	cfg.ActiveOrchestratorProfile = "fast"
	cfg.OrchestratorProfiles = DefaultOrchestratorModelProfiles()
	fast := cfg.OrchestratorProfiles["fast"]
	fast.Agents["dev"] = RoleDefault{Model: "opencode-admin/edited-shipped"}
	fast.OmpModelRoles = map[string]string{"default": "opencode-go/deepseek-v4-flash"}
	cfg.OrchestratorProfiles["fast"] = fast
	// ...and a profile of the user's own.
	cfg.OrchestratorProfiles["my-setup"] = OrchestratorModelProfile{
		Agents: RoleDefaults{"dev": {Model: "opencode-admin/mine"}},
	}

	if err := os.MkdirAll(DataDir(), 0o755); err != nil {
		t.Fatalf("create data dir: %v", err)
	}
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	fastLoaded := loaded.OrchestratorProfiles["fast"]
	if got := fastLoaded.Agents["dev"].Model; got != "opencode-admin/edited-shipped" {
		t.Errorf("shipped profile model override must survive reinstall, got %q", got)
	}
	if got := fastLoaded.Agents["qa"].Model; got != seed["fast"].Agents["qa"].Model {
		t.Errorf("seed agent not overridden must keep the seed model, got %q want %q", got, seed["fast"].Agents["qa"].Model)
	}
	if fastLoaded.OmpModelRoles["default"] != "opencode-go/deepseek-v4-flash" {
		t.Errorf("OMP modelRoles override must survive, got %q", fastLoaded.OmpModelRoles["default"])
	}
	if got := loaded.OrchestratorProfiles["my-setup"].Agents["dev"].Model; got != "opencode-admin/mine" {
		t.Errorf("a custom profile must survive untouched, got %q", got)
	}
}

func TestIsShippedProfile(t *testing.T) {
	for _, name := range []string{"balanced", "fast", "deep", "inherit"} {
		if !IsShippedProfile(name) {
			t.Errorf("%s is shipped and gets overwritten — the UI needs to say so", name)
		}
	}
	if IsShippedProfile("my-setup") {
		t.Error("a user profile must not be reported as shipped")
	}
}

func TestResyncOrchestratorModelProfiles_RestoresSeedsAndPreservesValidActiveProfile(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ActiveOrchestratorProfile = "fast"
	cfg.OrchestratorProfiles = DefaultOrchestratorModelProfiles()
	cfg.OrchestratorProfiles["fast"].Agents["dev"] = RoleDefault{Model: "opencode-admin/custom-model"}

	cfg.ResyncOrchestratorModelProfiles()

	if cfg.ActiveOrchestratorProfile != "fast" {
		t.Fatalf("expected valid active profile to be preserved, got %q", cfg.ActiveOrchestratorProfile)
	}
	got := cfg.OrchestratorProfiles["fast"].Agents["dev"]
	seed := DefaultOrchestratorModelProfiles()["fast"].Agents["dev"]
	if !reflect.DeepEqual(got, seed) {
		t.Fatalf("expected resync to restore seeded fast dev mapping, got %+v want %+v", got, seed)
	}
}

func TestResyncOrchestratorModelProfiles_FallsBackDeterministicallyWhenActiveProfileRemoved(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ActiveOrchestratorProfile = "experimental"
	cfg.OrchestratorProfiles = map[string]OrchestratorModelProfile{
		"experimental": {
			DisplayName: "Experimental",
			Agents: RoleDefaults{
				"dev": {Model: "opencode-admin/custom-model"},
			},
		},
	}

	cfg.ResyncOrchestratorModelProfiles()

	if cfg.ActiveOrchestratorProfile != DefaultOrchestratorModelProfileName {
		t.Fatalf("expected missing active profile to fall back to %q, got %q", DefaultOrchestratorModelProfileName, cfg.ActiveOrchestratorProfile)
	}
	if _, ok := cfg.OrchestratorProfiles["experimental"]; ok {
		t.Fatalf("expected resync to drop non-seed experimental profile")
	}
}

func TestGetOrchestratorAgentModel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ActiveOrchestratorProfile = "fast"
	if got := cfg.GetOrchestratorAgentModel("dev"); got != "opencode-admin/deepseek-v4-flash" {
		t.Fatalf("expected fast dev model, got %q", got)
	}
	if got := cfg.GetOrchestratorAgentModel("nonexistent-agent"); got != "" {
		t.Fatalf("expected empty model for unknown agent, got %q", got)
	}
}

// TestDefaultOrchestratorModelProfiles_BalancedUsesDeepSeek pins the balanced
// profile's model assignments after the DeepSeek migration: every Grok and
// MiniMax assignment was replaced with deepseek-v4-pro / deepseek-v4-flash.
func TestDefaultOrchestratorModelProfiles_BalancedUsesDeepSeek(t *testing.T) {
	profiles := DefaultOrchestratorModelProfiles()
	balanced, ok := profiles["balanced"]
	if !ok {
		t.Fatalf("expected seeded profile %q to exist", "balanced")
	}

	roleWant := map[string]string{
		"advisor": "opencode-admin/deepseek-v4-pro",
		"plan":    "opencode-admin/deepseek-v4-pro",
		"default": "opencode-admin/deepseek-v4-flash",
	}
	for role, want := range roleWant {
		if got := balanced.OmpModelRoles[role]; got != want {
			t.Errorf("balanced omp_model_roles[%q] = %q, want %q", role, got, want)
		}
	}

	agentWant := map[string]string{
		"advisor":       "opencode-admin/deepseek-v4-pro",
		"architect":     "opencode-admin/deepseek-v4-pro",
		"orchestrator":  "opencode-admin/deepseek-v4-pro",
		"planner-draft": "opencode-admin/deepseek-v4-pro",
		"planning":      "opencode-admin/deepseek-v4-pro",
		"dev":           "opencode-admin/deepseek-v4-flash",
		"finder":        "opencode-admin/deepseek-v4-flash",
		"qa":            "opencode-admin/deepseek-v4-flash",
		"ask":           "opencode-admin/deepseek-v4-flash",
	}
	for agent, want := range agentWant {
		if got := balanced.Agents[agent].Model; got != want {
			t.Errorf("balanced agent %q model = %q, want %q", agent, got, want)
		}
	}

	// No Grok or MiniMax may remain anywhere in the balanced profile.
	for role, model := range balanced.OmpModelRoles {
		if strings.Contains(model, "grok") || strings.Contains(model, "minimax") {
			t.Errorf("balanced omp_model_roles[%q] still references replaced model %q", role, model)
		}
	}
	for agent, def := range balanced.Agents {
		if strings.Contains(def.Model, "grok") || strings.Contains(def.Model, "minimax") {
			t.Errorf("balanced agent %q still references replaced model %q", agent, def.Model)
		}
	}

	// The deep profile is intentionally untouched: its slow alias stays Grok.
	deep, ok := profiles["deep"]
	if !ok {
		t.Fatalf("expected seeded profile %q to exist", "deep")
	}
	if got := deep.OmpModelRoles["slow"]; got != "opencode-admin/grok-4.5" {
		t.Errorf("deep omp_model_roles[slow] changed to %q; deep profile must stay untouched", got)
	}
}
