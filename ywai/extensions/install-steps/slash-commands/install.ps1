#requires -version 5.1
# Slash Commands Extension — Windows
param(
    [string]$TargetDir = "."
)

$ErrorActionPreference = 'Stop'

$extDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$sourceDir = $extDir
$targetPromptsDir = Join-Path $TargetDir '.github\prompts'
$legacyPromptsDir = Join-Path $TargetDir 'prompts'

$xdgConfig = $env:XDG_CONFIG_HOME
if (-not $xdgConfig) { $xdgConfig = Join-Path $env:LOCALAPPDATA '' }
$targetOpenCodeSkillsDir = Join-Path $xdgConfig 'opencode\skills'
$targetOpenCodeCommandsDir = Join-Path $xdgConfig 'opencode\commands'
$targetCopilotAgentsDir = Join-Path $env:USERPROFILE '.copilot\agents'

New-Item -ItemType Directory -Force -Path $targetPromptsDir | Out-Null
New-Item -ItemType Directory -Force -Path $targetOpenCodeSkillsDir | Out-Null
New-Item -ItemType Directory -Force -Path $targetOpenCodeCommandsDir | Out-Null
New-Item -ItemType Directory -Force -Path $targetCopilotAgentsDir | Out-Null

# Migrate legacy prompt location
if (Test-Path $legacyPromptsDir) {
    Get-ChildItem -Path $legacyPromptsDir -Filter 'sdd-*.md' | ForEach-Object {
        $dest = Join-Path $targetPromptsDir $_.Name
        if (-not (Test-Path $dest)) {
            Move-Item -Force $_.FullName $dest
        }
    }
}

$copied = 0
Get-ChildItem -Path $sourceDir -Filter '*.md' | ForEach-Object {
    $name = $_.BaseName

    # Copy to GitHub Copilot prompts (project-local)
    $copilotDest = Join-Path $targetPromptsDir "$name.md"
    if (-not (Test-Path $copilotDest)) {
        Copy-Item -Force $_.FullName $copilotDest
        $copied++
    }

    # Copy to OpenCode skills directory structure (global)
    $skillDir = Join-Path $targetOpenCodeSkillsDir $name
    New-Item -ItemType Directory -Force -Path $skillDir | Out-Null
    $skillDest = Join-Path $skillDir 'SKILL.md'
    if (-not (Test-Path $skillDest)) {
        Copy-Item -Force $_.FullName $skillDest
        $copied++
    }

    # Copy to OpenCode commands (global) — enables slash invocation like /sdd-init
    $opencodeCmdDest = Join-Path $targetOpenCodeCommandsDir "$name.md"
    if (-not (Test-Path $opencodeCmdDest)) {
        Copy-Item -Force $_.FullName $opencodeCmdDest
        $copied++
    }

    # Copy to Copilot agents (global)
    $copilotAgentDest = Join-Path $targetCopilotAgentsDir "$name.md"
    if (-not (Test-Path $copilotAgentDest)) {
        Copy-Item -Force $_.FullName $copilotAgentDest
        $copied++
    }
}

Write-Host "Installed slash commands to .github\prompts, OpenCode skills, OpenCode commands, and Copilot agents"
