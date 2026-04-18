# metronous-setup

Install-step extension that installs and configures [Metronous](https://github.com/kiosvantra/metronous):
a tool for AI agent telemetry, benchmarking, and model calibration for OpenCode agents.

## What it does

1. Installs the `metronous` CLI (via official install script from GitHub releases).
2. Runs `metronous install` which automatically:
   - Configures OpenCode with the metronous MCP shim
   - Installs the metronous TypeScript plugin to `~/.config/opencode/plugins/`
   - Sets up the daemon service (systemd on Linux, service on Windows)
3. Creates `.ywai/metronous/README.md` with usage notes and next steps.

## Opt-in

This step is **opt-in**. It is only executed when:
- the wizard enables it (checkbox), or
- the `--install-metronous` flag is set, or
- `metronous-setup` is listed in the active type's `extensions.install-steps`

## Skip

Use `--skip-metronous` to skip this step even if it is listed in the type config.

## Platform support

- **Linux/macOS**: Official support via one-command install script
- **Windows**: Experimental support via manual install flow

## References

- Repo: https://github.com/kiosvantra/metronous
- Docs: https://github.com/kiosvantra/metronous (see README.md)
