$ErrorActionPreference = "Stop"

$RepoOwner = "AppajiDheeraj"
$RepoName = "GoFetch"
$BinaryName = "gofetch.exe"

function Write-Info($Message) {
  Write-Host $Message
}

function Fail($Message) {
  Write-Host "ERROR: $Message"
  exit 1
}

function Get-Arch {
  if ([Environment]::Is64BitOperatingSystem) {
    return "amd64"
  }
  return "386"
}

Write-Info "Installing GoFetch..."

$Arch = Get-Arch
$Version = $env:GOFETCH_VERSION
if ([string]::IsNullOrWhiteSpace($Version)) {
  $Version = "latest"
}

if ($Version -eq "latest") {
  $Url = "https://github.com/$RepoOwner/$RepoName/releases/latest/download/gofetch-windows-$Arch.exe"
} else {
  $Url = "https://github.com/$RepoOwner/$RepoName/releases/download/$Version/gofetch-windows-$Arch.exe"
}

$InstallDir = Join-Path $env:LOCALAPPDATA "gofetch\bin"
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

$TmpPath = Join-Path $env:TEMP "gofetch.exe"
Invoke-WebRequest -Uri $Url -OutFile $TmpPath
Move-Item -Force $TmpPath (Join-Path $InstallDir $BinaryName)

$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
  [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
  Write-Info "Added $InstallDir to user PATH. Restart your terminal to use gofetch."
}

Write-Info "GoFetch installed successfully. Run: gofetch --help"
