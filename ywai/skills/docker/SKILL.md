---
name: docker
description: Author and harden Dockerfiles for NestJS/Node, .NET, and Angular/nginx services on AKS. Use when creating a Dockerfile, reviewing/auditing/hardening one, shrinking image size or attack surface, or fixing container findings — root user, leaked secret, writable code, vuln scan. Reaches the .dockerignore, compose, and the entrypoints/configs baked into the image.
---

# Docker

Every image here runs **non-root** under the AKS **runtime contract** (below): read-only rootfs, dropped caps, least privilege. Lead with **distroless** runtime bases, **multi-stage** builds, minimal **attack surface**. Two ways in:

- **Author** a new image → copy a template, adapt, enforce the Hard Rules.
- **Review** an existing image → audit against [`references/review-checklist.md`](references/review-checklist.md), severize, plan, fix.

## Author: pick the template

Detect the stack, copy the matching template; never write a Dockerfile from scratch when one applies.

| Project signal | Template |
|---|---|
| `package.json` with a server entrypoint (NestJS, Express, Fastify) | `templates/node-backend.Dockerfile` |
| `angular.json`, or a Vite/React SPA (static output) | `templates/spa-nginx.Dockerfile` + `templates/spa-nginx.conf` |
| `*.csproj` / `global.json` (.NET) | `templates/dotnet-api.Dockerfile` |

Pair every image with its `.dockerignore` (`templates/node.dockerignore` / `templates/dotnet.dockerignore`). Adapt versions to the project — Node major from `engines`, .NET from `global.json` — and never silently bump the runtime major.

## Hard Rules

Both branches enforce these; a review reports each violation by number.

1. **First line `# syntax=docker/dockerfile:1`** — unlocks BuildKit cache mounts, secrets, `--chmod`.
2. **Pin the base by tag AND digest** (`node:24-alpine@sha256:…`). Resolve with `docker buildx imagetools inspect <img>:<tag>`; automate bumps with Renovate. No network → leave the tag plus a commented `@sha256:` placeholder and tell the user to resolve it.
3. **Stage layout `deps → build → prod-deps → runtime`.** The runtime stage holds only artifacts and production dependencies — never source, package managers, or compilers.
4. **Minimal runtime base**, in order of preference: Node → `gcr.io/distroless/nodejs<major>-debian12:nonroot`; .NET → `mcr.microsoft.com/dotnet/aspnet:<ver>-noble-chiseled`; SPA → `nginxinc/nginx-unprivileged:<ver>-alpine` (uid 101, port 8080). Fall back to `-alpine`/`-slim` only if the container genuinely needs a shell — say so in a comment.
5. **Non-root user, port > 1024.** Distroless `:nonroot`, chiseled, and nginx-unprivileged already comply; on any other base add an explicit `USER`.
6. **App code stays root-owned.** Never `COPY --chown=<runtime-user>` source or `node_modules` — the runtime user needs read-only access, and a compromised process must not be able to rewrite its own code. Only `/tmp` is writable (K8s `emptyDir`).
7. **npm always runs `--ignore-scripts --no-audit --no-fund`** with a cache mount (`--mount=type=cache,target=/root/.npm`), and `npm ci` over `npm install`. `--ignore-scripts` is supply-chain defence — never drop it to "fix" a build without flagging it.
8. **Exec-form `CMD`/`ENTRYPOINT`** (JSON array) so the process receives SIGTERM directly.
9. **No secrets in the image.** Never via `ARG`/`ENV`/`COPY` — not even self-signed dev certs or an `.npmrc`. A build-time secret uses `RUN --mount=type=secret`. The `.dockerignore` must exclude `.env*`, `.npmrc`, keys, and certs.
10. **No `HEALTHCHECK`.** AKS Helm probes own liveness/readiness and distroless has nothing to run it with; expose an HTTP `/health_check` endpoint instead.
11. **No image tags or registry names in the Dockerfile** — tagging is `{{chartversion}}` and pushing is the `docker-acr-build-and-push` pipeline (see `devops`).
12. **`EXPOSE` matches the Helm `containerPort`** in `values.yaml`.

## Runtime Contract (Helm / AKS)

Every image must run clean under this `securityContext` — if it can't, the Dockerfile is wrong, not the chart:

```yaml
securityContext:
  runAsNonRoot: true
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities: { drop: [ALL] }
  seccompProfile: { type: RuntimeDefault }
```

Anything written at runtime goes to `/tmp` via an `emptyDir` (e.g. the SPA `env.js` runtime-config pattern).

## Review: audit an existing image

1. **Discover the whole package**, not just `Dockerfile*`: every `.dockerignore` (one per build context), `docker-compose*.y*ml`, and what the image COPYs or runs — entrypoints (`*.sh`) and server configs (`nginx.conf`, `default.conf`). These ship inside the image, so they are attack surface.
2. **Read each file fully** — stages, base, ports, user, `COPY`, `ENV`/`ARG`, and the logic of every script and config.
3. **Evaluate against [`references/review-checklist.md`](references/review-checklist.md)** — every item. Each finding: `file:line`, severity, impact, concrete fix.
4. **Confirm each build context has its own `.dockerignore`** (not just the repo root).
5. **Report** as the table + iteration plan below.
6. **If asked, apply the safe fixes**, then verify with a real `docker build` and a smoke test: `docker run` + `id` (confirms non-root) + hit `/health_check`.

### Output format

```
## Findings
| # | file:line | Finding | Severity | Fix |

## Iteration plan
- Iteration 1 (no build break): non-root, port > 1024, .dockerignore per context, ${VAR} in compose.
- Iteration 2 (larger): digest-pin, minimal/supported base, read-only rootfs, drop the package manager from the runtime stage.
- Iteration 3 (modernise): replace EOL bases, wire a CI image scan (Trivy/Grype) that fails on critical CVEs, move any baked secret to BuildKit secrets / Secret Store.
```

A secret in the repo or in a layer is always **Critical** — even a self-signed dev cert.

## Our stance vs generic hardening advice

Generic guides relax two things; we don't — keep them from being "corrected":

- **No `HEALTHCHECK`** (Rule 10) — Helm probes own it on AKS; the instruction is dead weight and distroless can't run it.
- **No `--chown` of app code to the runtime user** (Rule 6) — root-owned code is the defence against a compromised process rewriting itself. Generic `COPY --chown=app …` patterns violate this.
