#requires -version 5.1
# Global Agents Extension - Windows
#
# Preferred path: delegate to the `ywai` binary which runs the in-process
# globalagents generator (same code used by the wizard). The fallback copies
# templates directly but preserves user-owned files (any .md not matching a
# template basename stays untouched).

param(
    [string]$TargetDir = "."
)

$ErrorActionPreference = 'Stop'

$projectType = $env:YWAI_PROJECT_TYPE
if (-not $projectType) { $projectType = 'generic' }

$extDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$agentsSource = Join-Path $extDir 'templates'
$versionFile = Join-Path $agentsSource 'VERSION'
$stateDir = Join-Path $env:USERPROFILE '.ywai'
$stateVersionFile = Join-Path $stateDir 'global-agents-version'

Write-Host "Configuring global agents for project type: $projectType" -ForegroundColor Cyan

# --- Try the Go binary first --------------------------------------------------
$ywai = Get-Command ywai -ErrorAction SilentlyContinue
if ($ywai) {
    Write-Host "Delegating to: ywai --update-global-agents --type=$projectType --silent" -ForegroundColor Cyan
    & ywai --update-global-agents --type=$projectType --silent
    if ($LASTEXITCODE -eq 0) {
        if (Test-Path $versionFile) {
            New-Item -ItemType Directory -Force -Path $stateDir | Out-Null
            $ver = (Get-Content $versionFile -Raw).Trim()
            Set-Content -Path $stateVersionFile -Value $ver -NoNewline
        }
        exit 0
    }
    Write-Host "ywai delegation failed, falling back to PowerShell implementation" -ForegroundColor Yellow
}

# --- Fallback: copy templates, preserving user-owned files --------------------
if (-not (Test-Path $agentsSource)) {
    Write-Host "Agent templates not found: $agentsSource" -ForegroundColor Red
    exit 1
}

$localVersion = $null
if (Test-Path $versionFile) {
    $localVersion = (Get-Content $versionFile -Raw).Trim()
}

$installedVersion = $null
if (Test-Path $stateVersionFile) {
    $installedVersion = (Get-Content $stateVersionFile -Raw).Trim()
}

if ($localVersion -and $localVersion -eq $installedVersion) {
    Write-Host "Global agents already up to date (version $installedVersion)" -ForegroundColor Green
    Write-Host "To force reinstall, remove $stateVersionFile"
    exit 0
}

$homeDir = $env:USERPROFILE

$agentLocations = @{
    "OpenCode" = Join-Path $homeDir ".config\opencode\agent"
    "Copilot"  = Join-Path $homeDir ".copilot\agents"
    "Claude"   = Join-Path $homeDir ".claude\agents"
    "Agents"   = Join-Path $homeDir ".agents\agents"
}
# Gemini and Cursor are intentionally excluded from the managed agent set;
# the current policy is to only support OpenCode, Claude, and Copilot
# globally. User-owned files under ~/.gemini or ~/.cursor are not touched.

# Managed basenames: only these are removed/overwritten. Any other .md file in
# the destination is preserved across runs.
$managedFiles = Get-ChildItem -Path $agentsSource -Filter '*.md' | Select-Object -ExpandProperty Name

$copiedTotal = 0

foreach ($platformName in $agentLocations.Keys) {
    $destDir = $agentLocations[$platformName]
    New-Item -ItemType Directory -Force -Path $destDir | Out-Null

    foreach ($managed in $managedFiles) {
        $managedPath = Join-Path $destDir $managed
        if (Test-Path $managedPath) {
            Remove-Item -Force -ErrorAction SilentlyContinue $managedPath
        }
    }

    Get-ChildItem -Path $agentsSource -Filter '*.md' | ForEach-Object {
        $dest = Join-Path $destDir $_.Name
        Copy-Item -Force $_.FullName $dest
        $copiedTotal++
        Write-Host "  [$platformName] Installed agent: $($_.Name)" -ForegroundColor Green
    }
}

Write-Host ""
Write-Host "Global agents configured ($copiedTotal templates copied)" -ForegroundColor Green
Write-Host ""
Write-Host "Locations:" -ForegroundColor White
foreach ($platformName in $agentLocations.Keys) {
    Write-Host "  $platformName : $($agentLocations[$platformName])" -ForegroundColor Gray
}

if ($localVersion) {
    New-Item -ItemType Directory -Force -Path $stateDir | Out-Null
    Set-Content -Path $stateVersionFile -Value $localVersion -NoNewline
    Write-Host ""
    Write-Host "Installed global agents version: $localVersion" -ForegroundColor Cyan
}
