param(
    [string]$ImageTag = "gitriot-builder:local"
)

$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

$repoRoot = Split-Path -Parent $PSScriptRoot
$modCacheVolume = "gitriot-go-mod"
$buildCacheVolume = "gitriot-go-build"

Write-Host "Building Docker image $ImageTag..."
docker build -f "$repoRoot/Dockerfile.build" -t $ImageTag "$repoRoot" | Out-Host
if ($LASTEXITCODE -ne 0) {
    throw "Docker image build failed with exit code $LASTEXITCODE"
}

Write-Host "Running tests with cached Go modules..."
docker run --rm `
  -v "${repoRoot}:/src" `
  -w /src `
  -v "${modCacheVolume}:/go/pkg/mod" `
  -v "${buildCacheVolume}:/root/.cache/go-build" `
  $ImageTag sh -lc "/usr/local/go/bin/go test ./..." | Out-Host
if ($LASTEXITCODE -ne 0) {
    throw "Docker test run failed with exit code $LASTEXITCODE"
}

Write-Host "Done: tests passed"
