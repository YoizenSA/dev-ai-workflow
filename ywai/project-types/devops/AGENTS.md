# DevOps Project Agent Instructions

## Scope

- This template applies to infrastructure, CI/CD, container, Helm, Kubernetes, and release automation work.
- Prefer safe, reviewable, reversible changes. Do not run live deployment commands unless explicitly asked.
- Read existing pipeline/chart conventions before editing generated manifests or YAML.

## Operating Workflow

1. Discover the deployment topology: environments, image naming, chart structure, secrets, and rollback path.
2. Propose the smallest safe change and call out blast radius.
3. Edit source files, not generated outputs, unless the repo intentionally tracks generated files.
4. Validate with static checks, template rendering, dry-runs, or pipeline validation before suggesting deployment.
5. Document rollout, smoke checks, and rollback instructions for operational changes.

## Non-Negotiables

- Never commit secrets, kubeconfigs, cloud credentials, tokens, or production endpoints with credentials.
- Use secret managers, CI secret variables, sealed secrets, or external secret operators for sensitive values.
- Parameterize environment-specific values; keep defaults safe for local/dev use.
- Ask before destructive infrastructure actions: deleting namespaces, resources, databases, volumes, or state.
- Prefer immutable image tags or digests for release artifacts; avoid mutable `latest` in production.

## CI/CD Standards

- Keep pipelines deterministic: pinned tool versions, explicit inputs, and clear stage dependencies.
- Separate build, test, package, scan, deploy, and smoke-test responsibilities.
- Cache dependencies only when cache keys are safe and reproducible.
- Fail fast on validation errors; do not hide failures with blanket retries.
- Keep scripts idempotent and shell-safe (`set -euo pipefail` where supported).

## Docker Standards

- Use multi-stage builds and minimal runtime images.
- Do not copy `.git`, local env files, caches, or secrets into images.
- Run as a non-root user when the base image supports it.
- Add health/readiness signals at the application or orchestrator level.

## Helm / Kubernetes Standards

- Keep `values.yaml` as a documented contract; put environment overrides in separate values files.
- Prefer `helm lint` and `helm template` before cluster operations.
- Define resource requests/limits, probes, labels, annotations, and security contexts intentionally.
- Make rollout strategy explicit for user-facing services.
- Avoid hardcoding namespaces unless the deployment model requires it.

## Verification Commands

Use the commands already present in the repo. Common checks include:

```bash
helm lint <chart-dir>
helm template <release-name> <chart-dir> -f <values-file>
kubectl apply --dry-run=server -f <manifest-or-rendered-file>
docker build -t <local-image>:check .
```

If a check requires cluster/cloud credentials, explain what should be run and why instead of guessing.

## Skills

Read `.atl/skill-registry.md` (or `skills/skill-registry.md` in older setups) for the authoritative skill list, trigger patterns, and compact rules. When work matches a skill trigger, invoke that skill first.
