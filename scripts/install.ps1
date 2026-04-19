<#
.SYNOPSIS
Installs Themis on Windows.
iex "& { $(irm https://raw.githubusercontent.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/main/scripts/install.ps1) }"
#>

$ErrorActionPreference = "Stop"

$Repo = "syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey"
$BinName = "themis.exe"
$InstallDir = Join-Path $HOME ".themis\bin"

Write-Host "=== Installing Themis ===" -ForegroundColor Cyan

# 1. Detect Architecture
$Arch = $env:PROCESSOR_ARCHITECTURE
if ($Arch -eq "AMD64") {
    $TargetArch = "amd64"
} elseif ($Arch -eq "ARM64") {
    $TargetArch = "arm64"
} else {
    Write-Error "Unsupported architecture: $Arch"
    exit 1
}

Write-Host "Detected platform: windows-$TargetArch"

# 2. Setup Directories
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
}

# 3. Download the binary
$AssetName = "themis_windows_$TargetArch.zip"
$DownloadUrl = "https://github.com/$Repo/releases/latest/download/$AssetName"
$TmpDir = Join-Path $env:TEMP "themis_install"

if (Test-Path $TmpDir) { Remove-Item -Force -Recurse $TmpDir }
New-Item -ItemType Directory -Path $TmpDir | Out-Null
$ZipPath = Join-Path $TmpDir $AssetName

Write-Host "Downloading latest release..."
try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $ZipPath
    Expand-Archive -Path $ZipPath -DestinationPath $TmpDir -Force
    
    $ExtractedExe = Join-Path $TmpDir $BinName
    if (Test-Path $ExtractedExe) {
        Move-Item -Path $ExtractedExe -Destination (Join-Path $InstallDir $BinName) -Force
    }
} catch {
    Write-Host "Warning: Precompiled binary not found or download failed." -ForegroundColor Yellow
    Write-Host "Falling back to 'go install' if Go is installed..."
    
    if (Get-Command go -ErrorAction SilentlyContinue) {
        # Run go install
        Start-Process -FilePath "go" -ArgumentList "install github.com/$Repo@latest" -Wait -NoNewWindow
        
        $GoPath = (go env GOPATH).Trim()
        $BuiltExe = Join-Path $GoPath "bin\$BinName"
        if (Test-Path $BuiltExe) {
            Copy-Item -Path $BuiltExe -Destination (Join-Path $InstallDir $BinName) -Force
        }
    } else {
        Write-Error "Go is not installed. Cannot build from source."
        exit 1
    }
}

if (Test-Path $TmpDir) { Remove-Item -Force -Recurse $TmpDir }

# 4. Add to PATH (User Environment Variable)
function Add-ToPath {
    param([string]$PathToAdd)
    $UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    if ($UserPath -split ";" -notcontains $PathToAdd) {
        $NewPath = "$UserPath;$PathToAdd"
        [Environment]::SetEnvironmentVariable("PATH", $NewPath, "User")
        Write-Host "Added $PathToAdd to User PATH variable." -ForegroundColor Green
    }
}

Add-ToPath -PathToAdd $InstallDir

# 5. Success
Write-Host "`n✔ Themis was successfully installed to $InstallDir" -ForegroundColor Green
Write-Host "Please restart your PowerShell terminal for the PATH changes to take effect."
Write-Host "`nThen run: " -NoNewline
Write-Host "themis --help" -ForegroundColor Cyan
