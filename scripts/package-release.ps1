param(
    [Parameter(Mandatory = $true)]
    [string]$Version,
    [string]$OutputDir = "release"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$name = "amoeba-go-$Version-windows-amd64"
$stage = Join-Path $root "tmp/release/$name"
$zip = Join-Path $root "$OutputDir/$name.zip"
$exe = Join-Path $root "tmp/mygo-host/mygo-host.exe"

if (-not (Test-Path -LiteralPath $exe)) {
    throw "缺少构建产物: $exe"
}

if (Test-Path -LiteralPath $stage) {
    Remove-Item -LiteralPath $stage -Recurse -Force
}
New-Item -ItemType Directory -Path $stage -Force | Out-Null
Copy-Item -LiteralPath $exe -Destination (Join-Path $stage "amoeba-go.exe")

foreach ($component in Get-ChildItem -LiteralPath (Join-Path $root "components") -Directory) {
    $out = Join-Path $component.FullName "out"
    if (-not (Test-Path -LiteralPath (Join-Path $out "page/component.json"))) {
        continue
    }
    $dest = Join-Path $stage "components/$($component.Name)/out"
    New-Item -ItemType Directory -Path $dest -Force | Out-Null
    Copy-Item -Path (Join-Path $out "*") -Destination $dest -Recurse -Force
}

$vendor = Join-Path $root "web/page-vendor/out/dist"
if (-not (Test-Path -LiteralPath (Join-Path $vendor "manifest.json"))) {
    throw "缺少 vendor 共享层: $vendor"
}
$vendorDest = Join-Path $stage "web/page-vendor/out/dist"
New-Item -ItemType Directory -Path $vendorDest -Force | Out-Null
Copy-Item -Path (Join-Path $vendor "*") -Destination $vendorDest -Recurse -Force

Copy-Item -LiteralPath (Join-Path $root "README.md") -Destination $stage
New-Item -ItemType Directory -Path (Join-Path $root $OutputDir) -Force | Out-Null
Compress-Archive -Path (Join-Path $stage "*") -DestinationPath $zip -CompressionLevel Optimal -Force

Write-Output "release ready: $zip"
