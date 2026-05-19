<#
.SYNOPSIS
  Local Eml installer for Windows.

.DESCRIPTION
  Downloads the matching prebuilt binary from GitHub Releases, verifies its
  SHA-256 against SHA256SUMS, places it on the per-user PATH, and (unless
  -NoService is passed) runs `local-eml install` to register the Windows
  Service. Service registration requires elevated privileges.

.PARAMETER Version
  Specific release tag (e.g. v0.0.1). Defaults to "latest".

.PARAMETER InstallDir
  Install directory. Defaults to $env:LOCALAPPDATA\local-eml.

.PARAMETER NoService
  Skip the service registration step.

.EXAMPLE
  irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.ps1 | iex

.EXAMPLE
  & ([scriptblock]::Create((irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.ps1))) -Version v0.0.1
#>
[CmdletBinding()]
param(
    [string]$Version    = 'latest',
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'local-eml'),
    [switch]$NoService
)

$ErrorActionPreference = 'Stop'
$Repo = 'hwhang0917/local-eml'

# Detect architecture.
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64' -or $env:PROCESSOR_ARCHITEW6432 -eq 'ARM64') {
    'arm64'
} else {
    'amd64'
}
$binary = "local-eml-windows-$arch.exe"

if ($Version -eq 'latest') {
    $base = "https://github.com/$Repo/releases/latest/download"
} else {
    $base = "https://github.com/$Repo/releases/download/$Version"
}
$url     = "$base/$binary"
$sumsUrl = "$base/SHA256SUMS"

Write-Host 'Local Eml installer'
Write-Host "  Target:  windows/$arch"
Write-Host "  Version: $Version"
Write-Host "  URL:     $url"
Write-Host "  Dest:    $InstallDir\local-eml.exe"
Write-Host ''

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

$tmpRoot = [System.IO.Path]::GetTempPath()
$tmp     = New-Item -ItemType Directory -Path $tmpRoot -Name "local-eml-$([Guid]::NewGuid())" -Force

try {
    Write-Host 'Downloading binary...'
    Invoke-WebRequest -Uri $url     -OutFile (Join-Path $tmp 'local-eml.exe') -UseBasicParsing
    Invoke-WebRequest -Uri $sumsUrl -OutFile (Join-Path $tmp 'SHA256SUMS')    -UseBasicParsing

    Write-Host 'Verifying checksum...'
    $expectedLine = Get-Content (Join-Path $tmp 'SHA256SUMS') |
        Where-Object { ($_ -split '\s+')[1] -eq $binary }
    if (-not $expectedLine) {
        throw "No checksum entry for $binary in SHA256SUMS"
    }
    $expected = ($expectedLine -split '\s+')[0].ToLower()
    $actual   = (Get-FileHash (Join-Path $tmp 'local-eml.exe') -Algorithm SHA256).Hash.ToLower()
    if ($expected -ne $actual) {
        throw "Checksum mismatch!`n  expected: $expected`n  actual:   $actual"
    }

    $dest = Join-Path $InstallDir 'local-eml.exe'
    Copy-Item (Join-Path $tmp 'local-eml.exe') $dest -Force
    Write-Host "Installed to $dest"

    # Add to user PATH if missing.
    $userPath = [Environment]::GetEnvironmentVariable('PATH', 'User')
    if ([string]::IsNullOrEmpty($userPath) -or ($userPath -split ';') -notcontains $InstallDir) {
        $newPath = if ([string]::IsNullOrEmpty($userPath)) { $InstallDir } else { "$userPath;$InstallDir" }
        [Environment]::SetEnvironmentVariable('PATH', $newPath, 'User')
        Write-Host "Added $InstallDir to the user PATH (open a new shell to pick it up)."
    }

    if (-not $NoService) {
        Write-Host ''
        Write-Host 'Registering Windows Service (this step requires administrator privileges).'
        # Forward an interactive TTY when available, otherwise auto-accept.
        $hasInput = ([Console]::IsInputRedirected -eq $false)
        if ($hasInput) {
            & $dest install
        } else {
            & $dest install --yes
        }
    }
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
