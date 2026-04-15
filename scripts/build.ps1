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
$tmpExe = "publish/gitriot.tmp.exe"
docker run --rm `
  -v "${repoRoot}:/src" `
  -w /src `
  -v "${modCacheVolume}:/go/pkg/mod" `
  -v "${buildCacheVolume}:/root/.cache/go-build" `
  $ImageTag sh -lc "mkdir -p publish && /usr/local/go/bin/go mod download && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 /usr/local/go/bin/go build -o $tmpExe ./cmd/gitriot" | Out-Host
if ($LASTEXITCODE -ne 0) {
    throw "Docker run build failed with exit code $LASTEXITCODE"
}

$targetExe = Join-Path $repoRoot "publish/gitriot.exe"
$tmpExePath = Join-Path $repoRoot $tmpExe
try {
    Move-Item -Path $tmpExePath -Destination $targetExe -Force
    Write-Host "Done: publish/gitriot.exe"
}
catch {
    Write-Warning "Could not overwrite publish/gitriot.exe (likely in use). Built artifact left at publish/gitriot.tmp.exe"
}
