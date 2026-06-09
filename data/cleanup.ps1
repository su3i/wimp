#Requires -RunAsAdministrator
$ErrorActionPreference = "SilentlyContinue"

$FBDir    = ".\fluent-bit"
$AgentDir = ".\agent"
$TmpDir   = ".\tmp"

function Remove-Svc($name) {
    $svc = Get-Service -Name $name -ErrorAction SilentlyContinue
    if (-not $svc) {
        Write-Host "  [SKIP] $name (not found)"
        return
    }
    if ($svc.Status -eq "Running") {
        Stop-Service -Name $name -Force
        Write-Host "  [STOP] $name"
    }
    & sc.exe delete $name | Out-Null
    Write-Host "  [DEL]  $name"
}

# ── 1. windows_exporter ───────────────────────────────────────────────────────
Write-Host ""
Write-Host "[1/3] Removing windows_exporter..."
Remove-Svc "windows_exporter"

$uninstallKey = Get-ChildItem "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall" -ErrorAction SilentlyContinue |
    Get-ItemProperty |
    Where-Object { $_.DisplayName -like "*windows_exporter*" } |
    Select-Object -First 1

if ($uninstallKey) {
    Start-Process msiexec.exe -Wait -ArgumentList "/x `"$($uninstallKey.PSChildName)`" /quiet"
    Write-Host "  [OK]   MSI uninstalled"
} else {
    Write-Host "  [SKIP] MSI entry not found"
}

# ── 2. Fluent Bit ─────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "[2/3] Removing Fluent Bit..."
Remove-Svc "fluent-bit"
if (Test-Path $FBDir) {
    Remove-Item -Path $FBDir -Recurse -Force
    Write-Host "  [DEL]  $FBDir"
}

# ── 3. wimp Agent ─────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "[3/3] Removing wimp agent..."
Remove-Svc "wimp-agent"
if (Test-Path $AgentDir) {
    Remove-Item -Path $AgentDir -Recurse -Force
    Write-Host "  [DEL]  $AgentDir"
}

# ── Cleanup tmp ───────────────────────────────────────────────────────────────
if (Test-Path $TmpDir) {
    Remove-Item -Path $TmpDir -Recurse -Force
    Write-Host "  [DEL]  $TmpDir"
}

Write-Host ""
Write-Host "Uninstall complete." -ForegroundColor Green
