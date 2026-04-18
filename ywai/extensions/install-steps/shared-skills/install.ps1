#requires -version 5.1
# Shared Skills Extension — Windows
param(
    [string]$TargetDir = "."
)

$ErrorActionPreference = 'Stop'

$extDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$sourceDir = Join-Path $extDir '..\..\..\skills\_shared'
$targetSharedDir = Join-Path $TargetDir 'skills\_shared'

if (-not (Test-Path $sourceDir)) {
    Write-Host "Shared skills source not found: $sourceDir (skipping)"
    exit 0
}

New-Item -ItemType Directory -Force -Path $targetSharedDir | Out-Null

$copied = 0
Get-ChildItem -Path $sourceDir | ForEach-Object {
    $dest = Join-Path $targetSharedDir $_.Name
    if (-not (Test-Path $dest)) {
        if ($_.PSIsContainer) {
            Copy-Item -Recurse -Force $_.FullName $dest
        } else {
            Copy-Item -Force $_.FullName $dest
        }
        $copied++
    }
}

Write-Host "Installed $copied shared skill asset(s) into skills\_shared\"
