#requires -version 5.1
# Metronous Setup Extension - Windows (Experimental)
# Installs metronous CLI and configures OpenCode telemetry.
param(
    [string]$TargetDir = "."
)

$ErrorActionPreference = 'Stop'

$ExtDir = $PSScriptRoot

# In GlobalOnly mode the caller passes /tmp (Linux convention) - remap to TEMP on Windows.
if ($TargetDir -eq '/tmp' -or $TargetDir -eq '\tmp') { $TargetDir = $env:TEMP }

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

- Metronous CLI installed to %LOCALAPPDATA%\Programs\Metronous\
- OpenCode configured with metronous MCP shim
- Metronous plugin installed to ~/.config/opencode/plugins/
- Windows service registered (requires Administrator)

## Next steps

Start the metronous dashboard:

```powershell
metronous dashboard
```

The dashboard has 5 tabs for tracking, benchmarks, costs, config, and reports.

## Service control

```powershell
metronous service start
metronous service stop
metronous service status
metronous service uninstall
```

## References

- Repo: https://github.com/kiosvantra/metronous
- Docs: https://github.com/kiosvantra/metronous

## Note

Windows support for metronous is experimental. Run PowerShell as Administrator for
`metronous install` to register the Windows service. Report issues at
https://github.com/kiosvantra/metronous/issues.
'@ | Out-File -FilePath $ReadmeFile -Encoding UTF8

# ---------------------------------------------------------------------------
# 1. Install metronous CLI
# ---------------------------------------------------------------------------
$InstallDir = Join-Path $env:LOCALAPPDATA 'Programs\Metronous'
$MetronousExe = Join-Path $InstallDir 'metronous.exe'

if (Has-Cmd 'metronous') {
    $version = & metronous --version 2>$null
    if (-not $version) { $version = "present" }
    Write-Log "metronous CLI already installed: $version"
} elseif (Test-Path $MetronousExe) {
    Write-Log "metronous found at $MetronousExe - adding to PATH for this session"
    $env:PATH = "$InstallDir;$env:PATH"
    $version = & $MetronousExe --version 2>$null
    if (-not $version) { $version = "present" }
    Write-Log "metronous version: $version"
} else {
    Write-Log "metronous not found - downloading from GitHub Releases"

    try {
        # Resolve latest version tag
        $release = Invoke-RestMethod -Uri 'https://api.github.com/repos/kiosvantra/metronous/releases/latest' -UseBasicParsing
        $VERSION = $release.tag_name
        Write-Log "Latest release: $VERSION"

        $archive = "metronous_$($VERSION.TrimStart('v'))_windows_amd64.zip"
        $baseUrl = "https://github.com/kiosvantra/metronous/releases/download/$VERSION"
        $tmpDir  = Join-Path $env:TEMP "metronous-install-$PID"
        New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null

        $archivePath   = Join-Path $tmpDir $archive
        $checksumPath  = Join-Path $tmpDir 'checksums.txt'

        Write-Log "Downloading $archive ..."
        Invoke-WebRequest -Uri "$baseUrl/$archive"   -OutFile $archivePath   -UseBasicParsing
        Invoke-WebRequest -Uri "$baseUrl/checksums.txt" -OutFile $checksumPath -UseBasicParsing

        # Verify checksum
        $expected = (Get-Content $checksumPath | Where-Object { $_ -match [regex]::Escape($archive) }) -replace '\s+.*',''
        if ($expected) {
            $actual = (Get-FileHash $archivePath -Algorithm SHA256).Hash.ToLower()
            if ($actual -ne $expected.ToLower()) {
                throw "Checksum mismatch for $archive (expected $expected, got $actual)"
            }
            Write-Log "Checksum OK"
        } else {
            Write-Warn "No checksum entry found for $archive - skipping verification"
        }

        # Extract and install
        $extractDir = Join-Path $tmpDir 'extracted'
        Expand-Archive -Path $archivePath -DestinationPath $extractDir -Force
        $exeFound = Get-ChildItem $extractDir -Recurse -Filter 'metronous.exe' | Select-Object -First 1
        if (-not $exeFound) { throw "metronous.exe not found in archive" }

        New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
        Move-Item $exeFound.FullName $MetronousExe -Force
        Write-Log "Installed to $MetronousExe"

        $env:PATH = "$InstallDir;$env:PATH"
        Remove-Item $tmpDir -Recurse -Force -ErrorAction SilentlyContinue

    } catch {
        Write-Warn "Automatic install failed: $_"
        Write-Warn "Download manually from: https://github.com/kiosvantra/metronous/releases"
        Write-Warn "Extract the Windows zip and run: metronous.exe install  (as Administrator)"
        @"
metronous: install_failed
auto_configured: no
note: $($_.Exception.Message)
"@ | Out-File -FilePath $StatusFile -Encoding UTF8
        exit 0
    }
}

# ---------------------------------------------------------------------------
# 2. Run metronous install (registers Windows service - requires Administrator)
# ---------------------------------------------------------------------------
$metronousCmd = if (Has-Cmd 'metronous') { 'metronous' } else { $MetronousExe }

Write-Log "Running metronous install to configure OpenCode and register service"
Write-Warn "This step requires an elevated (Administrator) terminal to register the Windows service."

try {
    & $metronousCmd install 2>&1 | ForEach-Object { Write-Log $_ }
    Write-Log "OpenCode configured with metronous"
    $configured = 1
} catch {
    Write-Warn "metronous install failed: $_"
    Write-Warn "Run manually as Administrator: metronous install"
    $configured = 0
}

$version = & $metronousCmd --version 2>$null
if (-not $version) { $version = "unknown" }

@"
metronous: installed
version: ${version}
auto_configured: yes
configured: ${configured}
"@ | Out-File -FilePath $StatusFile -Encoding UTF8

Write-Log "Done (configured: $configured)"
