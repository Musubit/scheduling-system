param(
    [Parameter(Mandatory=$true)][string]$ExePath
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $ExePath)) {
    Write-Error "Subsystem check: file not found: $ExePath"
    exit 1
}

$bytes  = [IO.File]::ReadAllBytes($ExePath)
$peOff  = [BitConverter]::ToInt32($bytes, 60)
$sub    = [BitConverter]::ToUInt16($bytes, $peOff + 92)

# IMAGE_SUBSYSTEM_WINDOWS_GUI = 2
if ($sub -ne 2) {
    Write-Error ("Subsystem check FAILED for {0}: expected 2 (WINDOWS_GUI), got {1}. Rebuild with production BUILD_FLAGS (-H windowsgui)." -f $ExePath, $sub)
    exit 1
}

Write-Host ("Subsystem check OK: {0} is WINDOWS_GUI" -f $ExePath)
