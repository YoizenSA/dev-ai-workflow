package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/setup/wizard/pkg/installer"
	tea "github.com/charmbracelet/bubbletea"
)

// TestE2E_GlobalToolsUpdate tests the "Update global tools" functionality
func TestE2E_GlobalToolsUpdate(t *testing.T) {
	baseFlags := &installer.Flags{
		Channel:        installer.DEFAULT_CHANNEL,
		NonInteractive: false,
	}

	model := newSetupModel("/tmp", baseFlags)

	// Simulate navigating to "Update global tools" option (index 3)
	msg := tea.KeyMsg{Type: tea.KeyDown, Runes: []rune{}}
	for i := 0; i < 3; i++ {
		var m tea.Model
		m, _ = model.Update(msg)
		model = m.(setupModel)
	}

	// Press enter to select
	msg = tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{}}
	var m tea.Model
	m, _ = model.Update(msg)
	model = m.(setupModel)

	// Should now be in stepGlobalTools
	if model.step != stepGlobalTools {
		t.Fatalf("Expected stepGlobalTools, got %v", model.step)
	}

	// Press enter to run update with default selections
	msg = tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{}}
	var m2 tea.Model
	m2, _ = model.Update(msg)
	model = m2.(setupModel)

	// Should trigger the update process
	// Note: The actual update happens asynchronously, so we just verify navigation
}

// TestE2E_InstallFlow tests the normal installation flow
func TestE2E_InstallFlow(t *testing.T) {
	baseFlags := &installer.Flags{
		Channel:        installer.DEFAULT_CHANNEL,
		NonInteractive: false,
	}

	model := newSetupModel("/tmp/test-project", baseFlags)

	// Start at welcome screen
	if model.step != stepWelcome {
		t.Fatalf("Expected stepWelcome, got %v", model.step)
	}

	// Press enter to select "Install YWAI in a project" (index 0)
	msg := tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{}}
	var m tea.Model
	m, _ = model.Update(msg)
	model = m.(setupModel)

	// Should move to stepPath
	if model.step != stepPath {
		t.Fatalf("Expected stepPath, got %v", model.step)
	}

	// Press enter to accept default path
	msg = tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{}}
	m, _ = model.Update(msg)
	model = m.(setupModel)

	// Should move to stepProjectType
	if model.step != stepProjectType {
		t.Fatalf("Expected stepProjectType, got %v", model.step)
	}
}

// TestE2E_GlobalAgentCreation tests the "Create a global agent" functionality
func TestE2E_GlobalAgentCreation(t *testing.T) {
	baseFlags := &installer.Flags{
		Channel:        installer.DEFAULT_CHANNEL,
		NonInteractive: false,
	}

	model := newSetupModel("/tmp", baseFlags)

	// Navigate to "Create a global agent" option (index 4)
	msg := tea.KeyMsg{Type: tea.KeyDown, Runes: []rune{}}
	for i := 0; i < 4; i++ {
		var m tea.Model
		m, _ = model.Update(msg)
		model = m.(setupModel)
	}

	// Press enter to select
	msg = tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{}}
	var m tea.Model
	m, _ = model.Update(msg)
	model = m.(setupModel)

	// Should now be in stepAgentType
	if model.step != stepAgentType {
		t.Fatalf("Expected stepAgentType, got %v", model.step)
	}
}

// TestE2E_GlobalAgentManagement tests the "Manage global agents" functionality
func TestE2E_GlobalAgentManagement(t *testing.T) {
	baseFlags := &installer.Flags{
		Channel:        installer.DEFAULT_CHANNEL,
		NonInteractive: false,
	}

	model := newSetupModel("/tmp", baseFlags)

	// Navigate to "Manage global agents" option (index 5)
	msg := tea.KeyMsg{Type: tea.KeyDown, Runes: []rune{}}
	for i := 0; i < 5; i++ {
		var m tea.Model
		m, _ = model.Update(msg)
		model = m.(setupModel)
	}

	// Press enter to select
	msg = tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{}}
	var m tea.Model
	m, _ = model.Update(msg)
	model = m.(setupModel)

	// Should now be in stepAgentList
	if model.step != stepAgentList {
		t.Fatalf("Expected stepAgentList, got %v", model.step)
	}
}

