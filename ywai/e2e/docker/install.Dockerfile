# syntax=docker/dockerfile:1
# =============================================================================
# Tier B image: real install / update / uninstall cells.
#
# MODE build arg:
#   plain    the binary reads skills/agents from the repo checkout at /src
#            (data resolution walks up to go.mod) - /src holds a full copy
#   embedded a self-contained binary; /src stays empty so the binary must
#            rely on its embedded data
#
# The runtime keeps jq/curl/tar because the scenario assertions are
# filesystem-first (jq on opencode.json) and update-release downloads real
# releases. Build context is the repository root.
# =============================================================================
ARG GO_VERSION=1.26

# --- Builder -------------------------------------------------------------------
FROM golang:${GO_VERSION} AS builder
ARG MODE=plain
ARG YWAI_VERSION=0.0.0-e2e
# The old build differs only in its version string: update-swap needs a
# deterministic "previous binary" that needs no network to obtain.
ARG YWAI_VERSION_OLD=0.0.0-e2e-old

# Node 22 and bun come from their upstream images. The embedded MODE needs
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

# Embedded needs the embedded_data tree; plain reads data from disk at run
# time and skips the preparation entirely.
RUN if [ "$MODE" = "embedded" ]; then \
        npm --prefix internal/control/web ci --no-audit --no-fund \
        && bash scripts/prepare-embedded.sh; \
    fi

RUN go build -ldflags "-s -w -X main.version=${YWAI_VERSION}" \
        -o /out/ywai ./cmd/ywai \
 && go build -ldflags "-X main.version=${YWAI_VERSION_OLD}" \
        -o /out/ywai-old ./cmd/ywai

# MODE=plain keeps a repo checkout for run time; embedded leaves /src empty.
RUN mkdir -p /src \
 && if [ "$MODE" = "plain" ]; then cp -a /build/ywai/. /src/; fi

# --- Runtime -------------------------------------------------------------------
FROM ubuntu:24.04
ARG MODE=plain
ARG YWAI_VERSION=0.0.0-e2e
ARG YWAI_VERSION_OLD=0.0.0-e2e-old

ENV HOME=/home/ywai \
    DEBIAN_FRONTEND=noninteractive \
    YWAI_MODE=${MODE} \
    YWAI_VERSION_NEW=${YWAI_VERSION} \
    YWAI_VERSION_OLD=${YWAI_VERSION_OLD} \
    YWAI_BIN_NEW=/opt/ywai-e2e/bin/ywai-new \
    YWAI_BIN_OLD=/opt/ywai-e2e/bin/ywai-old

RUN apt-get update \
 && apt-get install -y --no-install-recommends jq curl ca-certificates tar \
 && rm -rf /var/lib/apt/lists/* \
 && groupadd --gid 10001 ywai \
 && useradd --uid 10001 --gid 10001 --create-home \
        --home-dir /home/ywai --shell /usr/sbin/nologin ywai

COPY --from=builder /out/ywai /usr/local/bin/ywai
COPY --from=builder /out/ywai /opt/ywai-e2e/bin/ywai-new
COPY --from=builder /out/ywai-old /opt/ywai-e2e/bin/ywai-old
COPY --from=builder /src/ /src/
COPY ywai/e2e/docker/fakes/opencode2.sh /usr/local/bin/opencode2
COPY ywai/e2e/docker/fakes/claude.sh /usr/local/bin/claude
COPY ywai/e2e/docker/lib /opt/ywai-e2e/lib
COPY ywai/e2e/docker/scenarios /opt/ywai-e2e/scenarios

# update-swap and update-release replace the binary on PATH, so the cell
# user owns it.
RUN chmod 0755 /usr/local/bin/ywai /opt/ywai-e2e/bin/ywai-new \
        /opt/ywai-e2e/bin/ywai-old /usr/local/bin/opencode2 /usr/local/bin/claude \
 && chown ywai:ywai /usr/local/bin/ywai

USER ywai
WORKDIR /home/ywai
