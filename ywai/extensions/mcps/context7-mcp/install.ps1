#requires -version 5.1
# Context7 MCP Extension — Windows
param(
    [string]$TargetDir = "."
)

$ErrorActionPreference = 'Continue'

$providers = $env:YWAI_PROVIDERS
if (-not $providers) { $providers = 'opencode,claude' }

Write-Host "Installing Context7 MCP for providers: $providers"

$providerArray = $providers -split ','
$successCount = 0

foreach ($provider in $providerArray) {
    $provider = $provider.Trim()
    Write-Host "Installing Context7 for $provider..."
    try {
        $result = npx ctx7 setup --$provider 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-Host "Context7 installed successfully for $provider" -ForegroundColor Green
            $successCount++
        } else {
            Write-Host "Could not install Context7 for $provider (provider may not be installed)" -ForegroundColor Yellow
        }
    } catch {
        Write-Host "Could not install Context7 for $provider" -ForegroundColor Yellow
    }
}

Write-Host ""
Write-Host "Context7 MCP installation complete!"
Write-Host "Successful: $successCount/$($providerArray.Count) providers"

# Create example file for backward compatibility
$targetMcpDir = Join-Path $TargetDir '.ywai\mcp'
$targetFile = Join-Path $targetMcpDir 'context7-mcp.example.json'
New-Item -ItemType Directory -Force -Path $targetMcpDir | Out-Null

@'
{
  "context7": {
    "type": "remote",
    "url": "https://mcp.context7.com/mcp",
    "enabled": true
  }
}
'@ | Set-Content $targetFile

Write-Host ""
Write-Host "Created example Context7 MCP config at $targetFile"
Write-Host "Note: Real MCP servers have been configured globally using npx ctx7 setup"
