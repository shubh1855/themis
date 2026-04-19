<#
.SYNOPSIS
Themis CLI Windows Installer for Private Repositories

.DESCRIPTION
Downloads the latest release of Themis directly from your private GitHub repository
and configures the system PATH automatically.

.EXAMPLE
powershell -Command "iwr https://public-host.com/install.ps1 -useb | iex"
#>

$ErrorActionPreference = "Stop"

Write-Host "🚀 Installing Themis CLI for Windows..." -ForegroundColor Cyan

# Check if Token was provided to the shell script args, env var, or fallback to manual prompt
$Token = $env:GITHUB_TOKEN
if (-not $Token) {
    if ($args.Count -gt 0) {
        $Token = $args[0]
    } else {
        $Token = Read-Host "Private repository detected! Enter your GitHub Personal Access Token"
    }
}

$Repo = "syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey"
$Headers = @{
    "Authorization" = "token $Token"
    "Accept" = "application/vnd.github.v3+json"
}

Write-Host "🔍 Finding latest Windows release..." -ForegroundColor Yellow
$ReleaseUrl = "https://api.github.com/repos/$Repo/releases/latest"
try {
    $Release = Invoke-RestMethod -Uri $ReleaseUrl -Headers $Headers
} catch {
    Write-Host "❌ Failed to fetch release. Incorrect token or lack of 'repo' scope." -ForegroundColor Red
    exit 1
}

# Auto-locate the windows zip artifact
$AssetUrl = $Release.assets | Where-Object { $_.name -like "*windows*.zip" -or $_.name -like "*win64*" } | Select-Object -ExpandProperty url

if (-not $AssetUrl) {
    Write-Host "❌ Could not find a suitable Windows release asset." -ForegroundColor Red
    exit 1
}

$ZipFile = "$env:TEMP\themis.zip"
$ExtractDir = "$env:TEMP\themis_extract"

Write-Host "⬇️ Downloading... ($($AssetUrl))" -ForegroundColor Cyan
$DownloadHeaders = @{
    "Authorization" = "token $Token"
    "Accept" = "application/octet-stream"
}
Invoke-WebRequest -Uri $AssetUrl -Headers $DownloadHeaders -OutFile $ZipFile

Write-Host "📦 Extracting..." -ForegroundColor Yellow
if (Test-Path $ExtractDir) { Remove-Item $ExtractDir -Recurse -Force }
New-Item -ItemType Directory -Path $ExtractDir | Out-Null
Expand-Archive -Path $ZipFile -DestinationPath $ExtractDir -Force

$InstallDir = "$env:LOCALAPPDATA\themis\bin"
if (-not (Test-Path $InstallDir)) { New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null }

$ExeFile = Get-ChildItem -Path $ExtractDir -Filter "themis.exe" -Recurse | Select-Object -First 1

if (-not $ExeFile) {
    Write-Host "❌ Could not locate themis.exe in the downloaded zip." -ForegroundColor Red
    exit 1
}

Move-Item -Path $ExeFile.FullName -Destination "$InstallDir\themis.exe" -Force

# Automatically configure user's Path environment variable securely
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    $NewPath = "$UserPath;$InstallDir"
    [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
    Write-Host "🔌 Added $InstallDir to your PATH variable!" -ForegroundColor Magenta
    Write-Host "⚠️  Please close and reopen your PowerShell window." -ForegroundColor Magenta
}

# Clean payload
Remove-Item $ExtractDir -Recurse -Force
Remove-Item $ZipFile -Force

Write-Host "✅ Themis CLI installed natively! Run 'themis' to launch your web interface." -ForegroundColor Green
