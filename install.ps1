# ---
# description: snap-css install script for Windows.
# usage: irm https://raw.githubusercontent.com/sidisinsane/snap-css/main/install.ps1 | iex
# exits:
#   0: success
#   1: fail
# ---

$ErrorActionPreference = "Stop"

# Define variables
$GithubUser  = "sidisinsane"
$GithubRepo  = "snap-css"
$BinaryName  = "snap-css"

# 1. Detect architecture
$Arch = if ([System.Environment]::Is64BitOperatingSystem) {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        "ARM64" { "arm64" }
        default { "x86_64" }
    }
} else {
    Write-Error "Error: Unsupported architecture: $env:PROCESSOR_ARCHITECTURE"
    exit 1
}

# 2. Construct filename and download URL
$Filename    = "${GithubRepo}_Windows_${Arch}.zip"
$DownloadUrl = "https://github.com/${GithubUser}/${GithubRepo}/releases/latest/download/${Filename}"

# 3. Create the installation directory
$InstallDir = "$env:USERPROFILE\.$BinaryName"
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

# 4. Download and extract
Write-Host "Downloading $BinaryName for Windows ($Arch)..."
$ZipPath = Join-Path $env:TEMP $Filename
Invoke-WebRequest -Uri $DownloadUrl -OutFile $ZipPath -UseBasicParsing
Expand-Archive -Path $ZipPath -DestinationPath $InstallDir -Force
Remove-Item $ZipPath

# 5. Add to PATH (user scope)
$UserPath = [System.Environment]::GetEnvironmentVariable("PATH", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Host "Adding $InstallDir to PATH..."
    [System.Environment]::SetEnvironmentVariable(
        "PATH",
        "$UserPath;$InstallDir",
        "User"
    )
    Write-Host "Done! Restart your terminal to use $BinaryName."
} else {
    Write-Host "$BinaryName is already in your PATH."
}

Write-Host "Installation complete!"
