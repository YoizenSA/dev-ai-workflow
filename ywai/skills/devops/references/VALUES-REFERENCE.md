# values.yaml Contract — Detailed Reference

## Structure Rules

- One top-level node per service (matching sub-chart directory name)
- `appName` MUST match the node key and the chart directory name
- `global` holds shared config available to all sub-charts
- `global.appVersion` uses `{{chartversion}}` — replaced by pipeline (see [Versioning Strategy](../SKILL.md#versioning-strategy))
- Values here are defaults — can be extended (never overridden) at deploy time

---

## Global Section

| Key | Purpose |
|-----|---------|
| `global.productName` | Product identifier — used in all resource naming |
| `global.appVersion` | `{{chartversion}}` — replaced by pipeline |
| `global.client` | Client/tenant identifier — used in resource naming |
| `global.environment` | Environment name (e.g. `prd`, `dev`) — used in resource naming and injected by CD pipeline |
| `global.registry` | ACR registry host (e.g. `your-acr.azurecr.io`) — used in default image resolution |
| `global.nodeEnv` | Default `NODE_ENV` for all services |
| `global.PORT` | Default port — available as ConfigMap value |
| `global.NODE_ENV` | Node environment — available as ConfigMap value |
| `global.imagePullSecrets` | Name of image pull secret for private registries (optional) |
| `global.nodeSelector` | Default agent pool name for affinity/tolerations (optional) |
| `global.enablePV` | Enable PersistentVolume creation at umbrella level (optional) |
| `global.enablePVC` | Enable PersistentVolumeClaim creation at umbrella level (optional) |
| `global.capacity` | Storage capacity for PV/PVC (required if `enablePV`/`enablePVC` is true) |
| `global.storageAccount` | Azure storage account for PV (required if `enablePV` is true) |
| `global.containerName` | Azure blob container for PV (required if `enablePV` is true) |
| `global.resourceGroup` | Azure resource group for PV (required if `enablePV` is true) |

> **Note**: `global.client` and `global.environment` are set in the base `values.yaml` as defaults but are typically overridden by the CD pipeline through Variable Groups. All keys in `requiredConfigMapEnv` and `optionalConfigMapEnv` are resolved from `global.*` values.

---

## Service Node Fields

| Key | Type | Default | Purpose |
|-----|------|---------|---------|
| **Identity** | | | |
| `appName` | string | — | Service identifier — MUST match node key and chart dir name |
| **Deployment** | | | |
| `replicas` | string | `0` | Pod replicas (ignored if `enableHPA` is true) |
| `containerPort` | int | `3000` | Port the container listens on |
| `image` | string | auto | Override full image path (default: `{registry}/{productName}-{appName}:{version}`) |
| `imagePullPolicy` | string | `IfNotPresent` | Kubernetes image pull policy |
| **Resources** | | | |
| `requestsCPU` | string | — | CPU request (e.g. `"50m"`) |
| `requestsMEM` | string | — | Memory request (e.g. `"256Mi"`) |
| `limitsCPU` | string | — | CPU limit (optional) |
| `limitMEM` | string | — | Memory limit (optional) |
| **Health probes** | | | |
| `healthcheck` | string | — | HTTP path for probes (e.g. `"/health"`). Enables startup, readiness, liveness probes. |
| **Networking** | | | |
| `enableIngress` | bool | — | Create an Ingress resource |
| `IngressUrl` | string | — | Single hostname for Ingress |
| `IngressClassName` | string | `nginx-private` | Ingress class |
| `IngressTlsSecretName` | string | `your-tls-cert` | TLS secret name |
| **Scaling** | | | |
| `enableHPA` | bool | — | Enable HorizontalPodAutoscaler |
| `minReplicas` | int | `1` | HPA min replicas |
| `maxReplicas` | int | `5` | HPA max replicas |
| `averageValue` | int | `20` | HPA RPS (requests per second) threshold |
| `enablePDB` | bool | — | Enable PodDisruptionBudget |
| `minAvailable` | int | `2` | PDB minimum available pods |
| **Storage** | | | |
| `mountPath` | string | — | Mount PVC to this path inside the container |
| **Environment** | | | |
| `requiredConfigMapEnv` | list | — | Non-sensitive vars REQUIRED for startup. Pod won't start if missing. |
| `optionalConfigMapEnv` | list | — | Non-sensitive vars that are OPTIONAL. |
| `requiredSecretEnv` | list | — | Sensitive vars (secrets) REQUIRED for startup. |
| `optionalSecretEnv` | list | — | Sensitive vars (secrets) that are OPTIONAL. |

---

## values.yaml Template

```yaml
global:
  productName: product-name         # Static — used for k8s resource naming
  appVersion: {{chartversion}}      # Replace key — substituted by pipeline
  client: service                   # Client/tenant identifier (overridden by CD pipeline)
  registry: your-acr.azurecr.io    # ACR registry host
  nodeEnv: production
  PORT: 8080
  NODE_ENV: production
  # environment: prd               # Set by CD pipeline Variable Group, not in defaults

service1:
  appName: service1                 # MUST match the top-level node name and chart dir
  enableIngress: true
  requestsCPU: "50m"
  requestsMEM: "256Mi"
  replicas: "1"
  containerPort: 8080
  healthcheck: "/health"            # Enables startup, readiness, and liveness probes
  requiredConfigMapEnv:
    - NODE_ENV
    - PORT
    # Add all required non-sensitive env vars
  optionalConfigMapEnv:
    - logLevel
    # Add all optional non-sensitive env vars
  requiredSecretEnv:
    - exampleSecretKey
    # Add all required sensitive env vars
  optionalSecretEnv: []
    # Add all optional sensitive env vars

# Repeat for each service...
```
