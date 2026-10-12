#Requires -RunAsAdministrator
$ErrorActionPreference = "Stop"

# ── Baked-in values (substituted by control plane at request time) ─────────────
$ControlPlaneUrl   = "{{.ControlPlaneUrl}}"
$AgentExeUrl       = "{{.AgentExeUrl}}"
$RegistrationToken = "{{.RegistrationToken}}"
$MachineId         = {{.MachineId}}
$LokiHost          = "{{.LokiHost}}"
$LokiPort          = {{.LokiPort}}

# ── Pinned release versions ────────────────────────────────────────────────────
$WinExpVersion    = "0.31.7"
$FluentBitVersion = "5.0.7"

# ── Install directories ────────────────────────────────────────────────────────
$BaseDir  = [System.IO.Path]::GetFullPath(".")
$FBDir    = [System.IO.Path]::GetFullPath(".\fluent-bit")
$AgentDir = [System.IO.Path]::GetFullPath(".\wimp")
$TmpDir   = [System.IO.Path]::GetFullPath(".\tmp")
# Prometheus text files the agent writes for windows_exporter to serve (SQL metrics that need
# a SQL login, which windows_exporter's own collectors cannot read).
$TextfileDir = [System.IO.Path]::GetFullPath(".\textfile")

foreach ($d in @($BaseDir, $FBDir, $AgentDir, $TmpDir, $TextfileDir)) {
    if (-not (Test-Path $d)) { New-Item -ItemType Directory -Path $d | Out-Null }
}

function Get-File($url, $dest) {
    Write-Host "    $url"
    Invoke-WebRequest -Uri $url -OutFile $dest -UseBasicParsing
}

function Remove-Existing($name) {
    Stop-Service -Name $name -Force -ErrorAction SilentlyContinue
    & sc.exe delete $name 2>&1 | Out-Null
    Get-Process -Name $name -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 1
}

function Install-Service($name, $binPath) {
    New-Service -Name $name -BinaryPathName $binPath -StartupType Automatic | Out-Null
}

function Test-ServiceRunning($name) {
    $svc = Get-Service -Name $name -ErrorAction SilentlyContinue
    if ($svc -and $svc.Status -eq "Running") {
        Write-Host "  [OK]   $name"
        return $true
    }
    $st = if ($svc) { $svc.Status } else { "NotFound" }
    Write-Host "  [FAIL] $name ($st)"
    return $false
}

# ── 1. windows_exporter ───────────────────────────────────────────────────────
Write-Host ""
Write-Host "[1/3] Installing windows_exporter v$WinExpVersion..."
$msi = "$TmpDir\windows_exporter.msi"
Get-File "https://github.com/prometheus-community/windows_exporter/releases/download/v$WinExpVersion/windows_exporter-$WinExpVersion-amd64.msi" $msi

# Collectors follow what the host runs, detected the same way the agent discovers it.
# Names must exist in the pinned $WinExpVersion: windows_exporter refuses to start at all on
# an unknown one (0.30 removed "cs", for example).
# Workloads installed after bootstrap need bootstrap re-run to get their collector, which works
# because an existing install is removed first (below).
$collectors = @("cpu", "logical_disk", "memory", "net", "os", "service", "system", "tcp", "textfile")
if (Test-Path "$env:SystemRoot\System32\inetsrv\appcmd.exe") {
    $collectors += "iis"
    Write-Host "  IIS detected"
}
if (Test-Path "HKLM:\SOFTWARE\Microsoft\Microsoft SQL Server\Instance Names\SQL") {
    $collectors += "mssql"
    Write-Host "  SQL Server detected"
}
$clusSvc = Get-Service -Name "ClusSvc" -ErrorAction SilentlyContinue
if ($clusSvc -and $clusSvc.Status -eq "Running") {
    $collectors += "mscluster"
    Write-Host "  Failover cluster node detected"
}

# Installing the MSI over an existing install of the same version is a no-op that keeps the old
# collector list, so a re-run would silently change nothing. Remove it first.
$existing = Get-ChildItem "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall" -ErrorAction SilentlyContinue |
    Get-ItemProperty |
    Where-Object { $_.DisplayName -like "*windows_exporter*" } |
    Select-Object -First 1
