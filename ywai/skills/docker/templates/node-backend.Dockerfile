# syntax=docker/dockerfile:1
# Node/NestJS backend — hardened multi-stage build.
# ADAPT: Node major must match package.json "engines"; resolve <digest> per SKILL.md "Digest Pinning".
ARG NODE_IMAGE=node:24-alpine@sha256:<digest>

FROM ${NODE_IMAGE} AS deps
WORKDIR /app
COPY package*.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --ignore-scripts --no-audit --no-fund

FROM ${NODE_IMAGE} AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM ${NODE_IMAGE} AS prod-deps
WORKDIR /app
COPY package*.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --omit=dev --ignore-scripts --no-audit --no-fund

# Distroless: no shell, no npm, no apk. Debug with `kubectl debug` (ephemeral containers).
# Runs as nonroot (uid 65532). App files stay root-owned on purpose (read-only to the app).
FROM gcr.io/distroless/nodejs24-debian12:nonroot AS runtime
WORKDIR /app
ENV NODE_ENV=production
COPY --from=prod-deps /app/node_modules ./node_modules
COPY --from=build /app/dist ./dist
# ADAPT: must match the Helm containerPort for this service.
EXPOSE 3000
# Entrypoint of the distroless image is `node`; CMD is just the script.
CMD ["dist/main.js"]
