# Setup AI Skills for Windows/PowerShell
# Mirrors setup.sh behavior for the most common automation paths.

[CmdletBinding()]
param(
    [switch]$All,
    [switch]$Claude,
    [switch]$Cursor,
    [switch]$Opencode,
    [switch]$Gemini,
    [switch]$Codex,
    [switch]$Copilot,
    [switch]$GlobalOnly,
    [string]$ProjectType = '',
    [switch]$Help
)

$ErrorActionPreference = "Stop"

if ($Help) {
    Write-Host "Usage: .\setup.ps1 [OPTIONS]"
    Write-Host ""
    Write-Host "Options:"
    Write-Host "  -All       Configure all AI assistants"
    Write-Host "  -Claude    Configure Claude Code"
    Write-Host "  -Cursor    Configure Cursor"
    Write-Host "  -Opencode  Configure OpenCode (same as Claude setup)"
    Write-Host "  -Gemini    Configure Gemini CLI"
    Write-Host "  -Codex     Configure Codex"
    Write-Host "  -Copilot   Configure GitHub Copilot"
    Write-Host "  -GlobalOnly       Configure only global user-profile agents (no repo files)"
    Write-Host "  -ProjectType T    Project type for global agent generation (nest, dotnet, generic, ...)"
    exit 0
}

function Invoke-GlobalAgentsGenerator {
    param([string]$Type, [string]$RepoRoot)

    $pt = $Type
    if (-not $pt) { $pt = 'generic' }

    # Prefer the Go binary if available: same logic as the wizard.
    $ywai = Get-Command ywai -ErrorAction SilentlyContinue
    if ($ywai) {
        Write-Host "[global-only] Delegating to ywai --update-global-agents --type=$pt" -ForegroundColor Cyan
        & ywai --update-global-agents --type=$pt --silent
        if ($LASTEXITCODE -eq 0) { return $true }
        Write-Warning "ywai delegation failed, falling back to install.ps1"
    }

    # Fallback: run the extension install script (which also contains a copy
    # fallback with user-file preservation).
    $ext = Join-Path $RepoRoot 'ywai\extensions\install-steps\global-agents\install.ps1'
    if (-not (Test-Path $ext)) {
        Write-Error "global-agents install.ps1 not found at: $ext"
        return $false
    }
    $env:YWAI_PROJECT_TYPE = $pt
    & $ext -TargetDir $RepoRoot
    return ($LASTEXITCODE -eq 0)
}

$ScriptDir = Split-Path -Parent $PSCommandPath

# Try to source shared UI (colors + Write-* functions)
$UiPath = Join-Path (Split-Path $ScriptDir -Parent) "auto\lib\ui.ps1"
if (Test-Path $UiPath) {
    . $UiPath
} else {
    # Fallback: define minimal colors when run standalone
    function Write-Ok([string]$msg) { Write-Host "[OK] $msg" -ForegroundColor Green }
    function Write-Info([string]$msg) { Write-Host "[INFO] $msg" -ForegroundColor Cyan }
    function Write-Warn([string]$msg) { Write-Host "[WARN] $msg" -ForegroundColor Yellow }
}

try {
    $RepoRoot = git rev-parse --show-toplevel 2>$null
    if ([string]::IsNullOrWhiteSpace($RepoRoot)) { throw "no git" }
} catch {
    if ($ScriptDir -match "skills$") {
        $RepoRoot = Split-Path $ScriptDir -Parent
    } else {
        $RepoRoot = (Get-Location).Path
    }
}
$SkillsSource = Join-Path $RepoRoot "skills"

if ($All) {
    # -All targets only the officially supported AI assistants:
    # OpenCode, Claude, GitHub Copilot. Cursor / Gemini / Codex remain
    # available via their own explicit switches but are never wired in by
    # -All or by the automation default below.
    $Claude = $true
    $Opencode = $true
    $Copilot = $true
}

if (-not ($Claude -or $Cursor -or $Opencode -or $Gemini -or $Codex -or $Copilot)) {
    # Non-interactive default for automation usage.
    $Claude = $true
    $Opencode = $true
    $Copilot = $true
}

function Set-SkillsLink {
    param([string]$TargetDir)

    if (-not (Test-Path $TargetDir)) {
        New-Item -ItemType Directory -Path $TargetDir -Force | Out-Null
    }

    $linkPath = Join-Path $TargetDir "skills"
    if (Test-Path $linkPath) {
        try {
            $existing = Get-Item -LiteralPath $linkPath -Force
            if ($existing.LinkType) {
                Remove-Item -LiteralPath $linkPath -Force
            } else {
                $backup = "$linkPath.backup.$([DateTimeOffset]::UtcNow.ToUnixTimeSeconds())"
                Move-Item -LiteralPath $linkPath -Destination $backup -Force
                Write-Info "Backed up existing skills dir to: $backup"
            }
        } catch {
            Remove-Item -LiteralPath $linkPath -Recurse -Force -ErrorAction SilentlyContinue
        }
    }

    try {
        New-Item -ItemType SymbolicLink -Path $linkPath -Target $SkillsSource -Force | Out-Null
    } catch {
        try {
            New-Item -ItemType Junction -Path $linkPath -Target $SkillsSource -Force | Out-Null
        } catch {
            Copy-Item -Path $SkillsSource -Destination $linkPath -Recurse -Force
            Write-Warn "Could not create link/junction, copied skills directory instead"
        }
    }
}

