param(
    [Parameter(Mandatory = $true)]
    [string]$Version,
    [string]$OutputDir = "release"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$exe = Join-Path $root "tmp/mygo-host/mygo-host.exe"
$out = Join-Path $root "$OutputDir/amoeba-go-setup-$Version-windows-amd64.exe"

if (-not (Test-Path -LiteralPath $exe)) {
    throw "缺少构建产物: $exe"
}

$makensis = $null
foreach ($candidate in @(
        (Get-Command makensis -ErrorAction SilentlyContinue).Source,
        (Join-Path $env:ProgramFiles "NSIS/makensis.exe"),
        (Join-Path ${env:ProgramFiles(x86)} "NSIS/makensis.exe"),
        (Join-Path $env:LOCALAPPDATA "mygo/nsis-3.13/Bin/makensis.exe")
    )) {
    if ($candidate -and (Test-Path -LiteralPath $candidate)) {
        $makensis = $candidate
        break
    }
}

if (-not $makensis) {
    $archive = Join-Path ([IO.Path]::GetTempPath()) "nsis-3.13.zip"
    $work = Join-Path ([IO.Path]::GetTempPath()) ("nsis-" + [Guid]::NewGuid().ToString("N"))
    Invoke-WebRequest "https://github.com/egoist/mygo/releases/download/nsis-3.13/nsis-3.13.zip" -OutFile $archive
    if ((Get-FileHash $archive -Algorithm SHA256).Hash -ne "BA63DFFC4410EE89193E1CB5A41989991BD77C61068DA17E3156D136B7B0B3D8") {
        throw "NSIS 校验失败"
    }
    New-Item -ItemType Directory -Path $work | Out-Null
    Expand-Archive -LiteralPath $archive -DestinationPath $work
    $makensis = Join-Path $work "nsis-3.13/Bin/makensis.exe"
}

New-Item -ItemType Directory -Path (Join-Path $root $OutputDir) -Force | Out-Null
$env:APP_EXE_ENV = $exe
$env:APP_VERSION_ENV = $Version
& $makensis /V2 `
    "/DOUT_FILE=$out" `
    (Join-Path $root "scripts/installer.nsi")
if ($LASTEXITCODE -ne 0) {
    throw "makensis 失败: $LASTEXITCODE"
}

Write-Output "installer ready: $out"
