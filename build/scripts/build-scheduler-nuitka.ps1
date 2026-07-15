# Nuitka Scheduler Build Script
# Usage: .\build-scheduler-nuitka.ps1 -OptLevel <1|2|3|4> -Version <version>
#
# Optimization Levels:
#   1 - Basic: standalone compilation
#   2 - Accelerated: +numpy plugin +follow-imports
#   3 - Aggressive: +LTO +remove-output
#   4 - Extreme: +MinGW64 +parallel jobs +all optimizations

param(
    [ValidateRange(1, 4)]
    [int]$OptLevel = 1,
    [string]$Version = "0.5.7",
    [int]$Jobs = 4
)

$ErrorActionPreference = "Stop"

$SchedulerDir = Join-Path $PSScriptRoot "..\..\scheduler"
$SolverPath = Join-Path $SchedulerDir "solver.py"
$OutputDir = Join-Path $SchedulerDir "dist"
$OrtoolsLibs = Join-Path $SchedulerDir ".venv\Lib\site-packages\ortools\.libs"

# Verify source exists
if (-not (Test-Path $SolverPath)) {
    Write-Error "solver.py not found at: $SolverPath"
    exit 1
}

# Create output directory
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

# Base arguments
$NuitkaArgs = @(
    "--standalone"
    "--onefile"
    "--output-filename=scheduler.exe"
    "--output-dir=$OutputDir"
    "--include-package=ortools"
    "--include-package=flask"
    "--include-package=jinja2"
    "--include-package=werkzeug"
    "--include-data-dir=$OrtoolsLibs=ortools/.libs"
    "--assume-yes-for-downloads"
    "--windows-console-mode=disable"
    $SolverPath
)

# Optimization level specific arguments
switch ($OptLevel) {
    1 {
        Write-Host "=== Building v$Version.$OptLevel (Basic) ===" -ForegroundColor Cyan
        # Basic: no additional optimization
    }
    2 {
        Write-Host "=== Building v$Version.$OptLevel (Accelerated) ===" -ForegroundColor Green
        $NuitkaArgs += "--enable-plugin=numpy"
        $NuitkaArgs += "--follow-imports"
    }
    3 {
        Write-Host "=== Building v$Version.$OptLevel (Aggressive) ===" -ForegroundColor Yellow
        $NuitkaArgs += "--enable-plugin=numpy"
        $NuitkaArgs += "--follow-imports"
        $NuitkaArgs += "--lto=yes"
        $NuitkaArgs += "--remove-output"
    }
    4 {
        Write-Host "=== Building v$Version.$OptLevel (Extreme) ===" -ForegroundColor Red
        $NuitkaArgs += "--enable-plugin=numpy"
        $NuitkaArgs += "--follow-imports"
        $NuitkaArgs += "--lto=yes"
        $NuitkaArgs += "--remove-output"
        $NuitkaArgs += "--mingw64"
        $NuitkaArgs += "--jobs=$Jobs"
        $NuitkaArgs += "--assume-yes-for-downloads"
    }
}

Write-Host "Nuitka arguments:" -ForegroundColor Gray
$NuitkaArgs | ForEach-Object { Write-Host "  $_" -ForegroundColor Gray }

# Run Nuitka (use venv's nuitka.cmd directly)
$NuitkCmd = Join-Path $SchedulerDir ".venv\Scripts\nuitka.cmd"
if (-not (Test-Path $NuitkCmd)) {
    Write-Error "Nuitka not found at: $NuitkCmd"
    exit 1
}
$StartTime = Get-Date
& $NuitkCmd @NuitkaArgs
$EndTime = Get-Date
$Duration = ($EndTime - $StartTime).TotalSeconds

# Check result
$OutputExe = Join-Path $OutputDir "scheduler.exe"
if (Test-Path $OutputExe) {
    $FileSize = (Get-Item $OutputExe).Length / 1MB
    Write-Host "`n=== Build Successful ===" -ForegroundColor Green
    Write-Host "Output: $OutputExe"
    Write-Host "Size: $([math]::Round($FileSize, 2)) MB"
    Write-Host "Duration: $([math]::Round($Duration, 1)) seconds"
    Write-Host "Version: $Version.$OptLevel"
} else {
    Write-Error "Build failed: scheduler.exe not found"
    exit 1
}
