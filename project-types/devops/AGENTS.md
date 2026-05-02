# DevOps Engineering Constitution & AI Agent Directives

## Part 1: Core Principles (NON-NEGOTIABLE)

### I. Delivery Safety
- Prioritize safe, reversible deployments.
- Prefer progressive rollout strategies when possible.
- Never ship infrastructure changes without validation steps.

### II. Security-First
- No hardcoded credentials, tokens, or keys.
- Use environment variables / secret managers for all sensitive values.
- Validate external inputs (pipeline variables, chart values, runtime config).

### III. Infrastructure Quality
- Keep CI/CD definitions readable and modular.
- Keep Docker/Helm/Kubernetes configs deterministic and versioned.
- Avoid hidden side effects in scripts.

### IV. Observability
- Add structured logs in scripts where relevant.
- Ensure deployments expose health/readiness signals.
- Prefer explicit failure over silent retries.

---

## Part 2: AI Agent Directives

When asked to implement DevOps work:
1. Analyze existing pipeline/chart/runtime conventions.
2. Propose the safest minimal change.
3. Validate with lint/check/plan where available.
4. Document rollback/verification steps.

### Safety Gates
- Ask confirmation for destructive infrastructure actions.
- Warn immediately if secrets are exposed in code or config.

---

## Part 3: Available Skills

### SDD Orchestrator
- `sdd-init`, `sdd-explore`, `sdd-propose`, `sdd-spec`, `sdd-design`, `sdd-tasks`, `sdd-apply`, `sdd-verify`, `sdd-archive`

### DevOps
- `devops` (CI/CD pipelines, Docker build/push, Helm, Kubernetes, deployment workflows)

### Meta
- `git-commit`, `skill-creator`, `skill-registry`

---

## Part 4: How to invoke

```text
/sdd:new release-hardening
/sdd:ff release-hardening
/sdd:apply
/sdd:verify

# DevOps requests
"Update Azure Pipeline for version tags"
"Create Helm values contract for new service"
"Adjust Kubernetes deployment strategy"
```
