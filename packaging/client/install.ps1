<#
  syncthingMem0 - CLIENT installer (Windows, PowerShell)

  Cài binary + TU TAO CAU HINH cho may client ket noi toi hub qua WSS:443.
  Tham so lay tu bien moi truong, neu thieu se hoi tuong tac:
    HUB_URL        vi du: vps.example.com:443   (bat buoc)
    HUB_DEVICE_ID  device ID cua hub             (bat buoc de ghep cap)
    HUB_TOKEN      JWT do hub cap                (tuy chon)
    FOLDER_PATH    thu muc dong bo  (mac dinh: %USERPROFILE%\SyncMem0)
    FOLDER_ID      id thu muc       (mac dinh: default)
    BIN_DIR        noi cai binary   (mac dinh: %LOCALAPPDATA%\syncthingMem0)
    STHOMEDIR      thu muc config   (mac dinh: %LOCALAPPDATA%\syncthingMem0\config)

  Dung: chuot phai -> Run with PowerShell, hoac:  powershell -ExecutionPolicy Bypass -File install.ps1
#>
$ErrorActionPreference = "Stop"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$binSrc = Join-Path $scriptDir "syncthing.exe"
if (-not (Test-Path $binSrc)) { throw "Khong tim thay syncthing.exe canh script." }

function Get-Val($envName, $default, $question) {
  $v = [Environment]::GetEnvironmentVariable($envName)
  if ([string]::IsNullOrEmpty($v)) { $v = $default }
  if ([string]::IsNullOrEmpty($v) -and [Environment]::UserInteractive) {
    $v = Read-Host $question
  }
  return $v
}

$binDir   = Get-Val "BIN_DIR"   (Join-Path $env:LOCALAPPDATA "syncthingMem0") $null
$cfgDir   = Get-Val "STHOMEDIR" (Join-Path $binDir "config") $null
$hubUrl   = Get-Val "HUB_URL"   "" "Dia chi hub (host:443)"
$hubId    = Get-Val "HUB_DEVICE_ID" "" "Device ID cua hub"
$hubToken = Get-Val "HUB_TOKEN" "" "Hub token (Enter de bo qua)"
$folderPath = Get-Val "FOLDER_PATH" (Join-Path $env:USERPROFILE "SyncMem0") $null
$folderId   = Get-Val "FOLDER_ID" "default" $null

Write-Host ">> Cai binary vao $binDir"
New-Item -ItemType Directory -Force -Path $binDir, $cfgDir, $folderPath | Out-Null
Copy-Item $binSrc (Join-Path $binDir "syncthingmem0.exe") -Force
$bin = Join-Path $binDir "syncthingmem0.exe"

Write-Host ">> Tao cau hinh + khoa thiet bi tai $cfgDir"
$env:STHOMEDIR = $cfgDir
& $bin generate --no-port-probing | Out-Host

$conf = Join-Path $cfgDir "config.xml"
$selfId = (& $bin device-id) 2>$null
if ([string]::IsNullOrWhiteSpace($selfId)) {
  $selfId = ([xml](Get-Content $conf)).configuration.device[0].id
}

if (-not [string]::IsNullOrEmpty($hubUrl) -and -not [string]::IsNullOrEmpty($hubId)) {
  Write-Host ">> Cau hinh ket noi toi hub $hubUrl ($hubId)"
  [xml]$xml = Get-Content $conf

  $dev = $xml.CreateElement("device"); $dev.SetAttribute("id", $hubId)
  $dev.SetAttribute("name", "hub"); $dev.SetAttribute("compression", "metadata")
  $addr = $xml.CreateElement("address"); $addr.InnerText = "wss://$hubUrl"; $dev.AppendChild($addr) | Out-Null
  $paused = $xml.CreateElement("paused"); $paused.InnerText = "false"; $dev.AppendChild($paused) | Out-Null
  $xml.configuration.AppendChild($dev) | Out-Null

  $fld = $xml.CreateElement("folder")
  $fld.SetAttribute("id", $folderId); $fld.SetAttribute("label", "SyncMem0")
  $fld.SetAttribute("path", $folderPath); $fld.SetAttribute("type", "sendreceive")
  foreach ($id in @($selfId, $hubId)) {
    $d = $xml.CreateElement("device"); $d.SetAttribute("id", $id); $fld.AppendChild($d) | Out-Null
  }
  $xml.configuration.AppendChild($fld) | Out-Null

  if (-not [string]::IsNullOrEmpty($hubToken)) {
    $t = $xml.CreateElement("deviceToken"); $t.InnerText = $hubToken
    $xml.configuration.AppendChild($t) | Out-Null
  }
  $xml.Save($conf)
}

Write-Host ""
Write-Host "=================================================================="
Write-Host " Cai dat xong."
Write-Host " Binary : $bin"
Write-Host " Config : $cfgDir"
Write-Host " Thu muc: $folderPath"
Write-Host " Device ID cua may nay:"
Write-Host "   $selfId"
Write-Host " -> Hay authorize device ID nay tren hub."
Write-Host ""
Write-Host " Chay client:"
Write-Host "   `$env:STHOMEDIR='$cfgDir'; & '$bin' serve"
Write-Host "=================================================================="
