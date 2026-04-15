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

Write-Host "Building publish/gitriot.exe with cached Go modules..."
docker run --rm `
  -v "${repoRoot}:/src" `
  -w /src `
  -v "${modCacheVolume}:/go/pkg/mod" `
  -v "${buildCacheVolume}:/root/.cache/go-build" `
  $ImageTag | Out-Host
if ($LASTEXITCODE -ne 0) {
    throw "Docker run build failed with exit code $LASTEXITCODE"
}

Write-Host "Done: publish/gitriot.exe"
