param()

$ErrorActionPreference = "Stop"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent $scriptDir
$embedDir = "$repoRoot\cmd\ywai\embedded_data"

if (Test-Path $embedDir) {
    Remove-Item -Recurse -Force $embedDir
}

$skillsDir = "$embedDir\skills"
$ptDir = "$embedDir\project-types"
New-Item -ItemType Directory -Path $skillsDir -Force | Out-Null
New-Item -ItemType Directory -Path $ptDir -Force | Out-Null

Get-ChildItem -Force "$repoRoot\skills" | Copy-Item -Destination $skillsDir -Recurse -Force
Get-ChildItem -Force "$repoRoot\project-types" | Copy-Item -Destination $ptDir -Recurse -Force

$skillCount = (Get-ChildItem -Directory $skillsDir).Count
$ptCount = (Get-ChildItem -Directory $ptDir).Count
Write-Host "Prepared embedded data: $skillCount skills, $ptCount project types"