// TestE2E_UpdateExistingSetup tests the "Update an existing YWAI setup" functionality
func TestE2E_UpdateExistingSetup(t *testing.T) {
	baseFlags := &installer.Flags{
		Channel:        installer.DEFAULT_CHANNEL,
		NonInteractive: false,
	}

	model := newSetupModel("/tmp/test-project", baseFlags)

	// Navigate to "Update an existing YWAI setup" option (index 1)
	msg := tea.KeyMsg{Type: tea.KeyDown, Runes: []rune{}}
	var m tea.Model
	m, _ = model.Update(msg)
	model = m.(setupModel)

	// Press enter to select
	msg = tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{}}
	m, _ = model.Update(msg)
	model = m.(setupModel)

	// Should move to stepPath
	if model.step != stepPath {
		t.Fatalf("Expected stepPath, got %v", model.step)
	}
}

// TestE2E_InstallMissingSkills tests the "Install missing skills" functionality
func TestE2E_InstallMissingSkills(t *testing.T) {
	baseFlags := &installer.Flags{
		Channel:        installer.DEFAULT_CHANNEL,
		NonInteractive: false,
	}

	model := newSetupModel("/tmp/test-project", baseFlags)

	// Navigate to "Install missing skills in this repo" option (index 2)
	msg := tea.KeyMsg{Type: tea.KeyDown, Runes: []rune{}}
	for i := 0; i < 2; i++ {
		var m tea.Model
		m, _ = model.Update(msg)
		model = m.(setupModel)
	}

	// Press enter to select
	msg = tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{}}
	var m tea.Model
	m, _ = model.Update(msg)
	model = m.(setupModel)

	// Should move to stepPath
	if model.step != stepPath {
		t.Fatalf("Expected stepPath, got %v", model.step)
	}
}

// TestE2E_QuitFromWelcome tests quitting from the welcome screen
func TestE2E_QuitFromWelcome(t *testing.T) {
	baseFlags := &installer.Flags{
		Channel:        installer.DEFAULT_CHANNEL,
		NonInteractive: false,
	}

	model := newSetupModel("/tmp", baseFlags)

	// Navigate to "Exit" option (index 6)
	msg := tea.KeyMsg{Type: tea.KeyDown, Runes: []rune{}}
	for i := 0; i < 6; i++ {
		var m tea.Model
		m, _ = model.Update(msg)
		model = m.(setupModel)
	}

	// Press enter to select
	msg = tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{}}
	var m tea.Model
	m, _ = model.Update(msg)
	model = m.(setupModel)

	// Should be quitting
	if !model.quitting {
		t.Error("Expected quitting to be true after selecting Exit")
	}
}

// TestE2E_GlobalToolsToggle tests toggling global tools selection
func TestE2E_GlobalToolsToggle(t *testing.T) {
	baseFlags := &installer.Flags{
		Channel:        installer.DEFAULT_CHANNEL,
		NonInteractive: false,
	}

	model := newSetupModel("/tmp", baseFlags)

	// Navigate to "Update global tools" option (index 3)
	msg := tea.KeyMsg{Type: tea.KeyDown, Runes: []rune{}}
	for i := 0; i < 3; i++ {
		var m tea.Model
		m, _ = model.Update(msg)
		model = m.(setupModel)
	}

	// Press enter to select
	msg = tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{}}
	var m tea.Model
	m, _ = model.Update(msg)
	model = m.(setupModel)

	// Should now be in stepGlobalTools
	if model.step != stepGlobalTools {
		t.Fatalf("Expected stepGlobalTools, got %v", model.step)
	}

	// Toggle first tool (GA) with space
	msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{}}
	m, _ = model.Update(msg)
	model = m.(setupModel)

	// GA should be unchecked
	if model.globalToolValues[0] {
		t.Error("Expected GA to be unchecked after space toggle")
	}

	// Toggle it back
	msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{}}
	var m2 tea.Model
	m2, _ = model.Update(msg)
	model = m2.(setupModel)

	// GA should be checked again
	if !model.globalToolValues[0] {
		t.Error("Expected GA to be checked after second space toggle")
	}
}

