# ywai Feature Plans — opencode-manager Gap Analysis

This directory contains Architecture Decision Records (ADRs) and implementation plans for features that **ywai** is missing compared to [opencode-manager](https://github.com/chriswritescode-dev/opencode-manager).

## Feature Areas

| # | Feature | ADR | Implementation Plan | Priority | Effort |
|---|---------|-----|-------------------|----------|--------|
| 1 | Multi-repo management hub | [ADR-001](./adr-001-multi-repo-hub.md) | [Plan](./plan-multi-repo-hub.md) | High | Medium |
| 2 | Session CLI (`ocm-cli`) | [ADR-002](./adr-002-session-cli.md) | [Plan](./plan-session-cli.md) | High | Medium |
| 3 | Session scheduling & calendar view | [ADR-003](./adr-003-session-scheduling.md) | [Plan](./plan-session-scheduling.md) | Medium | Large |
| 4 | Health monitoring for repos & agents | [ADR-005](./adr-005-health-monitoring.md) | [Plan](./plan-health-monitoring.md) | Low | Small |
| 5 | omp harness ideas → ywai contracts | — (Fase 0) | [Plan](./plan-omp-harness-ideas.md) | High | Medium |
| 6 | ywai-fastfs MCP (in-process search/read) | — | [Plan](./plan-fastfs-mcp.md) | High | Medium |

## Effort Estimates

| Effort | Calendar Days | Description |
|--------|--------------|-------------|
| Small | 2–3 days | Single focused sprint, one developer |
| Medium | 4–8 days | Cross-cutting: backend + frontend + CLI |
| Large | 10–15 days | Complex: scheduling engine, calendar UI, background jobs |

## Dependencies Between Features

```
Multi-repo hub ──► Session CLI ──► Session Scheduling
      │                                 │
      └──► Health Monitoring            │
                                         ▼
                                   Calendar View
```

- **Multi-repo hub** is the foundation — every other feature references it.
- **Session CLI** depends on sessions existing in a multi-repo context.
- **Session scheduling** depends on the Session CLI model plus a scheduler engine.
- **Health monitoring** is standalone but benefits from multi-repo data.