if ($existing) {
    Write-Host "  Removing existing windows_exporter $($existing.DisplayVersion)..."
    Stop-Service -Name "windows_exporter" -Force -ErrorAction SilentlyContinue
    Start-Process msiexec.exe -Wait -ArgumentList "/x `"$($existing.PSChildName)`" /quiet"
}

Start-Process msiexec.exe -Wait -ArgumentList @(
    "/i", $msi, "/quiet",
    "ENABLED_COLLECTORS=$($collectors -join ',')",
    "TEXTFILE_DIRS=`"$TextfileDir`"",
    "ADD_FIREWALL_EXCEPTION=yes"
)
Write-Host "  Done."

# ── 2. Fluent Bit ─────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "[2/3] Installing Fluent Bit v$FluentBitVersion..."
Remove-Existing "fluent-bit"

$fbZip = "$TmpDir\fluent-bit.zip"
Get-File "https://packages.fluentbit.io/windows/fluent-bit-$FluentBitVersion-win64.zip" $fbZip
Expand-Archive -Path $fbZip -DestinationPath $TmpDir -Force

$extracted = Get-ChildItem $TmpDir -Directory | Where-Object { $_.Name -like "fluent-bit*" } | Select-Object -First 1
Copy-Item "$($extracted.FullName)\*" $FBDir -Recurse -Force

$fbConf = @"
[SERVICE]
    Flush     5
    Log_Level info

[INPUT]
    Name         winlog
    Channels     Application,System
    Interval_Sec 1
    Tag          winlog.*

[FILTER]
    Name      throttle
    Match     winlog.*
    Rate      3000
    Window    5
    Interval  1s

[FILTER]
    Name    record_modifier
    Match   winlog.*
    Record  machine_id $MachineId
    Record  hostname   $env:COMPUTERNAME

[OUTPUT]
    Name     loki
    Match    winlog.*
    Host     $LokiHost
    Port     $LokiPort
    Labels   job=wimp,machine_id=$MachineId
    Workers  1
"@
Set-Content "$FBDir\fluent-bit.conf" $fbConf

$fbExe = if (Test-Path "$FBDir\bin\fluent-bit.exe") { "$FBDir\bin\fluent-bit.exe" } else { "$FBDir\fluent-bit.exe" }
Install-Service "fluent-bit" "$fbExe -c $FBDir\fluent-bit.conf"
Write-Host "  Done."

# ── 3. wimp Agent ─────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "[3/3] Installing wimp agent..."
Remove-Existing "wimp-agent"

$agentExe = "$AgentDir\agent.exe"
Get-File $AgentExeUrl $agentExe

@{
    control_plane_url  = $ControlPlaneUrl
    registration_token = $RegistrationToken
    machine_id         = $MachineId
    base_dir           = $BaseDir
    agent_dir          = $AgentDir
    fluent_bit_dir     = $FBDir
    tmp_dir            = $TmpDir
    textfile_dir       = $TextfileDir
} | ConvertTo-Json | Set-Content "$AgentDir\config.json"

Install-Service "wimp-agent" $agentExe
Write-Host "  Done."

# ── Start services ─────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "Starting services..."
foreach ($svc in @("windows_exporter", "fluent-bit", "wimp-agent")) {
    Start-Service -Name $svc -ErrorAction SilentlyContinue
}

Start-Sleep -Seconds 3

# ── Verify ─────────────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "Verifying..."
$allOk = $true
foreach ($svc in @("windows_exporter", "fluent-bit", "wimp-agent")) {
    if (-not (Test-ServiceRunning $svc)) { $allOk = $false }
}

# windows_exporter exits at startup on a bad collector or flag and says why only in the event
# log, so surface that here rather than leaving it to be dug out by hand.
if (-not (Get-Service "windows_exporter" -ErrorAction SilentlyContinue | Where-Object Status -eq "Running")) {
    $why = Get-EventLog -LogName Application -Source windows_exporter -Newest 5 -ErrorAction SilentlyContinue |
        Where-Object { $_.Message -match 'err=|level=error' } |
        Select-Object -First 1
    if ($why) { Write-Host "         windows_exporter: $($why.Message)" -ForegroundColor Yellow }
}

Write-Host ""
if ($allOk) {
    Write-Host "Bootstrap complete. Machine ID $MachineId is now online." -ForegroundColor Green
} else {
    Write-Host "Bootstrap finished with one or more failures. Check service logs." -ForegroundColor Red
    exit 1
}
