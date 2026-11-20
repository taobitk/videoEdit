<#
.SYNOPSIS
Builds the unified VelocityEngine v2 binary (frontend static SPA embedded into Go velocity.exe).
#>
[CmdletBinding()]
param(
    [switch]$Test,
    [switch]$E2E
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$RootDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$FrontendDir = Join-Path $RootDir 'frontend'
$BackendDir = Join-Path $RootDir 'backend'
$FrontendBuildDir = Join-Path $FrontendDir 'build'
$DistDir = Join-Path $BackendDir 'cmd\velocity\dist'
$OutputExe = Join-Path $RootDir 'velocity.exe'

$PreferredGo = 'D:\code\environment\GoSDK\GoSDK-1.26.2\bin\go.exe'
$GoExe = if (Test-Path $PreferredGo) { $PreferredGo } else { 'go' }

$sw = [System.Diagnostics.Stopwatch]::StartNew()

$MingwGxx = 'D:\code\environment\mingw64\bin\g++.exe'
$CudaMain = Join-Path $RootDir 'cuda_render\main.cpp'
$CudaExe  = Join-Path $RootDir 'velocity-cuda.exe'
if ((Test-Path $MingwGxx) -and (Test-Path $CudaMain)) {
    Write-Host '==> Compiling CUDA NVRTC Zero-Copy engine (velocity-cuda.exe)...' -ForegroundColor Cyan
    & $MingwGxx -O3 -std=c++17 -static -I"D:\code\environment\ffmpeg-shared-9.0.2\include" -I"D:\code\environment\cuda-12.8\include" -L"D:\code\environment\ffmpeg-shared-9.0.2\lib" $CudaMain -o $CudaExe -lavformat -lavcodec -lavfilter -lavutil
    if ($LASTEXITCODE -ne 0) { throw "velocity-cuda.exe build failed with exit code $LASTEXITCODE" }
}

if ($Test) {
    Write-Host '==> Running backend Go tests...' -ForegroundColor Cyan
    Push-Location $BackendDir
    try {
        $env:CGO_ENABLED = '0'
        & $GoExe test -count=1 -v ./...
        if ($LASTEXITCODE -ne 0) { throw "Go tests failed with exit code $LASTEXITCODE" }
    } finally {
        Pop-Location
    }

    Write-Host '==> Running frontend Vitest & svelte-check...' -ForegroundColor Cyan
    Push-Location $FrontendDir
    try {
        npm run test:unit
        if ($LASTEXITCODE -ne 0) { throw "Vitest failed with exit code $LASTEXITCODE" }
        npm run check
        if ($LASTEXITCODE -ne 0) { throw "svelte-check failed with exit code $LASTEXITCODE" }
    } finally {
        Pop-Location
    }
}

Write-Host '==> Building SvelteKit static bundle...' -ForegroundColor Cyan
Push-Location $FrontendDir
try {
    npm run build
    if ($LASTEXITCODE -ne 0) { throw "Frontend build failed with exit code $LASTEXITCODE" }
} finally {
    Pop-Location
}

if (-not (Test-Path (Join-Path $FrontendBuildDir 'index.html'))) {
    throw "Frontend build output missing index.html at $FrontendBuildDir"
}

Write-Host '==> Syncing frontend/build -> backend/cmd/velocity/dist...' -ForegroundColor Cyan
if (Test-Path $DistDir) {
    Remove-Item -Path $DistDir -Recurse -Force
}
New-Item -ItemType Directory -Path $DistDir -Force | Out-Null
Copy-Item -Path (Join-Path $FrontendBuildDir '*') -Destination $DistDir -Recurse -Force

Write-Host '==> Compiling single-binary velocity.exe (CGO_ENABLED=0)...' -ForegroundColor Cyan
Push-Location $BackendDir
try {
    $env:CGO_ENABLED = '0'
    & $GoExe build -ldflags='-s -w -H windowsgui' -o $OutputExe ./cmd/velocity
    if ($LASTEXITCODE -ne 0) { throw "Go build failed with exit code $LASTEXITCODE" }
} finally {
    Pop-Location
}

if ($E2E) {
    Write-Host '==> Running Playwright Full Browser E2E tests...' -ForegroundColor Cyan
    Push-Location $FrontendDir
    try {
        npx playwright test
        if ($LASTEXITCODE -ne 0) { throw "Playwright E2E tests failed with exit code $LASTEXITCODE" }
    } finally {
        Pop-Location
    }
}

$sw.Stop()
$exeInfo = Get-Item $OutputExe
$sizeMb = [Math]::Round($exeInfo.Length / 1MB, 2)
Write-Host "==> Build complete in $([Math]::Round($sw.Elapsed.TotalSeconds, 2))s: $($exeInfo.FullName) ($sizeMb MB)" -ForegroundColor Green
