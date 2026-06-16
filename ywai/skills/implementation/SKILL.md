---
name: implementation
description: Generic implementation worker (default dev role)
---

# implementation

## Required Skills and Tools
- git

## Work Procedure

1. Read the feature description and expected behavior
2. Write failing tests first (TDD)
3. Implement the feature to make tests pass
4. Run tests and verify they pass
5. Manually verify the implementation
6. Return a structured handoff

## Example Handoff

{
  "salientSummary": "Implemented the feature as described",
  "whatWasImplemented": "Feature implementation completed with tests",
  "whatWasLeftUndone": "",
  "verification": {
    "commandsRun": [
      {"command": "go test ./...", "exitCode": 0, "observation": "All tests passed"}
    ]
  },
  "tests": {
    "added": [],
    "coverage": "N/A"
  },
  "discoveredIssues": []
}

## When to Return to Orchestrator

Return to orchestrator if: requirements are ambiguous, existing bugs affect this feature, or you cannot complete within mission boundaries
