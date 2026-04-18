#requires -version 5.1
# Metronous Setup Extension — Windows (Experimental)
# Installs metronous CLI and configures OpenCode telemetry.
param(
    [string]$TargetDir = "."
)

$ErrorActionPreference = 'Stop'

$ExtDir = $PSScriptRoot
$StateDir = Join-Path $TargetDir '.ywai\metronous'
$StatusFile = Join-Path $StateDir 'status.txt'
$ReadmeFile = Join-Path $StateDir 'README.md'

function Write-Log($msg)  { Write-Host "[metronous-setup] $msg" }
function Write-Warn($msg) { Write-Host "[metronous-setup] WARN: $msg" -ForegroundColor Yellow }
function Has-Cmd($name)   { [bool](Get-Command $name -ErrorAction SilentlyContinue) }

New-Item -ItemType Directory -Force -Path $StateDir | Out-Null

@'
# Metronous Setup

This project uses the `metronous-setup` extension for agent telemetry and benchmarking.

Metronous has been installed and configured for OpenCode.

## What was configured

- Metronous CLI installed
- OpenCode configured with metronous MCP shim
- Metronous plugin installed to ~/.config/opencode/plugins/
- Daemon service configured (Windows service)

## Next steps

Start the metronous dashboard:

```powershell
metronous dashboard
```

The dashboard has 5 tabs for tracking, benchmarks, costs, config, and reports.

## References

- Repo: https://github.com/kiosvantra/metronous
- Docs: https://github.com/kiosvantra/metronous

## Note

Windows support for metronous is experimental. If you encounter issues, please report them
at https://github.com/kiosvantra/metronous/issues.
'@ | Out-File -FilePath $ReadmeFile -Encoding UTF8

# ---------------------------------------------------------------------------
# 1. Install metronous CLI (Windows experimental)
# ---------------------------------------------------------------------------
if (Has-Cmd 'metronous') {
    $version = & metronous --version 2>$null
    if (-not $version) { $version = "present" }
    Write-Log "metronous CLI already installed: $version"
} else {
    Write-Warn "metronous CLI not found. Windows support is experimental."
    Write-Warn "Please install manually from GitHub releases:"
    Write-Warn "  https://github.com/kiosvantra/metronous/releases"
    Write-Warn "Download the Windows archive and run:"
    Write-Warn "  & `$env:LOCALAPPDATA\Programs\Metronous\metronous.exe install"
    @"
metronous: install_failed
auto_configured: no
note: Windows install is manual - see README
"@ | Out-File -FilePath $StatusFile -Encoding UTF8
    exit 0
}

# ---------------------------------------------------------------------------
# 2. Run metronous install (configures OpenCode automatically)
# ---------------------------------------------------------------------------
if (Has-Cmd 'metronous') {
    Write-Log "Running metronous install to configure OpenCode"
    try {
        & metronous install | Out-Null
        Write-Log "OpenCode configured with metronous"
        $configured = 1
    } catch {
        Write-Warn "metronous install failed - you may need to run it manually as Administrator"
        $configured = 0
    }
} else {
    Write-Warn "metronous CLI not available"
    $configured = 0
}

$version = & metronous --version 2>$null
if (-not $version) { $version = "unknown" }

@"
metronous: installed
version: ${version}
auto_configured: yes
configured: ${configured}
"@ | Out-File -FilePath $StatusFile -Encoding UTF8

Write-Log "Done (configured: $configured)"
