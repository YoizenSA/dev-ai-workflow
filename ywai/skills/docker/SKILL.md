---
name: docker
description: 'Hardened multi-stage Dockerfiles (NestJS/Node 24 backends, .NET 10 APIs, Angular SPAs on nginx). Trigger: creating or reviewing a Dockerfile, containerizing a service, image size or security hardening.'
---

## When to Use

- Create a Dockerfile for a new service
- Review or harden an existing Dockerfile
- Reduce image size or attack surface
- Fix container security findings (root user, writable code, leaked secrets, vuln scans)

---

## Pick the Template

Detect the stack from the repo, copy the matching template, then adapt and run the checklist. Never write a Dockerfile from scratch when a template applies.

| Project signal | Template |
|---|---|
| `package.json` with a server entrypoint (NestJS, Express, Fastify) | `templates/node-backend.Dockerfile` |
| `angular.json`, or Vite/React SPA (static build output) | `templates/spa-nginx.Dockerfile` + `templates/spa-nginx.conf` |
| `*.csproj` / `global.json` (.NET) | `templates/dotnet-api.Dockerfile` |

Always pair with the matching `.dockerignore`: `templates/node.dockerignore` or `templates/dotnet.dockerignore`.

Adapt versions to the project — Node major from `package.json` `engines`, .NET SDK from `global.json`/`TargetFramework`. Do not silently upgrade the project's runtime major.

---

## Hard Rules

1. **First line is `# syntax=docker/dockerfile:1`** — enables BuildKit features (cache mounts, secrets, `--chmod`).
2. **Pin base images by tag AND digest** (`node:24-alpine@sha256:...`). Tags are mutable; digests make builds reproducible. See "Digest pinning" below.
3. **Stage layout**: `deps → build → prod-deps → runtime`. The runtime stage contains only artifacts and production dependencies — never source code, package managers, or compilers.
4. **Minimal runtime base, in this order of preference**:
   - Node backend → `gcr.io/distroless/nodejs<major>-debian12:nonroot` (no shell, no npm)
   - .NET API → `mcr.microsoft.com/dotnet/aspnet:<ver>-noble-chiseled` (no shell, non-root by default)
   - SPA → `nginxinc/nginx-unprivileged:<ver>-alpine` (runs as uid 101, port 8080)
   - Fall back to `*-alpine`/`*-slim` only if the team needs a shell in the container (e.g. no ephemeral-container debugging available) — say so explicitly in a comment.
5. **Non-root user, port > 1024.** Distroless `:nonroot`, chiseled, and nginx-unprivileged already satisfy this; with other bases add an explicit `USER`.
6. **App files stay root-owned.** NEVER `COPY --chown=<runtime-user>` application code or `node_modules`: the runtime user only needs read access, and a compromised process must not be able to rewrite its own code. The only writable path is `/tmp` (backed by `emptyDir` in K8s).
7. **npm always runs with `--ignore-scripts --no-audit --no-fund`** and a BuildKit cache mount (`--mount=type=cache,target=/root/.npm`). `--ignore-scripts` is supply-chain protection; never remove it to "fix" a build without flagging it.
8. **Exec-form `CMD`/`ENTRYPOINT`** (JSON array) so the process receives SIGTERM directly.
9. **No secrets in the image.** Never pass secrets via `ARG`/`ENV`/`COPY`. If a build step needs one (private registry token), use `RUN --mount=type=secret,...`. The `.dockerignore` must exclude `.env*`, `.npmrc`, keys, and certs.
10. **No `HEALTHCHECK` instruction.** Services run on AKS where Helm liveness/readiness probes own this (see `devops` skill `healthcheck` values); the instruction is ignored by Kubernetes and distroless images have nothing to execute it with. Expose an HTTP `/health_check` endpoint instead.
11. **No image tags or registry names in the Dockerfile.** Tagging is `{{chartversion}}` and pushing is the `docker-acr-build-and-push` pipeline template (see `devops` skill).
12. **`EXPOSE` must match the Helm `containerPort`** in `values.yaml`.

---

## Runtime Contract (Helm / AKS)

Every image built with these templates must run cleanly under this `securityContext` — if it can't, the Dockerfile is wrong, not the chart:

```yaml
securityContext:
  runAsNonRoot: true
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities: { drop: [ALL] }
  seccompProfile: { type: RuntimeDefault }
```

Anything the app writes at runtime goes to `/tmp` via an `emptyDir` mount (e.g. the SPA `env.js` runtime-config pattern).

---

## Digest Pinning

```bash
docker buildx imagetools inspect node:24-alpine   # prints the manifest digest
```

Use the multi-arch index digest. If you have no network access during the task, leave the tag pinned, keep the `@sha256:<digest>` placeholder commented, and tell the user to resolve it. Digest bumps should be automated (Renovate/Dependabot) — mention this if the repo has neither.

---

## Review Checklist

When reviewing an existing Dockerfile, check each item and report violations with the rule number:

- [ ] `# syntax=docker/dockerfile:1` present
- [ ] Base images digest-pinned; runtime base is distroless/chiseled/unprivileged
- [ ] Runtime stage has no package manager, compiler, or source code
- [ ] Non-root user; port > 1024
- [ ] App code root-owned (no `--chown` to the runtime user)
- [ ] npm: `--ignore-scripts --no-audit --no-fund` + cache mount; deps install in their own stage (layer caching)
- [ ] Exec-form `CMD`/`ENTRYPOINT`
- [ ] `.dockerignore` exists and excludes secrets, `.git`, `node_modules`/`bin`/`obj`, build output
- [ ] No secrets in `ARG`/`ENV`/layers
- [ ] No `HEALTHCHECK`; `EXPOSE` matches Helm `containerPort`

---

## Anti-patterns → Fix

| Found | Replace with |
|---|---|
| `FROM node:latest` / tag-only pin | Tag + digest pin (Rule 2) |
| `npm install` | `npm ci --ignore-scripts --no-audit --no-fund` |
| `npm ci` in the runtime stage | Separate `prod-deps` stage; runtime only COPYs `node_modules` |
| `COPY . .` before installing deps | `COPY package*.json ./` → install → then `COPY . .` |
| `COPY --chown=node:node dist ./dist` | `COPY dist ./dist` (root-owned, Rule 6) |
| `USER root` or no `USER` on a shell-based image | Non-root user (Rule 5) |
| `CMD npm start` | `CMD ["node", "dist/main.js"]` — exec form, no npm wrapper |
| `ENV API_KEY=...` / `ARG TOKEN` | `RUN --mount=type=secret` or runtime env from K8s secrets |
| `HEALTHCHECK CMD curl ...` | Remove; Helm probes (Rule 10) |
| `apk add curl` (or similar) in runtime stage | Remove — debug with `kubectl debug` ephemeral containers |
| `X-XSS-Protection` header in nginx.conf | Remove it; set a `Content-Security-Policy` instead |
