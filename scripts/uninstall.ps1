<#
.SYNOPSIS
  Local Eml uninstaller for Windows.

.DESCRIPTION
  Unregisters the Windows Service via `local-eml uninstall`, removes the
  binary (unless -KeepBinary), and optionally deletes the user data
  directory (-Purge). Service unregister requires administrator privileges.

.PARAMETER InstallDir
  Install directory. Defaults to $env:LOCALAPPDATA\local-eml.

.PARAMETER KeepBinary
  Unregister the service but leave the binary in place.

.PARAMETER Purge
  Also delete $env:USERPROFILE\.local-eml\ (EMLs, database, logs).

.EXAMPLE
  irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.ps1 | iex

.EXAMPLE
  & ([scriptblock]::Create((irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.ps1))) -Purge
#>
[CmdletBinding()]
param(
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'local-eml'),
    [switch]$KeepBinary,
    [switch]$Purge
)

$ErrorActionPreference = 'Stop'

$bin = Join-Path $InstallDir 'local-eml.exe'
if (-not (Test-Path $bin)) {
    $cmd = Get-Command 'local-eml.exe' -ErrorAction SilentlyContinue
    if ($cmd) { $bin = $cmd.Source }
}

Write-Host 'Local Eml uninstaller'
Write-Host "  Binary:  $bin"
Write-Host "  Purge:   $([bool]$Purge)"
Write-Host ''

if (Test-Path $bin) {
    try {
        $hasInput = ([Console]::IsInputRedirected -eq $false)
        if ($hasInput) {
            & $bin uninstall
        } else {
            & $bin uninstall --yes
        }
    } catch {
        Write-Warning "Service unregister returned: $_  (continuing)"
    }
}

if (-not $KeepBinary -and (Test-Path $bin)) {
    Remove-Item -Force $bin
    Write-Host "Removed $bin"
}

if ($Purge) {
    $data = Join-Path $env:USERPROFILE '.local-eml'
    if (Test-Path $data) {
        Remove-Item -Recurse -Force $data
        Write-Host "Removed $data"
    }
}

Write-Host ''
Write-Host 'Done.'
