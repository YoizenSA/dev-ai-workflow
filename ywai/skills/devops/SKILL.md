---
name: devops
description: >
  Azure DevOps YAML pipelines and Helm Umbrella chart conventions for multi-service projects.
  Concrete actions: create multi-stage pipeline YAML for Docker build/push to ACR, generate Umbrella Helm charts with sub-charts, configure values.yaml environment contracts, set up tag-triggered versioning strategy.
  Trigger: When the user asks to create or modify Azure DevOps pipelines, generate or maintain Helm Umbrella charts, configure values.yaml service nodes, set up Docker-to-ACR image tagging, or scaffold the DevOps directory for a new project.
license: Apache-2.0
metadata:
  author: Yoizen
  version: "3.0"
  scope: [root]
  auto_invoke:
    - "pipeline"
    - "azure pipelines"
    - "azure devops"
    - "helm"
    - "helm chart"
    - "umbrella chart"
    - "docker acr"
    - "devops"
    - "kubernetes"
    - "k8s"
    - "deploy"
    - "ci/cd"
    - "cicd"
    - "values.yaml"
allowed-tools: Read, Edit, Write, Glob, Grep, Bash
---

## When to Use

- Create Azure DevOps YAML pipeline files for new multi-service projects
- Generate Helm Umbrella chart structure (`Chart.yaml`, sub-charts, `values.yaml`)
- Add a new service (sub-chart + pipeline matrix entry + values node)
- Configure `values.yaml` environment variable contracts for services
- Maintain and evolve existing pipelines, Helm charts, and values
- Set up Docker image tagging and Helm chart versioning strategy

---

## Versioning Strategy

All version management uses a single mechanism: the `{{chartversion}}` placeholder.

- **Where it appears**: Umbrella `Chart.yaml` (`version` + `appVersion`), sub-chart `Chart.yaml` (`version`), and `values.yaml` (`global.appVersion`)
- **When it's replaced**: The pipeline's `helm-acr-build-and-push` template runs `find ... -exec sed` to replace `{{chartversion}}` in ALL files recursively before packaging
- **Production version source**: Extracted from the git tag name (`refs/tags/version/*` → `$(Build.SourceBranchName)`)
- **Dev version source**: Static variable in the dev pipeline (e.g. `appVersion: 0.1.1`)
- **Rule**: The Umbrella chart version and all Docker image versions **MUST be identical** for the same release. Never hardcode versions — always use `{{chartversion}}`

---

## Azure Pipelines

Each project has two pipeline files: production (tag-triggered) and dev (branch-triggered). Both consume shared templates from a central `pipeline-templates` repo via `@pipelines-templates`.

### Trigger Strategy

| Pipeline | Trigger | Version Source |
|----------|---------|----------------|
| `azure-pipelines.yml` | `refs/tags/version/*` only | `$(Build.SourceBranchName)` from tag |
| `azure-pipelines-dev.yml` | Branch pushes (`dev`, feature) | Static variable (e.g. `0.1.1`) |

**NEVER** trigger production builds on branch pushes — only tags.

### Shared Templates

| Template | Purpose |
|----------|---------|
| `docker-acr-build-and-push.yml` | Build Docker image → push to ACR |
| `helm-acr-build-and-push.yml` | Replace `{{chartversion}}` → package → push Helm chart to ACR |
| `restart-deployment.yml` | Restart K8s deployments in AKS (optional, dev only) |

> Full pipeline YAML examples and template parameter tables: [references/PIPELINES.md](references/PIPELINES.md)

---

## Helm Chart Structure

The project uses an **Umbrella Helm chart** pattern. The umbrella declares `common-helpers` as its only explicit dependency. Sub-charts live inside `charts/` and are auto-discovered by Helm.

```
DevOps/Helm/
├── Chart.yaml                  # apiVersion v2 — common-helpers dependency only
├── values.yaml                 # Unified values: global + one node per service
├── templates/
│   └── utils.yaml              # ConfigMap, Secret, PV, PVC (umbrella-level)
└── charts/
    ├── service1/
    │   ├── Chart.yaml          # apiVersion v1 — no dependencies
    │   └── templates/
    │       ├── deployment.yaml # {{ include "common-helpers.deploymenttemplate" . }}
    │       └── utils.yaml      # Service, Ingress, PDB, HPA via common-helpers
    └── service2/               # Same structure — copy for each service
```

### Key Rules

- Sub-charts are **NOT** listed as dependencies in the umbrella `Chart.yaml` — they are auto-discovered from `charts/`
- Sub-charts use `apiVersion: v1` and declare **no dependencies** — `common-helpers` is inherited from the umbrella
- Sub-chart templates are **generic** (copy as-is for each new service — no modification needed)
- All K8s resource names follow: `{productName}-{client}-{environment}-{appName}`

> Full chart YAML templates, common-helpers reference, and deployment features: [references/HELM-STRUCTURE.md](references/HELM-STRUCTURE.md)

