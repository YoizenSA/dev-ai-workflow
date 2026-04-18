#requires -version 5.1
# SDD Engram Plugin Setup — Windows
# Registers opencode-sdd-engram-manage plugin in ~/.config/opencode/tui.json
param(
    [string]$TargetDir = "."
)

$ErrorActionPreference = 'Stop'

function Write-Log($msg)  { Write-Host "[sdd-engram-plugin] $msg" }
function Write-Warn($msg) { Write-Host "[sdd-engram-plugin] WARN: $msg" -ForegroundColor Yellow }

$TuiJson = Join-Path $HOME '.config' 'opencode' 'tui.json'
$PluginEntry = 'opencode-sdd-engram-manage'
$ProfilesDir = Join-Path $HOME '.config' 'opencode' 'profiles'
$ScriptDir = $PSScriptRoot
$ExampleProfilesDir = Join-Path $ScriptDir 'profiles'

# ---------------------------------------------------------------------------
# Copy example profiles if they don't exist
# ---------------------------------------------------------------------------
if (Test-Path $ExampleProfilesDir) {
  Write-Log "Copying example profiles to $ProfilesDir"
  if (-not (Test-Path $ProfilesDir)) {
    New-Item -ItemType Directory -Path $ProfilesDir -Force | Out-Null
  }
  
  $profileFiles = Get-ChildItem -Path $ExampleProfilesDir -Filter "*.json"
  foreach ($profileFile in $profileFiles) {
    $target = Join-Path $ProfilesDir $profileFile.Name
    
    if (-not (Test-Path $target)) {
      Copy-Item -Path $profileFile.FullName -Destination $target
      Write-Log "Created example profile: $($profileFile.Name)"
    } else {
      Write-Log "Profile already exists, skipping: $($profileFile.Name)"
    }
  }
}

# ---------------------------------------------------------------------------
# Ensure tui.json exists
# ---------------------------------------------------------------------------
if (-not (Test-Path $TuiJson)) {
  Write-Log "Creating $TuiJson with plugin entry"
  $TuiDir = Split-Path $TuiJson -Parent
  if (-not (Test-Path $TuiDir)) {
    New-Item -ItemType Directory -Path $TuiDir -Force | Out-Null
  }
  @{
    '$schema' = 'https://opencode.ai/tui.json'
    plugin = @($PluginEntry)
  } | ConvertTo-Json -Depth 20 | Set-Content -Path $TuiJson -Encoding UTF8
  Write-Log "Created $TuiJson with $PluginEntry"
  exit 0
}

# ---------------------------------------------------------------------------
# Add plugin to tui.json
# ---------------------------------------------------------------------------
try {
  $raw = Get-Content $TuiJson -Raw
  $cfg = $raw | ConvertFrom-Json

  if (-not $cfg.plugin) {
    $cfg | Add-Member -NotePropertyName plugin -NotePropertyValue @() -Force
  }

  $plugins = @($cfg.plugin)
  if ($plugins -contains $PluginEntry) {
    Write-Log "Plugin already present in $TuiJson"
  } else {
    $plugins += $PluginEntry
    $cfg.plugin = $plugins
    ($cfg | ConvertTo-Json -Depth 20) | Set-Content -Path $TuiJson -Encoding UTF8
    Write-Log "Added $PluginEntry to plugin[] in $TuiJson"
  }
} catch {
  Write-Warn "Could not edit $TuiJson: $($_.Exception.Message)"
  Write-Warn "Manually add '$PluginEntry' to the plugin[] array in $TuiJson"
}

Write-Log "Done"
