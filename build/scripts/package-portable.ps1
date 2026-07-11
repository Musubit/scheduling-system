param(
    [string]$BinDir = "bin",
    [string]$AppName = "scheduling-system",
    [string]$Version = "0.3.2"
)

$ErrorActionPreference = "Stop"

$mainExe = Join-Path $BinDir "$AppName.exe"
$schedulerSrc = Join-Path (Join-Path "scheduler" "dist") "scheduler.exe"
$schedulerDstDir = Join-Path $BinDir "scheduler"
$schedulerDst = Join-Path $schedulerDstDir "scheduler.exe"

# Verify the main exe exists
if (-not (Test-Path $mainExe)) {
    Write-Error "Main executable not found: $mainExe"
    Write-Error "Run 'wails3 task windows:build' first."
    exit 1
}

# Bundle scheduler.exe into a subfolder (clean layout: only 1 exe at root)
$hasScheduler = $false
if (Test-Path $schedulerSrc) {
    New-Item -ItemType Directory -Force -Path $schedulerDstDir | Out-Null
    Copy-Item $schedulerSrc $schedulerDst -Force
    $hasScheduler = $true
    Write-Host "Scheduler bundled: scheduler\scheduler.exe"
} else {
    Write-Host "Scheduler not found at $schedulerSrc — skipping (SA-only mode)"
}

# Create ZIP
$zipName = "$AppName-portable-v$Version.zip"
$zipPath = Join-Path $BinDir $zipName

$files = @($mainExe)
if ($hasScheduler) { $files += $schedulerDstDir }

Compress-Archive -Path $files -DestinationPath $zipPath -Force

Write-Host "======================================"
Write-Host "Portable ZIP created: $zipPath"
Write-Host "Contents:"
Write-Host "  - $AppName.exe"
if ($hasScheduler) {
    Write-Host "  - scheduler\scheduler.exe  (OR-Tools solver)"
}
Write-Host ""
Write-Host "To use: Extract the ZIP anywhere and double-click $AppName.exe"
Write-Host "======================================"
