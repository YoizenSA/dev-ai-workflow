param(
    [string]$Version = "latest",
    [string]$InstallDir = ""
)

$ErrorActionPreference = "Stop"

$Repo = "Yoizen/dev-ai-workflow"
$Binary = "ywai"
$DataDir = Join-Path $env:USERPROFILE ".ywai"

if (-not $InstallDir) {
    $InstallDir = Join-Path $env:USERPROFILE "bin"
}

if ($Version -eq "latest") {
    $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    $Version = $Release.tag_name
}

$VersionClean = $Version.TrimStart("v")
$FileName = "ywai_${VersionClean}_windows_amd64.zip"
$DownloadUrl = "https://github.com/$Repo/releases/download/$Version/$FileName"

$TempDir = Join-Path $env:TEMP "ywai-install"
$ZipPath = Join-Path $TempDir $FileName

Write-Host "Installing $Binary $Version..." -ForegroundColor Cyan

if (-not (Test-Path $TempDir)) {
    New-Item -ItemType Directory -Path $TempDir -Force | Out-Null
}

Write-Host "  Downloading $DownloadUrl..."
Invoke-WebRequest -Uri $DownloadUrl -OutFile $ZipPath -UseBasicParsing

Write-Host "  Extracting..."
Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force

Write-Host "  Cleaning old cached data..."
@("skills", "project-types") | ForEach-Object {
    $dir = Join-Path $DataDir $_
    if (Test-Path $dir) {
        Remove-Item -Path $dir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$SourceExe = Join-Path $TempDir "$Binary.exe"
if (-not (Test-Path $SourceExe)) {
    $SourceExe = Get-ChildItem -Path $TempDir -Filter "$Binary.exe" -Recurse | Select-Object -First 1
    if (-not $SourceExe) {
        Write-Error "Binary $Binary.exe not found in archive"
        exit 1
    }
    $SourceExe = $SourceExe.FullName
}

Copy-Item -Path $SourceExe -Destination (Join-Path $InstallDir "$Binary.exe") -Force

$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Host "  Adding $InstallDir to user PATH..."
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    $env:Path = "$env:Path;$InstallDir"
} else {
    Write-Host "  $InstallDir already in PATH."
}

Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue

Write-Host "  Seeding data..."
$ExePath = Join-Path $InstallDir "$Binary.exe"
& $ExePath version 2>&1 | Out-Null

$Installed = Get-Command $Binary -ErrorAction SilentlyContinue
if ($Installed) {
    Write-Host ""
    Write-Host "  $Binary $Version installed!" -ForegroundColor Green
    Write-Host "  Location: $($Installed.Source)" -ForegroundColor Gray
    Write-Host ""
    Write-Host "  Run: ywai install" -ForegroundColor Yellow
} else {
    Write-Host ""
    Write-Host "  $Binary installed to $InstallDir" -ForegroundColor Green
    Write-Host "  Restart your terminal or run: refreshenv" -ForegroundColor Yellow
}
