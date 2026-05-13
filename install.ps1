# VibeCoding Installer for Windows
# Downloads and installs the latest release from GitHub

$ErrorActionPreference = "Stop"

$REPO = "fuckvibecoding/vibecoding"
$BINARY_NAME = "vibecoding.exe"
$DEFAULT_INSTALL_DIR = "$env:LOCALAPPDATA\vibecoding"

# Colors
function Write-Info { Write-Host "[INFO] $args" -ForegroundColor Cyan }
function Write-Success { Write-Host "[SUCCESS] $args" -ForegroundColor Green }
function Write-Warn { Write-Host "[WARN] $args" -ForegroundColor Yellow }
function Write-Error { Write-Host "[ERROR] $args" -ForegroundColor Red; exit 1 }

# Banner
Write-Host ""
Write-Host "╔═══════════════════════════════════════════════════════════════╗" -ForegroundColor DarkCyan
Write-Host "║                   VibeCoding Installer                       ║" -ForegroundColor DarkCyan
Write-Host "╚═══════════════════════════════════════════════════════════════╝" -ForegroundColor DarkCyan
Write-Host ""

# Detect architecture
$arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { Write-Error "32-bit systems are not supported" }
Write-Info "Detected architecture: windows/$arch"

# Get install directory
$installDir = if ($env:VIBECODING_INSTALL_DIR) { $env:VIBECODING_INSTALL_DIR } else { $DEFAULT_INSTALL_DIR }
Write-Info "Install directory: $installDir"

# Get latest version from GitHub
Write-Info "Fetching latest version..."
try {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$REPO/releases/latest" -Headers @{
        "Accept" = "application/vnd.github.v3+json"
    }
    $version = $release.tag_name
    Write-Info "Latest version: $version"
} catch {
    Write-Error "Failed to fetch latest version: $_"
}

# Find download URL
$archiveName = "vibecoding-windows-$arch.zip"
$asset = $release.assets | Where-Object { $_.name -eq $archiveName }

if (-not $asset) {
    Write-Error "Release asset not found: $archiveName"
}

$downloadUrl = $asset.browser_download_url
Write-Info "Download URL: $downloadUrl"

# Create temp directory
$tempDir = Join-Path $env:TEMP "vibecoding-install-$(Get-Random)"
New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

try {
    # Download archive
    $archivePath = Join-Path $tempDir $archiveName
    Write-Info "Downloading $archiveName..."
    
    $progressPreference = 'SilentlyContinue'
    Invoke-WebRequest -Uri $downloadUrl -OutFile $archivePath -UseBasicParsing
    $progressPreference = 'Continue'
    
    Write-Success "Download complete"

    # Extract archive
    Write-Info "Extracting archive..."
    $extractPath = Join-Path $tempDir "extract"
    Expand-Archive -Path $archivePath -DestinationPath $extractPath -Force
    
    # Find binary
    $binaryPath = Get-ChildItem -Path $extractPath -Filter $BINARY_NAME -Recurse | Select-Object -First 1
    
    if (-not $binaryPath) {
        Write-Error "Binary not found in archive"
    }

    # Create install directory
    if (-not (Test-Path $installDir)) {
        Write-Info "Creating install directory: $installDir"
        New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    }

    # Install binary
    $destPath = Join-Path $installDir $BINARY_NAME
    Write-Info "Installing to $destPath..."
    Copy-Item -Path $binaryPath.FullName -Destination $destPath -Force
    Write-Success "Installed $BINARY_NAME to $installDir"

    # Add to PATH if not already present
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    
    if ($currentPath -notlike "*$installDir*") {
        Write-Info "Adding $installDir to PATH..."
        [Environment]::SetEnvironmentVariable("Path", "$currentPath;$installDir", "User")
        $env:Path = "$env:Path;$installDir"
        Write-Success "Added to PATH (restart terminal to take effect)"
    } else {
        Write-Info "$installDir is already in PATH"
    }

    # Verify installation
    Write-Host ""
    Write-Success "Installation complete!"
    Write-Host ""
    Write-Host "  Version: $version" -ForegroundColor White
    Write-Host ""
    Write-Host "  Get started:" -ForegroundColor White
    Write-Host "    vibecoding --help" -ForegroundColor Gray
    Write-Host ""
    Write-Host "  Note: Restart your terminal to use vibecoding" -ForegroundColor Yellow
    Write-Host ""

} catch {
    Write-Error "Installation failed: $_"
} finally {
    # Cleanup
    if (Test-Path $tempDir) {
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}
