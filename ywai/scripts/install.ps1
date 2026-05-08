param(
    [string]$Version = "latest",
    [string]$InstallDir = ""
)

$ErrorActionPreference = "Stop"

$Repo = "YoizenSA/dev-ai-workflow"
$Binary = "ywai"
$DataDir = Join-Path $env:USERPROFILE ".ywai"

function Trim-TrailingSlash {
    param([string]$Value)
    if (-not $Value) {
        return ""
    }
    return $Value.Trim().TrimEnd([char[]]@('\', '/'))
}

function Set-PathFirst {
    param([string]$Directory)

    $separator = [IO.Path]::PathSeparator
    $normalizedDirectory = Trim-TrailingSlash $Directory

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $userParts = @($userPath -split [regex]::Escape($separator) | Where-Object {
        $_ -and ((Trim-TrailingSlash $_) -ine $normalizedDirectory)
    })
    [Environment]::SetEnvironmentVariable("Path", (($Directory) + $separator + ($userParts -join $separator)).TrimEnd($separator), "User")

    $processParts = @($env:Path -split [regex]::Escape($separator) | Where-Object {
        $_ -and ((Trim-TrailingSlash $_) -ine $normalizedDirectory)
    })
    $env:Path = (($Directory) + $separator + ($processParts -join $separator)).TrimEnd($separator)
}

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

Write-Host "  Cleaning old cached data..."
@("skills", "project-types") | ForEach-Object {
    $dir = Join-Path $DataDir $_
    if (Test-Path $dir) {
        Remove-Item -Path $dir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

Write-Host "  Ensuring $InstallDir is first in user PATH..."
Set-PathFirst $InstallDir

Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue

Write-Host "  Seeding data..."
$ExePath = Join-Path $InstallDir "$Binary.exe"
$prevErrorAction = $ErrorActionPreference
$ErrorActionPreference = "Continue"
try {
    & $ExePath skills *> $null
    $seedExit = $LASTEXITCODE
} finally {
    $ErrorActionPreference = $prevErrorAction
}
if ($seedExit -ne 0) {
    Write-Warning "Data seed check failed. Try running: $ExePath skills"
}

$Installed = Get-Command $Binary -ErrorAction SilentlyContinue
if ($Installed) {
    Write-Host ""
    Write-Host "  $Binary $Version installed!" -ForegroundColor Green
    Write-Host "  Location: $($Installed.Source)" -ForegroundColor Gray
    $RunCommand = "ywai install"
    if ((Trim-TrailingSlash $Installed.Source) -ine (Trim-TrailingSlash $ExePath)) {
        Write-Host ""
        Write-Host "  Warning: PowerShell currently resolves '$Binary' to:" -ForegroundColor Yellow
        Write-Host "    $($Installed.Source)" -ForegroundColor Yellow
        Write-Host "  Start a new terminal or move $InstallDir earlier in PATH." -ForegroundColor Yellow
        $RunCommand = "& `"$ExePath`" install"
    }
    Write-Host ""
    Write-Host "  Run: $RunCommand" -ForegroundColor Yellow
} else {
    Write-Host ""
    Write-Host "  $Binary installed to $InstallDir" -ForegroundColor Green
    Write-Host "  Restart your terminal or run: refreshenv" -ForegroundColor Yellow
}
