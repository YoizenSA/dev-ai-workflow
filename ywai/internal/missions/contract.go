package missions

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ─── Errors ────────────────────────────────────────────────────────────────

var (
	ErrContractNotFound    = fmt.Errorf("validation contract not found")
	ErrInvalidContract    = fmt.Errorf("invalid validation contract")
	ErrCoverageIncomplete = fmt.Errorf("validation contract coverage incomplete")
)

// ─── Validation Contract Parser ───────────────────────────────────────────

// ContractParser parses validation-contract.md files.
type ContractParser struct {
	missionDir string
}

// NewContractParser creates a new ContractParser for the given mission directory.
func NewContractParser(missionDir string) *ContractParser {
	return &ContractParser{missionDir: missionDir}
}

// LoadContract loads and parses the validation contract from validation-contract.md.
func (cp *ContractParser) LoadContract() (*ValidationContract, error) {
	contractPath := filepath.Join(cp.missionDir, "validation-contract.md")
	
	content, err := os.ReadFile(contractPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrContractNotFound, contractPath)
		}
		return nil, fmt.Errorf("read contract file: %w", err)
	}
	
	return cp.parseContract(content)
}

// parseContract parses validation-contract.md content into a ValidationContract.
func (cp *ContractParser) parseContract(content []byte) (*ValidationContract, error) {
	lines := strings.Split(string(content), "\n")
	
	contract := &ValidationContract{}
	var currentArea string
	var assertion ContractAssertion
	var descBuilder strings.Builder
	
	// Regex to match assertion headers: ### VAL-XXX-001: Title
	assertionRegex := regexp.MustCompile(`^###\s+(VAL-[A-Z0-9]+-\d+):\s*(.+)`)
	areaRegex := regexp.MustCompile(`^##\s+(.+)`)
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Match area headers (## Area Name)
		if areaMatches := areaRegex.FindStringSubmatch(trimmed); len(areaMatches) > 1 {
			currentArea = strings.TrimSpace(areaMatches[1])
			// Remove "Area:" prefix if present
			currentArea = strings.TrimPrefix(currentArea, "Area:")
			currentArea = strings.TrimSpace(currentArea)
			continue
		}
		
		// Match assertion headers (### VAL-XXX-001: Title)
		if matches := assertionRegex.FindStringSubmatch(trimmed); len(matches) > 2 {
			// Save previous assertion if exists
			if assertion.ID != "" {
				assertion.Description = strings.TrimSpace(descBuilder.String())
				assertion.Area = currentArea
				contract.Assertions = append(contract.Assertions, assertion)
			}
			
			// Start new assertion
			assertion = ContractAssertion{
				ID:    matches[1],
				Title: strings.TrimSpace(matches[2]),
				Area:  currentArea,
			}
			descBuilder.Reset()
			continue
		}
		
		// Parse assertion fields
		if assertion.ID != "" {
			if strings.HasPrefix(trimmed, "Tool:") {
				assertion.Tool = strings.TrimSpace(strings.TrimPrefix(trimmed, "Tool:"))
			} else if strings.HasPrefix(trimmed, "Evidence:") {
				evidenceStr := strings.TrimSpace(strings.TrimPrefix(trimmed, "Evidence:"))
				// Parse comma-separated evidence types
				evidenceItems := strings.Split(evidenceStr, ",")
				for _, item := range evidenceItems {
					item = strings.TrimSpace(item)
					if item != "" {
						assertion.Evidence = append(assertion.Evidence, item)
					}
				}
			} else if !strings.HasPrefix(trimmed, "###") && !strings.HasPrefix(trimmed, "##") {
				// Collect description lines
				if trimmed != "" {
					descBuilder.WriteString(line + "\n")
				}
			}
		}
	}
	
	// Save last assertion
	if assertion.ID != "" {
		assertion.Description = strings.TrimSpace(descBuilder.String())
		assertion.Area = currentArea
		contract.Assertions = append(contract.Assertions, assertion)
	}
	
	if len(contract.Assertions) == 0 {
		return nil, fmt.Errorf("%w: no assertions found", ErrInvalidContract)
	}
	
	return contract, nil
}

// CheckCoverage verifies that every assertion in the contract is claimed
// by exactly one feature in the features list.
func (cp *ContractParser) CheckCoverage(contract *ValidationContract, features []Feature) error {
	// Build map of claimed assertions
	claimed := make(map[string]int) // assertion ID -> count
	
	for _, feat := range features {
		for _, fulfills := range feat.Fulfills {
			claimed[fulfills]++
		}
	}
	
	// Check for unclaimed assertions
	var unclaimed []string
	for _, assertion := range contract.Assertions {
		if claimed[assertion.ID] == 0 {
			unclaimed = append(unclaimed, assertion.ID)
		}
	}
	
	if len(unclaimed) > 0 {
		return fmt.Errorf("%w: %d assertions not claimed by any feature: %v", 
			ErrCoverageIncomplete, len(unclaimed), unclaimed)
	}
	
	// Check for over-claimed assertions (claimed by multiple features)
	var overclaimed []string
	for id, count := range claimed {
		if count > 1 {
			overclaimed = append(overclaimed, fmt.Sprintf("%s (%d times)", id, count))
		}
	}
	
	if len(overclaimed) > 0 {
		return fmt.Errorf("%w: %d assertions claimed by multiple features: %v", 
			ErrInvalidContract, len(overclaimed), overclaimed)
	}
	
	return nil
}

// GetAssertionByID returns an assertion by its ID.
func (c *ValidationContract) GetAssertionByID(id string) *ContractAssertion {
	for _, assertion := range c.Assertions {
		if assertion.ID == id {
			return &assertion
		}
	}
	return nil
}

// GetAssertionsByArea returns all assertions for a given area.
func (c *ValidationContract) GetAssertionsByArea(area string) []ContractAssertion {
	var result []ContractAssertion
	for _, assertion := range c.Assertions {
		if assertion.Area == area {
			result = append(result, assertion)
		}
	}
	return result
}
