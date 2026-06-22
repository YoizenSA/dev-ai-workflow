# Review checklist

The exhaustive checklist for the **review** branch of the `docker` skill. Walk every item against every file discovered in step 1. The Hard Rules in `SKILL.md` are the non-negotiable subset; this is the full surface, including what ships *around* the Dockerfile (compose, entrypoints, server configs).

Severity guide: **Critical** = secret in repo/layer, root at runtime. **High** = writable app code, EOL base, no digest pin. **Medium** = missing CI scan, copying source before deps, sprawl. **Low** = missing labels/metadata.

## A. Base image & supply chain
- [ ] Base is **supported, not EOL** (check endoflife.date; .NET warns `NETSDK1138`). Flag `node:14`, `dotnet/core:3.1`, etc. — High.
- [ ] **Digest-pinned** (`img:tag@sha256:…`) on top of the tag, for reproducible builds (Rule 2) — High.
- [ ] Runtime stage uses a **minimal base** — distroless / chiseled / `-unprivileged`, never the build base (Rule 4).
- [ ] Pulled from a **trusted registry** (official / MCR / private ACR).
- [ ] **CI image scan** (Trivy/Grype) wired into the pipeline, failing on critical CVEs — Medium (lives in the `devops` pipeline, not the Dockerfile).

## B. Multi-stage & dependencies
- [ ] Runtime stage carries **no SDK, compiler, or build artifacts** (Rule 3).
- [ ] **Deterministic install**: `npm ci` + lockfile, `dotnet restore`, hashed pip. For npm, `--ignore-scripts --no-audit --no-fund` + cache mount (Rule 7).
- [ ] Manifests copied and deps installed **before** the source (`COPY package*.json` → install → `COPY . .`) so the dependency layer caches.
- [ ] `COPY` scoped by `.dockerignore`; nothing copied that the image doesn't need.

## C. User & permissions
- [ ] **Non-root `USER`**, port > 1024 (Rule 5).
- [ ] **App code root-owned** — no `--chown` to the runtime user (Rule 6). This is the spot generic guides get wrong.
- [ ] Only `/tmp` (or an explicit `emptyDir` path) is writable; rootfs is read-only.

## D. Runtime hardening
- [ ] **No secrets** in `ENV`/`ARG`/layers — dev certs, `.npmrc` tokens, and keys all count (Rule 9). A secret in the repo or a layer is **Critical**.
- [ ] **No `HEALTHCHECK`** — Helm probes own it (Rule 10); expose `/health_check`.
- [ ] **Exec-form** `ENTRYPOINT`/`CMD`, explicit `WORKDIR` (Rule 8).
- [ ] `EXPOSE` minimal and matching the Helm `containerPort` (Rule 12).
- [ ] OCI labels (`org.opencontainers.image.*`) for provenance — Low.

## E. docker-compose (dev / local)
- [ ] No plaintext credentials → `${VAR}` interpolation + a committed `.env.example`, with `.env` git-ignored.
- [ ] `security_opt: [no-new-privileges:true]`, `cap_drop: [ALL]`, `read_only: true` where the service allows, `tmpfs` for the temp paths it still needs.
- [ ] Only the necessary host ports published; internal services left unexposed.

## F. .dockerignore (one per build context)
- [ ] Exists for **every** context, not just the repo root.
- [ ] Excludes `.git`, `node_modules`, `bin/`/`obj/`, `dist`, `.env*`, `.npmrc`, keys/certs, tests, IDE files.

## G. Entrypoints / scripts (.sh) that ship in the image
- [ ] `#!/bin/sh` + `set -eu` (and `set -o pipefail` where the shell supports it).
- [ ] **No injection** when env values are written into generated files (e.g. `env.js`): escape per destination context (JS/JSON/YAML) so a value can't break the string or inject code. Validate any value used in `sed`.
- [ ] No `eval` on external data; quote every expansion (`"$VAR"`).
- [ ] Never log secrets, never write them into client-served files.
- [ ] Runs non-root and writes only to paths that user owns.
- [ ] Ends on `exec "$@"` so PID 1 forwards signals.

## H. Server / reverse-proxy config (nginx, httpd) in the image
- [ ] **Security headers**: `X-Content-Type-Options: nosniff`, `X-Frame-Options`, `Referrer-Policy`, and a `Content-Security-Policy` (validate against external deps — e.g. a Google login — before tightening). Backend headers don't apply when nginx serves — set them in nginx. Drop `X-XSS-Protection`; use CSP instead.
- [ ] `server_tokens off;` so the version doesn't leak.
- [ ] `client_max_body_size` sized to real uploads — neither a DoS vector nor a block on legit ones.
- [ ] Listens on > 1024 when non-root; TLS normally terminates at the ingress, not here.
- [ ] Upstreams resolved resiliently (variable + `resolver`), destination configurable by env — don't hardcode service names.
- [ ] A `location` with its own `add_header` re-declares the server headers (they do not inherit).

## Escape snippet (anti-injection in an entrypoint)

```sh
js_escape() { printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g; s/</\\u003c/g'; }
VAL_ESC="$(js_escape "${SOME_ENV:-}")"     # inject "$VAL_ESC" inside the JS string
case "$UPSTREAM" in *[!A-Za-z0-9.:_-]*) echo "invalid value" >&2; exit 1 ;; esac
```
