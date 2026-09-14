# syntax=docker/dockerfile:1
# =============================================================================
# Tier A image: install --dry-run cells.
#
# Run time is networkless (compose sets network_mode: none) and toolless:
# no git, go, node or jq exists on PATH, which is the point. A dry run must
# complete and write nothing with only the binary and the fake agents
# present. Build context is the repository root.
# =============================================================================
ARG GO_VERSION=1.26

# --- Builder: embed skills/agents/plugins and compile the binary -------------
FROM golang:${GO_VERSION} AS builder
ARG YWAI_VERSION=0.0.0-e2e-dryrun

# Node 22 and bun come from their upstream images. prepare-embedded.sh needs
# them to build the control UI and the plugin bundles it embeds. npm is
# re-linked by hand: COPY of a single symlinked file would dereference it.
COPY --from=node:22 /usr/local/bin/node /usr/local/bin/node
COPY --from=node:22 /usr/local/lib/node_modules /usr/local/lib/node_modules
COPY --from=oven/bun:latest /usr/local/bin/bun /usr/local/bin/bun
RUN ln -sf ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm \
 && ln -sf ../lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx

WORKDIR /build/ywai
COPY ywai/ /build/ywai/
COPY docs/ /build/docs/

RUN npm --prefix internal/control/web ci --no-audit --no-fund \
 && bash scripts/prepare-embedded.sh

RUN go build -tags embedded -ldflags "-s -w -X main.version=${YWAI_VERSION}" \
        -o /out/ywai ./cmd/ywai

# --- Runtime: ubuntu + binary + fake agents. Nothing else. --------------------
FROM ubuntu:24.04
ENV HOME=/home/ywai

RUN groupadd --gid 10001 ywai \
 && useradd --uid 10001 --gid 10001 --create-home \
        --home-dir /home/ywai --shell /usr/sbin/nologin ywai

COPY --from=builder /out/ywai /usr/local/bin/ywai
COPY ywai/e2e/docker/fakes/opencode2.sh /usr/local/bin/opencode2
COPY ywai/e2e/docker/fakes/claude.sh /usr/local/bin/claude
COPY ywai/e2e/docker/lib /opt/ywai-e2e/lib
COPY ywai/e2e/docker/scenarios /opt/ywai-e2e/scenarios

RUN chmod 0755 /usr/local/bin/ywai /usr/local/bin/opencode2 /usr/local/bin/claude

USER ywai
WORKDIR /home/ywai
