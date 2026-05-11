param()

$ErrorActionPreference = "Stop"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent $scriptDir
$embedDir = "$repoRoot\cmd\ywai\embedded_data"

if (Test-Path $embedDir) {
    Remove-Item -Recurse -Force $embedDir
}

$skillsDir = "$embedDir\skills"
New-Item -ItemType Directory -Path $skillsDir -Force | Out-Null

Get-ChildItem -Force "$repoRoot\skills" | Copy-Item -Destination $skillsDir -Recurse -Force

$skillCount = (Get-ChildItem -Directory $skillsDir).Count
Write-Host "Prepared embedded data: $skillCount skills"
