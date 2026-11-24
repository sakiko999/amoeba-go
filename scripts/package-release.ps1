param(
    [Parameter(Mandatory = $true)]
    [string]$Version,
    [string]$OutputDir = "release"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$name = "amoeba-go-$Version-windows-amd64.exe"
$target = Join-Path $root "$OutputDir/$name"
$exe = Join-Path $root "tmp/mygo-host/mygo-host.exe"

if (-not (Test-Path -LiteralPath $exe)) {
    throw "缺少构建产物: $exe"
}

New-Item -ItemType Directory -Path (Join-Path $root $OutputDir) -Force | Out-Null
Copy-Item -LiteralPath $exe -Destination $target -Force

Write-Output "portable ready: $target"
