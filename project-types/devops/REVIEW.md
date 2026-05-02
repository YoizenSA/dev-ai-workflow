# DevOps Code Review Rules

## CI/CD
- Pipelines must be deterministic and reproducible.
- Production releases should be tag/version driven.
- Avoid implicit defaults that can change behavior between environments.

## Security
- ❌ No hardcoded secrets in YAML/scripts.
- Secrets must come from secure providers or environment variables.
- Validate and sanitize external inputs/parameters.

## Docker / Images
- Use explicit image tags and versioning strategy.
- Keep base images updated and minimal.
- Avoid unnecessary root privileges in runtime containers.

## Helm / Kubernetes
- Chart values must remain coherent with app/runtime contracts.
- Readiness/liveness/resource settings should be explicit.
- Changes should include operational verification notes.

## Testing / Validation
- Run lint/check/plan equivalents before merge.
- Include rollback and post-deploy verification criteria.
