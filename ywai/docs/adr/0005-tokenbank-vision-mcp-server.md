# ADR-005: Tokenbank Vision MCP Server

**Status:** Proposed
**Date:** 2026-07-08

---

## Context

Non-vision AI models (like DeepSeek V4 Pro) cannot process images. Tokenbank has vision-capable models (MiMo V2.5, kimi-k2.6, qwen3.6-plus) that can. We need an MCP server that bridges this gap: a text-only model calls an MCP tool, the tool routes the image to a Tokenbank vision model, and returns the text analysis back.

## Decision

Build a stdio MCP server in Go that exposes vision tools backed by Tokenbank's vision-capable models.

### Models

- **Consumer**: DeepSeek V4 Pro (`deepseek-v4-pro`) — text-only model that calls the MCP tool
- **Principal vision model**: MiMo V2.5 (`mimo-v2.5`) — cheapest ($0.14/$0.28 per M tokens), broadest modality support (text, image, audio, video)
- **Fallback vision models**: `kimi-k2.6`, `qwen3.6-plus`, `mimo-v2.5-pro`

### Transport

stdio — JSON-RPC 2.0 over stdin/stdout. The AI agent (opencode) spawns the MCP server as a subprocess. No port, no daemon, no auth layer needed.

### API Integration

- Endpoint: `POST {TokenBankURL}/v1/chat/completions` (OpenAI-compatible)
- Auth: `Authorization: Bearer {TokenBankAPIKey}`
- Image format: OpenAI-compatible content array with `type: "image_url"` and `image_url.url` as `data:image/png;base64,...`
- Config source: ywai config at `~/.config/ywai/userconfig.yaml` for `TokenBankURL` and `TokenBankAPIKey`

### MCP Tools

1. `analyze_image` — Send an image (base64) + a question/prompt, get back text analysis from the vision model. Optional `model` parameter to override default (MiMo V2.5).
2. `list_vision_models` — List available vision-capable models in Tokenbank.

### Flow

```
DeepSeek V4 Pro (text-only)
  → calls MCP tool `analyze_image` with base64 image + prompt
  → MCP server sends POST to Tokenbank /v1/chat/completions with model=mimo-v2.5
  → Tokenbank routes to MiMo V2.5 (vision)
  → Returns text response
  → MCP server returns text to DeepSeek V4 Pro
```

## Alternatives Considered

1. **Remote HTTP MCP server** — rejected: overkill for single-agent use, adds port/lifecycle/auth complexity.
2. **Native vision in every model** — rejected: not all models support vision, and the whole point is bridging text-only models.
3. **Direct API calls without MCP** — rejected: MCP gives a standard tool interface any agent can use without custom integration.

## Consequences

- **Cost**: Each image call costs ~$0.14/M input + $0.28/M output (MiMo V2.5 pricing).
- **Latency**: One extra round-trip through Tokenbank per image.
- **Dependency**: Requires Tokenbank to be running and accessible.
- **Config**: Requires `TokenBankURL` and `TokenBankAPIKey` in ywai config.

## Risks and Mitigations

- **Tokenbank unavailable**: Return clear error to the calling model so it can inform the user.
- **Large images**: Base64 encoding inflates payload size — enforce size limit (e.g. 10MB) and return error if exceeded.
- **Model selection**: Default to MiMo V2.5 (cheapest), allow override via `model` parameter in `analyze_image`.