---

## values.yaml Contract

One top-level node per service (key MUST match sub-chart directory name and `appName`). `global` holds shared config.

### Key Fields (quick reference)

| Scope | Key Fields |
|-------|------------|
| **Global** | `productName`, `appVersion` (`{{chartversion}}`), `client`, `environment`, `registry` |
| **Service identity** | `appName` (must match node key + chart dir) |
| **Deployment** | `replicas`, `containerPort`, `image`, `healthcheck` |
| **Resources** | `requestsCPU`, `requestsMEM`, `limitsCPU`, `limitMEM` |
| **Networking** | `enableIngress`, `IngressUrl`, `IngressClassName` |
| **Scaling** | `enableHPA`, `minReplicas`, `maxReplicas`, `enablePDB` |
| **Environment** | `requiredConfigMapEnv`, `optionalConfigMapEnv`, `requiredSecretEnv`, `optionalSecretEnv` |

> Full field tables, global section details, and values.yaml template: [references/VALUES-REFERENCE.md](references/VALUES-REFERENCE.md)

---

## Validation & Verification

Run these checks before committing changes or after generating new files:

### Helm Chart Validation

```bash
# Lint the chart (catches schema errors, missing required values)
helm lint DevOps/Helm/

# Render templates locally to verify output (without deploying)
helm template my-release DevOps/Helm/ --debug

# Verify dependency resolution works
helm dependency update DevOps/Helm/

# Check that {{chartversion}} placeholder exists in all expected files
grep -r '{{chartversion}}' DevOps/Helm/
```

### Pipeline Validation

```bash
# Verify YAML syntax
python -c "import yaml; yaml.safe_load(open('DevOps/azure-pipelines.yml'))"

# Check that shared template references are correct
grep -n '@pipelines-templates' DevOps/azure-pipelines*.yml
```

### Naming Consistency Check

```bash
# Verify service names are in sync across values.yaml nodes, chart dirs, and appName values
# Each sub-chart dir name must match its Chart.yaml 'name' and its values.yaml node key + appName
ls DevOps/Helm/charts/
grep 'appName:' DevOps/Helm/values.yaml
grep '^name:' DevOps/Helm/charts/*/Chart.yaml
```

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `{{chartversion}}` appears in deployed resources | Pipeline did not run `sed` replacement | Verify `helm-acr-build-and-push` template `innerPath` points to `DevOps/Helm` |
| `helm dependency update` fails | `common-helpers` version mismatch or ACR auth issue | Check umbrella `Chart.yaml` dependency version matches published chart; verify ACR login |
| Pod stuck in `CreateContainerConfigError` | Missing `requiredConfigMapEnv` or `requiredSecretEnv` values | Ensure all required keys exist as `global.*` values or are injected by CD pipeline Variable Group |
| Ingress not created | `enableIngress` not set to `true` in service node | Add `enableIngress: true` and `IngressUrl` to the service's values.yaml node |
| HPA ignoring `replicas` field | Expected behavior when `enableHPA: true` | HPA manages replica count — `replicas` is ignored. Set `minReplicas`/`maxReplicas` instead |
| Sub-chart not rendered | Directory missing from `charts/` or `Chart.yaml` malformed | Verify sub-chart dir exists and its `Chart.yaml` has valid `apiVersion: v1` + `name` matching dir |
| Resources not appearing in deployment | No resource fields set | At least one of `requestsCPU`/`requestsMEM`/`limitsCPU`/`limitMEM` must be set to render resources block |

---

## Critical Patterns

- **ALWAYS** use `{{chartversion}}` for all versions — never hardcode (see [Versioning Strategy](#versioning-strategy))
- **ALWAYS** keep service node key, `appName` value, and sub-chart directory name **in sync**
- **NEVER** override `values.yaml` defaults at deploy time — only extend with new keys
- **ALWAYS** use shared pipeline templates from `pipelines-templates` repo — never inline Docker or Helm commands
- Sub-charts declare **no dependencies** — `common-helpers` is resolved at the umbrella level only
- **ALL resource names** follow `{productName}-{client}-{environment}-{appName}` — never deviate
- `global.environment` and `global.client` are populated by the CD pipeline from Variable Groups
- Variable Group name convention: `{appName}-{clientName}-{environment}` — keys use `__` as level separator
- ConfigMap values resolve from `global.*` — any key in `requiredConfigMapEnv`/`optionalConfigMapEnv` **MUST** exist as `global.{key}`
- **ALWAYS** run `helm lint` and `helm template --debug` before committing chart changes

---

## Resources

- [references/PIPELINES.md](references/PIPELINES.md) — Full pipeline YAML, template parameters
- [references/HELM-STRUCTURE.md](references/HELM-STRUCTURE.md) — Chart templates, common-helpers library, deployment features
- [references/VALUES-REFERENCE.md](references/VALUES-REFERENCE.md) — Global section, service node fields, values.yaml template

