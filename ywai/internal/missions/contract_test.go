package missions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContractParser_LoadContract(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "ywai-contract-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a validation-contract.md file
	contractContent := `# Validation Contract

## Area: Authentication

### VAL-AUTH-001: Successful login
A user with valid credentials submits the login form and is redirected to the dashboard.
Tool: agent-browser
Evidence: screenshot, console-errors, network(POST /api/auth/login -> 200)

### VAL-AUTH-002: Login form validation
Submitting the login form with empty fields shows per-field validation errors without making a network request.
Tool: agent-browser
Evidence: screenshot, console-errors

## Cross-Area Flows

### VAL-CROSS-001: Auth gates pricing
A guest user sees "Sign in for pricing" on the catalog. After logging in, real prices are shown.
Tool: agent-browser
Evidence: screenshot(guest-view), screenshot(authed-view)
`
	if err := os.WriteFile(filepath.Join(tmpDir, "validation-contract.md"), []byte(contractContent), 0644); err != nil {
		t.Fatalf("failed to write validation-contract.md: %v", err)
	}

	// Test loading the contract
	parser := NewContractParser(tmpDir)
	contract, err := parser.LoadContract()
	if err != nil {
		t.Fatalf("failed to load contract: %v", err)
	}

	// Verify contract content
	if len(contract.Assertions) != 3 {
		t.Errorf("expected 3 assertions, got %d", len(contract.Assertions))
	}

	// Check first assertion
	auth1 := contract.GetAssertionByID("VAL-AUTH-001")
	if auth1 == nil {
		t.Error("expected to find VAL-AUTH-001")
	} else {
		if auth1.Title != "Successful login" {
			t.Errorf("expected title 'Successful login', got '%s'", auth1.Title)
		}
		if auth1.Tool != "agent-browser" {
			t.Errorf("expected tool 'agent-browser', got '%s'", auth1.Tool)
		}
		if auth1.Area != "Authentication" {
			t.Errorf("expected area 'Authentication', got '%s'", auth1.Area)
		}
	}

	// Check cross-area assertion
	cross1 := contract.GetAssertionByID("VAL-CROSS-001")
	if cross1 == nil {
		t.Error("expected to find VAL-CROSS-001")
	} else {
		if cross1.Area != "Cross-Area Flows" {
			t.Errorf("expected area 'Cross-Area Flows', got '%s'", cross1.Area)
		}
	}
}

func TestContractParser_ContractNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ywai-contract-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	parser := NewContractParser(tmpDir)
	_, err = parser.LoadContract()
	if err == nil {
		t.Error("expected error for non-existent contract, got nil")
	}
	// Check if error wraps ErrContractNotFound
	if err == nil || !strings.Contains(err.Error(), "validation contract not found") {
		t.Errorf("expected validation contract not found error, got %v", err)
	}
}

func TestContractParser_CheckCoverage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ywai-contract-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a contract
	contractContent := `# Validation Contract

## Core

### VAL-CORE-001: Basic feature
Basic feature works.
Tool: engine
Evidence: logs
`
	if err := os.WriteFile(filepath.Join(tmpDir, "validation-contract.md"), []byte(contractContent), 0644); err != nil {
		t.Fatalf("failed to write validation-contract.md: %v", err)
	}

	parser := NewContractParser(tmpDir)
	contract, err := parser.LoadContract()
	if err != nil {
		t.Fatalf("failed to load contract: %v", err)
	}

	// Test with complete coverage
	features := []Feature{
		{
			ID:       "feat-1",
			Fulfills: []string{"VAL-CORE-001"},
		},
	}
	err = parser.CheckCoverage(contract, features)
	if err != nil {
		t.Errorf("expected no error for complete coverage, got %v", err)
	}

	// Test with incomplete coverage
	features = []Feature{
		{
			ID:       "feat-1",
			Fulfills: []string{}, // No assertions claimed
		},
	}
	err = parser.CheckCoverage(contract, features)
	if err == nil {
		t.Error("expected error for incomplete coverage, got nil")
	}
	// Check if error wraps ErrCoverageIncomplete
	if err == nil || !strings.Contains(err.Error(), "validation contract coverage incomplete") {
		t.Errorf("expected coverage incomplete error, got %v", err)
	}

	// Test with over-claimed assertion
	features = []Feature{
		{
			ID:       "feat-1",
			Fulfills: []string{"VAL-CORE-001"},
		},
		{
			ID:       "feat-2",
			Fulfills: []string{"VAL-CORE-001"}, // Claimed again
		},
	}
	err = parser.CheckCoverage(contract, features)
	if err == nil {
		t.Error("expected error for over-claimed assertion, got nil")
	}
}