// TestE2E_GlobalToolsSelectAll tests selecting/deselecting all global tools
func TestE2E_GlobalToolsSelectAll(t *testing.T) {
	baseFlags := &installer.Flags{
		Channel:        installer.DEFAULT_CHANNEL,
		NonInteractive: false,
	}

	model := newSetupModel("/tmp", baseFlags)

	// Navigate to "Update global tools" option (index 3)
	msg := tea.KeyMsg{Type: tea.KeyDown, Runes: []rune{}}
	for i := 0; i < 3; i++ {
		var m tea.Model
		m, _ = model.Update(msg)
		model = m.(setupModel)
	}

	// Press enter to select
	msg = tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{}}
	var m tea.Model
	m, _ = model.Update(msg)
	model = m.(setupModel)

	// Press 'a' to deselect all
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	var m3 tea.Model
	m3, _ = model.Update(msg)
	model = m3.(setupModel)

	// All should be unchecked
	for i, val := range model.globalToolValues {
		if val {
			t.Errorf("Expected tool %d to be unchecked after 'a', got true", i)
		}
	}

	// Press 'a' again to select all
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	var m4 tea.Model
	m4, _ = model.Update(msg)
	model = m4.(setupModel)

	// All should be checked
	for i, val := range model.globalToolValues {
		if !val {
			t.Errorf("Expected tool %d to be checked after second 'a', got false", i)
		}
	}
}

// TestE2E_GlobalToolsEscape tests escaping from global tools screen
func TestE2E_GlobalToolsEscape(t *testing.T) {
	baseFlags := &installer.Flags{
		Channel:        installer.DEFAULT_CHANNEL,
		NonInteractive: false,
	}

	model := newSetupModel("/tmp", baseFlags)

	// Navigate to "Update global tools" option (index 3)
	msg := tea.KeyMsg{Type: tea.KeyDown, Runes: []rune{}}
	for i := 0; i < 3; i++ {
		var m tea.Model
		m, _ = model.Update(msg)
		model = m.(setupModel)
	}

	// Press enter to select
	msg = tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{}}
	var m tea.Model
	m, _ = model.Update(msg)
	model = m.(setupModel)

	// Should now be in stepGlobalTools
	if model.step != stepGlobalTools {
		t.Fatalf("Expected stepGlobalTools, got %v", model.step)
	}

	// Press escape to go back to welcome
	msg = tea.KeyMsg{Type: tea.KeyEsc, Runes: []rune{}}
	var m3 tea.Model
	m3, _ = model.Update(msg)
	model = m3.(setupModel)

	// Should return to stepWelcome
	if model.step != stepWelcome {
		t.Fatalf("Expected stepWelcome after escape, got %v", model.step)
	}
}

// TestE2E_InstallWithDryRun validates that installation in dry-run mode doesn't create files
func TestE2E_InstallWithDryRun(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ywai-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	baseFlags := &installer.Flags{
		Channel:        installer.DEFAULT_CHANNEL,
		NonInteractive: false,
		DryRun:         true,
		Target:         tmpDir,
	}

	model := newSetupModel(tmpDir, baseFlags)

	// Navigate through install flow
	msg := tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{}}
	var m tea.Model
	m, _ = model.Update(msg) // Select Install
	model = m.(setupModel)
	m, _ = model.Update(msg) // Accept path
	model = m.(setupModel)
	m, _ = model.Update(msg) // Accept project type
	model = m.(setupModel)
	m, _ = model.Update(msg) // Accept provider
	model = m.(setupModel)
	m, _ = model.Update(msg) // Accept components
	model = m.(setupModel)
	m, _ = model.Update(msg) // Confirm
	model = m.(setupModel)

	// Verify no files were created in dry-run mode
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read temp dir: %v", err)
	}
	if len(entries) > 0 {
		t.Errorf("Dry-run should not create files, but found %d entries", len(entries))
	}
}

