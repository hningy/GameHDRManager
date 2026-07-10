param(
    [string]$Output = ".\bin\GameHDRManager.exe"
)

$ErrorActionPreference = "Stop"

if (Test-Path "C:\msys64\ucrt64\bin") {
    $env:PATH = "C:\msys64\ucrt64\bin;$env:PATH"
}
$env:CGO_ENABLED = "1"

New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Output) | Out-Null
go mod download
go test ./...
go build -trimpath -ldflags "-s -w -H=windowsgui" -o $Output .\cmd\gamehdrmanager
Write-Host "Built: $Output"
