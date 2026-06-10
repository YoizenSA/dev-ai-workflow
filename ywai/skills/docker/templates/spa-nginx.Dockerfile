# syntax=docker/dockerfile:1
# Angular/React SPA served by nginx-unprivileged — hardened multi-stage build.
# ADAPT: Node major must match package.json "engines"; resolve <digest> per SKILL.md "Digest Pinning".
ARG NODE_IMAGE=node:24-alpine@sha256:<digest>

FROM ${NODE_IMAGE} AS build
WORKDIR /app
COPY package*.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --ignore-scripts --no-audit --no-fund
COPY . .
RUN npm run build

# nginx-unprivileged runs as uid 101 and listens on 8080 — non-root out of the box.
FROM nginxinc/nginx-unprivileged:1.29-alpine@sha256:<digest> AS runtime
# No --chown: html and config stay root-owned; nginx only needs read access.
# ADAPT output path: Angular → dist/<project>/browser ; Vite/React → dist
COPY --from=build /app/dist/<project>/browser /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
# Optional runtime-config pattern: a script in /docker-entrypoint.d/ generates /tmp/env.js
# from env vars before nginx starts; index.html loads <script src="/env.js">.
# /tmp must be an emptyDir mount when readOnlyRootFilesystem is on.
# COPY --chmod=555 docker-entrypoint.sh /docker-entrypoint.d/40-generate-env.sh
EXPOSE 8080
