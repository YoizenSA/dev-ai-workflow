# Azure Pipelines — Detailed Reference

## Shared Templates Repository

All pipelines consume reusable templates from a **central templates repo**. This repo is declared as a `resource` and referenced via `@pipelines-templates`.

```yaml
resources:
  repositories:
    - repository: pipelines-templates
      type: git
      name: your-project/pipeline-templates
      ref: refs/heads/main
```

Available templates:

| Template | Purpose |
|----------|---------|
| `docker-acr-build-and-push.yml` | Builds Docker image and pushes to ACR |
| `helm-acr-build-and-push.yml` | Packages Helm chart, replaces `{{chartversion}}`, pushes to ACR |
| `restart-deployment.yml` | Restarts Kubernetes deployments in AKS (optional) |

---

## Production Pipeline (`azure-pipelines.yml`)

Triggered **only** on tag creation matching `refs/tags/version/*`. The version is extracted from the tag name via `$(Build.SourceBranchName)`.

```yaml
resources:
  repositories:
    - repository: pipelines-templates
      type: git
      name: your-project/pipeline-templates
      ref: refs/heads/main

trigger:
  branches:
    include:
      - refs/tags/version/*

stages:
  - stage: acrbuild
    pool:
      name: your-agent-pool
    displayName: 'Build and push to ACR'
    jobs:
      - job: buildandpush
        steps:
          - checkout: self
          - template: docker-acr-build-and-push.yml@pipelines-templates
            parameters:
              acrName: '$(acrName)'
              appName: '$(serviceName)'
              serviceConnectionName: '$(serviceConnectionName)'
              appVersion: $(Build.SourceBranchName)
          - template: helm-acr-build-and-push.yml@pipelines-templates
            parameters:
              innerPath: '$(repoDir)/DevOps/Helm'
              serviceConnectionName: '$(serviceConnectionName)'
              acrName: '$(acrName)'
              appName: '$(productName)'
              appVersion: $(Build.SourceBranchName)
```

---

## Dev Pipeline (`azure-pipelines-dev.yml`)

Same structure as production but with these differences:
- **Trigger**: branch pushes (`dev`, feature branches) instead of tags
- **Version**: static variable (`appVersion: 0.1.1`) instead of `$(Build.SourceBranchName)`
- `docker-acr-build-and-push` adds `innerPath` and `dockerfile` parameters (needed when Dockerfile is not at repo root)
- Optionally includes `restart-deployment.yml@pipelines-templates` to restart pods after push

---

## Template Parameters

### `docker-acr-build-and-push.yml`

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `innerPath` | No | `"."` | Path to Docker build context (project root) |
| `dockerfile` | Yes | — | Path to `Dockerfile` (relative to repo root) |
| `serviceConnectionName` | Yes | — | Azure ARM service connection name |
| `acrName` | Yes | — | ACR registry name (without `.azurecr.io`) |
| `appName` | Yes | — | Image name (used as repository path in ACR) |
| `appVersion` | Yes | — | Image tag — from git tag or static variable |
| `buildArgs` | No | `""` | Additional Docker build arguments (e.g. `--build-arg KEY=VAL`) |

Internally runs: `docker build -t {acrUrl}/{appName}:{appVersion} {innerPath} -f {dockerfile} {buildArgs}`

### `helm-acr-build-and-push.yml`

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `innerPath` | No | `"."` | Path to `DevOps/Helm` directory |
| `serviceConnectionName` | Yes | — | Azure ARM service connection name |
| `acrName` | Yes | — | ACR registry name (without `.azurecr.io`) |
| `appName` | Yes | — | Helm chart name (product name) |
| `appVersion` | Yes | — | Chart version — from git tag or static variable |

Internal steps:
1. `find {innerPath} -type f -exec sed -i 's/{{chartversion}}/{appVersion}/g' {} +`
2. `helm dependency update {innerPath}`
3. `helm package {innerPath} --version {appVersion} --app-version {appVersion}`
4. `helm push {appName}-{appVersion}.tgz oci://{acrUrl}/helm`

### `restart-deployment.yml` (optional)

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `deployments` | No | `""` | **Space-separated** list of deployment names. If empty, **all** deployments in the namespace are restarted. |
| `namespace` | Yes | — | Kubernetes namespace |
| `serviceConnectionName` | No | `"azure"` | Azure ARM service connection name |
| `aksName` | No | `"your-aks"` | AKS cluster name |
| `aksRG` | No | `"your-aks-rg"` | AKS resource group |