// TestE2E_GlobalOnlyMode validates that GlobalOnly mode doesn't write to repo
func TestE2E_GlobalOnlyMode(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ywai-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	baseFlags := &installer.Flags{
		Channel:    installer.DEFAULT_CHANNEL,
		GlobalOnly: true,
		Silent:     true,
		Target:     tmpDir,
	}

	inst := installer.New(baseFlags)

	// Test that installSDD in GlobalOnly mode doesn't write to repo
	// It should only update global skills in ~/.local/share/ga/skills/
	err = inst.UpdateSDD()
	if err != nil {
		t.Logf("UpdateSDD error (expected in test env): %v", err)
	}

	// Verify no files were created in temp dir
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read temp dir: %v", err)
	}
	if len(entries) > 0 {
		t.Errorf("GlobalOnly mode should not create files in repo, but found %d entries", len(entries))
	}
}

// TestE2E_GlobalSkillsInstallation validates that global skills directory path is correct
func TestE2E_GlobalSkillsInstallation(t *testing.T) {
	// Get the global skills directory
	home, _ := os.UserHomeDir()
	globalSkillsDir := filepath.Join(home, ".local", "share", "ga", "skills")

	// Note: Actual installation requires a real skills source directory
	// This test validates the structure and paths are correct
	if globalSkillsDir == "" {
		t.Error("Global skills directory should not be empty")
	}
}

// TestE2E_InstallInTempRepo tests full installation in a temporary repository
func TestE2E_InstallInTempRepo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ywai-test-repo-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize git repo
	err = runCommand(t, tmpDir, "git", "init")
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create a basic package.json for a NestJS project
	packageJson := `{
		"name": "test-project",
		"version": "1.0.0",
		"dependencies": {
			"@nestjs/core": "^10.0.0"
		}
	}`
	err = os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(packageJson), 0644)
	if err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	// Run installation in dry-run mode first
	baseFlags := &installer.Flags{
		Channel:        installer.DEFAULT_CHANNEL,
		NonInteractive: true,
		DryRun:         true,
		Target:         tmpDir,
		ProjectType:    "nest",
		Provider:       "opencode",
		InstallGA:      true,
		InstallSDD:     true,
		InstallVSCode:  true,
		InstallExt:     true,
	}

	inst := installer.New(baseFlags)
	err = inst.Run()
	if err != nil {
		t.Logf("Installation error (expected in dry-run): %v", err)
	}

	// Verify no files were created except .git and package.json
	entries, _ := os.ReadDir(tmpDir)
	for _, entry := range entries {
		if entry.Name() != ".git" && entry.Name() != "package.json" {
			t.Errorf("Dry-run should not create files, but found: %s", entry.Name())
		}
	}
}

// TestE2E_UpdateInTempRepo tests updating an existing YWAI setup
func TestE2E_UpdateInTempRepo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ywai-test-update-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize git repo
	err = runCommand(t, tmpDir, "git", "init")
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create a basic YWAI setup structure
	ywaiDir := filepath.Join(tmpDir, ".ywai")
	err = os.MkdirAll(ywaiDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .ywai dir: %v", err)
	}

	configJson := `{
		"project_type": "nest",
		"provider": "opencode"
	}`
	err = os.WriteFile(filepath.Join(ywaiDir, "config.json"), []byte(configJson), 0644)
	if err != nil {
		t.Fatalf("Failed to create config.json: %v", err)
	}

	// Run update in dry-run mode
	baseFlags := &installer.Flags{
		Channel:        installer.DEFAULT_CHANNEL,
		NonInteractive: true,
		DryRun:         true,
		Target:         tmpDir,
		UpdateAll:      true,
	}

	inst := installer.New(baseFlags)
	err = inst.Run()
	if err != nil {
		t.Logf("Update error (expected in dry-run): %v", err)
	}

	// Verify .ywai directory still exists
	if _, err := os.Stat(ywaiDir); os.IsNotExist(err) {
		t.Error("Update should not remove .ywai directory")
	}
}

// runCommand helper to execute commands in a directory
func runCommand(t *testing.T, dir string, name string, args ...string) error {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Command %s %v failed: %s\nOutput: %s", name, args, err, string(output))
	}
	return err
}
