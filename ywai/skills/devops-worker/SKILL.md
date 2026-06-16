---
name: devops-worker
description: DevOps and infrastructure worker
---

# devops-worker

## Required Skills and Tools
- docker
- kubectl
- helm

## Work Procedure

1. Read the feature description and expected behavior
2. Implement infrastructure changes (Docker, K8s, CI/CD)
3. Test the infrastructure locally
4. Verify services start and healthcheck passes
5. Return a structured handoff

## Example Handoff

{
  "salientSummary": "Added Docker configuration for API service",
  "whatWasImplemented": "Created Dockerfile and docker-compose configuration for API service",
  "whatWasLeftUndone": "",
  "verification": {
    "commandsRun": [
      {"command": "docker compose up -d api", "exitCode": 0, "observation": "Service started successfully"},
      {"command": "curl -sf http://localhost:3100/health", "exitCode": 0, "observation": "Healthcheck passed"}
    ]
  },
  "tests": {
    "added": [],
    "coverage": "N/A"
  },
  "discoveredIssues": []
}

## When to Return to Orchestrator

Return to orchestrator if: infrastructure requirements are unclear, or you cannot complete within mission boundaries