function Copy-AgentsToFile {
    param([string]$TargetName)
    $count = 0
    $agents = Get-ChildItem -Path $RepoRoot -Recurse -File -Force -ErrorAction SilentlyContinue |
        Where-Object {
            ($_.Name -ieq "AGENTS.md" -or $_.Name -ieq "AGENTS.MD") -and
            $_.FullName -notmatch '\\node_modules\\|\\\.git\\|\\bin\\|\\obj\\|\\\.next\\|\\dist\\'
        }
    foreach ($f in $agents) {
        $dest = Join-Path $f.DirectoryName $TargetName
        Copy-Item -Path $f.FullName -Destination $dest -Force
        $count++
    }
    Write-Ok "Copied $count instruction files -> $TargetName"
}

function Ensure-VSCodeSettings {
    $vscodeDir = Join-Path $RepoRoot ".vscode"
    $settingsPath = Join-Path $vscodeDir "settings.json"
    New-Item -ItemType Directory -Path $vscodeDir -Force | Out-Null
    if (-not (Test-Path $settingsPath)) {
        $settings = @'
{
  "chat.useAgentsMdFile": true,
  "github.copilot.chat.commitMessage.enabled": true
}
'@
        Set-Content -Path $settingsPath -Value $settings -Encoding UTF8
        Write-Ok "Created .vscode/settings.json"
    } else {
        Write-Info "settings.json already exists, skipping update"
    }
}

function Ensure-ClaudeMd {
    $dir = Join-Path $RepoRoot ".claude"
    $file = Join-Path $dir "Claude.md"
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
    if (-not (Test-Path $file)) {
        $content = @'
# Claude Code Instructions

- Follow AGENTS.md in this repository.
- Use linked skills from .claude/skills/.
'@
        Set-Content -Path $file -Value $content -Encoding UTF8
        Write-Ok "Created .claude/Claude.md"
    }
}

function Ensure-GeminiMd {
    $dir = Join-Path $RepoRoot ".gemini"
    $file = Join-Path $dir "gemini.md"
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
    if (-not (Test-Path $file)) {
        $content = @'
# Gemini CLI Instructions

- Follow AGENTS.md in this repository.
- Use linked skills from .gemini/skills/.
'@
        Set-Content -Path $file -Value $content -Encoding UTF8
        Write-Ok "Created .gemini/gemini.md"
    }
}

function Setup-Claude {
    $dir = Join-Path $RepoRoot ".claude"
    Set-SkillsLink -TargetDir $dir
    Write-Ok ".claude/skills -> skills/"
    Copy-AgentsToFile -TargetName "CLAUDE.md"
    if (Test-Path (Join-Path $RepoRoot ".gitignore")) {
        Copy-Item -Path (Join-Path $RepoRoot ".gitignore") -Destination (Join-Path $dir ".gitignore") -Force
    }
    Ensure-ClaudeMd
}

function Setup-Cursor {
    $dir = Join-Path $RepoRoot ".cursor"
    Set-SkillsLink -TargetDir $dir
    Write-Ok ".cursor/skills -> skills/"
    Copy-AgentsToFile -TargetName "CURSOR.md"
    $agentsMd = Join-Path $RepoRoot "AGENTS.MD"
    if (Test-Path $agentsMd) {
        Copy-Item -Path $agentsMd -Destination (Join-Path $RepoRoot ".cursorrules") -Force
    }
}

function Setup-OpenCode { Setup-Claude }

function Setup-Gemini {
    $dir = Join-Path $RepoRoot ".gemini"
    Set-SkillsLink -TargetDir $dir
    Write-Ok ".gemini/skills -> skills/"
    Copy-AgentsToFile -TargetName "GEMINI.md"
    if (Test-Path (Join-Path $RepoRoot ".gitignore")) {
        Copy-Item -Path (Join-Path $RepoRoot ".gitignore") -Destination (Join-Path $dir ".gitignore") -Force
    }
    Ensure-GeminiMd
}

function Setup-Codex {
    $dir = Join-Path $RepoRoot ".codex"
    Set-SkillsLink -TargetDir $dir
    Write-Ok ".codex/skills -> skills/"
}

function Setup-Copilot {
    $dir = Join-Path $RepoRoot ".github"
    Set-SkillsLink -TargetDir $dir
    Write-Ok ".github/skills -> skills/"
    Ensure-VSCodeSettings
}

if ($GlobalOnly) {
    Write-Info "Global-only mode: installing global user-profile agents (no repo writes)"
    $ok = Invoke-GlobalAgentsGenerator -Type $ProjectType -RepoRoot $RepoRoot
    if ($ok) {
        Write-Ok "Global agents configured"
        exit 0
    } else {
        Write-Host "[ERROR] Global agents configuration failed" -ForegroundColor Red
        exit 1
    }
}

Write-Info "Configuring AI assistants from skills/setup.ps1"
if ($Claude) { Setup-Claude }
if ($Cursor) { Setup-Cursor }
if ($Opencode) { Setup-OpenCode }
if ($Gemini) { Setup-Gemini }
if ($Codex) { Setup-Codex }
if ($Copilot) { Setup-Copilot }

Write-Ok "AI skills setup completed"
