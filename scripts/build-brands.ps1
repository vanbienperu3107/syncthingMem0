param(
    [ValidateSet("client", "server", "both")]
    [string]$Mode = "both",
    [string]$GoOsList = "",
    [string]$GoArchList = "",
    [switch]$IncludeInfra,
    [string]$OutDir = "dist/brands"
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

if ([string]::IsNullOrWhiteSpace($GoOsList)) {
    $GoOsList = go env GOOS
}
if ([string]::IsNullOrWhiteSpace($GoArchList)) {
    $GoArchList = go env GOARCH
}

$goosList = $GoOsList -split "," | ForEach-Object { $_.Trim() } | Where-Object { $_ }
$goarchList = $GoArchList -split "," | ForEach-Object { $_.Trim() } | Where-Object { $_ }

function Build-OneTarget {
    param(
        [string]$Target,
        [string]$Goos,
        [string]$Goarch,
        [string]$Destination
    )

    if (Test-Path "${Target}-*-${Goos}-${Goarch}*.zip") {
        Remove-Item "${Target}-*-${Goos}-${Goarch}*.zip" -Force -ErrorAction SilentlyContinue
    }

    Write-Host "[Build] $Target ($Goos/$Goarch)"
    $log = & go run build.go -goos $Goos -goarch $Goarch zip $Target 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host $log
        throw "Build failed for target $Target on $Goos/$Goarch"
    }

    $artifact = Get-ChildItem -Path "." -Filter "${Target}-*-${Goos}-${Goarch}*.zip" | Sort-Object LastWriteTime -Descending | Select-Object -First 1
    if (-not $artifact) {
        Write-Host $log
        throw "No artifact found for $Target on $Goos/$Goarch"
    }

    New-Item -Path $Destination -ItemType Directory -Force | Out-Null
    Copy-Item $artifact.FullName (Join-Path $Destination $artifact.Name) -Force
    Write-Host "[OK] $Target -> $Destination\$($artifact.Name)"
}

function Build-Client {
    param([string]$Goos, [string]$Goarch)

    $out = Join-Path $OutDir "client\$Goos\$Goarch"
    Build-OneTarget -Target "syncthing" -Goos $Goos -Goarch $Goarch -Destination $out
    Copy-Item "docs/ci-cd.md" (Join-Path $out "README-CLIENT-CI.md") -Force
}

function Build-Server {
    param([string]$Goos, [string]$Goarch)

    $out = Join-Path $OutDir "server\$Goos\$Goarch"
    Build-OneTarget -Target "stdiscosrv" -Goos $Goos -Goarch $Goarch -Destination $out
    Build-OneTarget -Target "strelaysrv" -Goos $Goos -Goarch $Goarch -Destination $out

    if ($IncludeInfra.IsPresent) {
        Build-OneTarget -Target "strelaypoolsrv" -Goos $Goos -Goarch $Goarch -Destination $out
        Build-OneTarget -Target "stupgrades" -Goos $Goos -Goarch $Goarch -Destination $out
        Build-OneTarget -Target "stcrashreceiver" -Goos $Goos -Goarch $Goarch -Destination $out
        Build-OneTarget -Target "ursrv" -Goos $Goos -Goarch $Goarch -Destination $out
    }

    Copy-Item "deploy/docker-compose.optional-servers.yml" (Join-Path $out "docker-compose.optional-servers.yml") -Force
    Copy-Item "docs/deploy-servers.md" (Join-Path $out "README-SERVER.md") -Force
    Copy-Item "docs/ci-cd.md" (Join-Path $out "README-SERVER-CI.md") -Force
}

foreach ($goos in $goosList) {
    foreach ($goarch in $goarchList) {
        if ($Mode -in @("client", "both")) {
            Build-Client -Goos $goos -Goarch $goarch
        }
        if ($Mode -in @("server", "both")) {
            Build-Server -Goos $goos -Goarch $goarch
        }
    }
}

Write-Host "Build output:"
Write-Host " - Client brand: $OutDir\client\<goos>\<goarch>"
Write-Host " - Server brand: $OutDir\server\<goos>\<goarch>"
