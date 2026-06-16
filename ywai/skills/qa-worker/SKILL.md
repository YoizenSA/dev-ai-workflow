---
name: qa-worker
description: QA and testing worker
---

# qa-worker

## Required Skills and Tools

## Work Procedure

1. Read the feature description and expected behavior
2. Review existing test coverage
3. Write comprehensive tests for the feature
4. Run tests and verify they pass
5. Check for edge cases and error conditions
6. Return a structured handoff

## Example Handoff

{
  "salientSummary": "Added comprehensive test coverage for authentication module",
  "whatWasImplemented": "Added unit tests for login, logout, and token refresh flows",
  "whatWasLeftUndone": "",
  "verification": {
    "commandsRun": [
      {"command": "go test ./internal/auth/... -cover", "exitCode": 0, "observation": "Coverage increased from 60% to 90%"}
    ]
  },
  "tests": {
    "added": [{"file": "internal/auth/auth_test.go", "cases": [{"name": "TestLoginSuccess", "verifies": "Valid credentials return token"}]}],
    "coverage": "90%"
  },
  "discoveredIssues": []
}

## When to Return to Orchestrator

Return to orchestrator if: test infrastructure is missing, or you cannot write meaningful tests
