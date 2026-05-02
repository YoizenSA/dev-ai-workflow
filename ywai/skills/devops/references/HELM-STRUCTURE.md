# Helm Chart Structure — Detailed Reference

## Umbrella Chart Pattern

The project uses an **Umbrella Helm chart**. The umbrella declares `common-helpers` as its only explicit dependency. Sub-charts live inside the `charts/` directory and are auto-discovered by Helm (they are NOT listed as `file://` dependencies in `Chart.yaml`).

```
DevOps/Helm/
├── Chart.yaml                  # Umbrella chart — declares common-helpers dependency only
├── values.yaml                 # Unified values for all services + global config
├── templates/
│   └── utils.yaml              # ConfigMap, Secret, PV, PVC (umbrella-level resources)
└── charts/
    ├── service1/
    │   ├── Chart.yaml          # Lightweight chart (apiVersion v1, no dependencies)
    │   └── templates/
    │       ├── deployment.yaml # Deployment via common-helpers
    │       └── utils.yaml      # Service, Ingress, PDB, HPA via common-helpers
    ├── service2/               # Same structure as service1
    └── service3/               # Same structure as service1
```

---

## Umbrella `Chart.yaml`

```yaml
apiVersion: v2
name: product-name
description: Helm chart for Product Name.
version: {{chartversion}}         # Replace key — substituted by pipeline
appVersion: {{chartversion}}
type: application

dependencies:
  - name: common-helpers
    version: "x.x.x"
    repository: "oci://your-acr.azurecr.io/helm"
```

> **Important**: Sub-charts are NOT listed as dependencies. They are auto-discovered from the `charts/` directory.

---

## Sub-chart `Chart.yaml`

Sub-charts use `apiVersion: v1` and declare **no dependencies** — `common-helpers` is inherited from the umbrella.

```yaml
# DevOps/Helm/charts/service1/Chart.yaml
apiVersion: v1
name: service1                    # MUST match the directory name and the values.yaml node name
description: Helm chart for child chart.
version: {{chartversion}}
type: application
```

---

## Sub-chart Templates

Both files are **generic** — no manual modification needed per service. Copy them as-is for each new sub-chart.

### `templates/deployment.yaml`

```yaml
{{ include "common-helpers.deploymenttemplate" . }}
```

### `templates/utils.yaml`

Renders Service + conditional Ingress, PDB, HPA:

```yaml
{{ include "common-helpers.servicetemplate" . }}
---
{{- if .Values.enableIngress }}
{{ include "common-helpers.ingresstemplate" . }}
---
{{- end }}
{{- if .Values.enablePDB }}
{{ include "common-helpers.pdb" . }}
---
{{- end }}
{{- if .Values.enableHPA }}
{{ include "common-helpers.hpa" . }}
---
{{- end }}
```

---

## Umbrella `templates/utils.yaml`

Renders shared ConfigMap, Secret, and optionally PV/PVC at the umbrella level:

```yaml
{{ include "common-helpers.requiredConfigmap" . }}
---
{{ $.Values.secretYaml | nindent 0 }}
---
{{- if .Values.global.enablePV }}
{{ include "common-helpers.pv" . }}
---
{{- end }}
{{- if .Values.global.enablePVC }}
{{ include "common-helpers.pvc" . }}
---
{{- end }}
```

---

## `common-helpers` Library

`common-helpers` is a shared Helm library chart (`oci://your-acr.azurecr.io/helm/common-helpers`) that provides all Kubernetes resource templates. It is declared only at the **umbrella level** and inherited by all sub-charts.

### Resource Naming Convention

All resources follow the pattern: `{productName}-{client}-{environment}-{appName}`

| Resource | Suffix |
|----------|--------|
| Service | `-svc` |
| Ingress | `-ing` |
| HPA | `-hpa` |
| ConfigMap | `-generic-configmap` (shared across services) |
| Secret | `-generic-secret` (shared across services) |

Values come from `global.*` and the service node's `appName`.

Default image: `{global.registry}/{global.productName}-{appName}:{Chart.Version}` — override per service with `.Values.image`.

### Template Reference

| Template | Location | Purpose |
|----------|----------|---------|
| `common-helpers.deploymenttemplate` | Sub-chart `deployment.yaml` | Kubernetes Deployment (RollingUpdate: maxSurge 10%, maxUnavailable 0) |
| `common-helpers.servicetemplate` | Sub-chart `utils.yaml` | Kubernetes Service (default: ClusterIP, port 80 → containerPort) |
| `common-helpers.ingresstemplate` | Sub-chart `utils.yaml` | Ingress (conditional: `enableIngress`). Supports single or multiple URLs. |
| `common-helpers.pdb` | Sub-chart `utils.yaml` | PodDisruptionBudget (conditional: `enablePDB`, default minAvailable: 2) |
| `common-helpers.hpa` | Sub-chart `utils.yaml` | HorizontalPodAutoscaler (conditional: `enableHPA`). Metrics: CPU, Memory, RPS. |
| `common-helpers.cronJob` | Sub-chart `deployment.yaml` | CronJob (use instead of `deploymenttemplate` for scheduled workloads) |
| `common-helpers.requiredConfigmap` | Umbrella `utils.yaml` | ConfigMap aggregating all `requiredConfigMapEnv` + `optionalConfigMapEnv` keys from global |
| `secretYaml` (raw value) | Umbrella `utils.yaml` | SealedSecret YAML injected at deploy time by CD pipeline |
| `common-helpers.pv` | Umbrella `utils.yaml` | PersistentVolume — Azure Blob NFS (conditional: `global.enablePV`) |
| `common-helpers.pvc` | Umbrella `utils.yaml` | PersistentVolumeClaim (conditional: `global.enablePVC`) |

### Deployment Features (`deploymenttemplate`)

- **Health probes**: If `healthcheck` is set, generates `startupProbe`, `readinessProbe`, and `livenessProbe` (all HTTP GET)
- **Resources**: Requests (`requestsCPU`/`requestsMEM`) and limits (`limitsCPU`/`limitMEM`) — only rendered if at least one is set
- **Volume mounts**: If `mountPath` is set, mounts a PVC named `pvc-{productName}-{client}-{environment}`
- **Affinity**: If `affinity` or `global.nodeSelector` is set, creates node affinity for `kubernetes.azure.com/agentpool`
- **Tolerations**: If `tolerations` or `global.nodeSelector` is set, creates workload toleration
- **Image pull secrets**: If `global.imagePullSecrets` is set, adds to pod spec
- **Replicas**: Controlled by `replicas` unless `enableHPA` is true (then managed by HPA)
