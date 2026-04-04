#Requires -Version 5.1
<#
.SYNOPSIS
    Interactive installer for YinMoNote on Windows.

.DESCRIPTION
    Installs the YinMoNote binary and configures it as either a Task Scheduler
    task (user-level, no admin required) or a Windows Service (system-level,
    requires administrator privileges).

.PARAMETER Binary
    Path to the yinmonote.exe binary. Defaults to yinmonote.exe in the same
    directory as this script.
#>

param(
    [string]$Binary = ""
)

[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$ErrorActionPreference = "Stop"

# ── Resolve binary path ───────────────────────────────────────────────────────
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
if ([string]::IsNullOrEmpty($Binary)) {
    $Binary = Join-Path $ScriptDir "yinmonote.exe"
}

if (-not (Test-Path $Binary -PathType Leaf)) {
    Write-Host "Error: binary not found: $Binary" -ForegroundColor Red
    Write-Host "Usage: .\Install.ps1 [-Binary <path-to-yinmonote.exe>]"
    exit 1
}

# ── Admin detection ───────────────────────────────────────────────────────────
$IsAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole(
    [Security.Principal.WindowsBuiltInRole]::Administrator
)

# ── Defaults ──────────────────────────────────────────────────────────────────
$DefaultDataDir = Join-Path $env:USERPROFILE ".yinmonote\notes"
$DefaultPort    = "8080"
$YinMoNoteDir   = Join-Path $env:USERPROFILE ".yinmonote"

# ── Install mode selection ────────────────────────────────────────────────────
# 1 = Task Scheduler (user-level), 2 = Windows Service (system-level)
$InstallMode = 1

if ($IsAdmin) {
    Write-Host ""
    Write-Host "-- Install Mode -----------------------------------------"
    Write-Host "  1) Task Scheduler  -- user-level, starts at login (no admin needed)"
    Write-Host "  2) Windows Service -- system-level, starts at boot (admin)"
    Write-Host ""
    $modeInput = Read-Host "  Choice [1/2] (default: 1)"
    if ($modeInput -eq "2") {
        $InstallMode = 2
    }
} else {
    Write-Host ""
    Write-Host "  (Running without administrator privileges -- using Task Scheduler mode)"
}

# ── Mode-aware directory defaults ─────────────────────────────────────────────
# Windows Service runs as SYSTEM, which cannot access %USERPROFILE%.
# Use C:\ProgramData\YinMoNote so the service can read/write its own files.
if ($InstallMode -eq 2) {
    $DefaultDataDir = Join-Path $env:ProgramData "YinMoNote\notes"
    $YinMoNoteDir   = Join-Path $env:ProgramData "YinMoNote"
}

# ── Upgrade detection ─────────────────────────────────────────────────────────
$IsUpgrade         = $false
$ExistingDataDir   = ""
$ExistingPort      = ""
$ExistingStatus    = ""
$ExistingTlsMode   = ""   # "self" | "acme" | ""
$ExistingAcmeDomain = ""
$ExistingExtraIPs  = ""
$ExistingWebDavDisabled = ""

if ($InstallMode -eq 1) {
    # Task Scheduler upgrade detection
    $existingTask = Get-ScheduledTask -TaskName "YinMoNote" -ErrorAction SilentlyContinue
    if ($existingTask) {
        $IsUpgrade = $true
        # Read config from Launch.ps1 if it exists
        $existingInstallDir = Join-Path $env:LOCALAPPDATA "YinMoNote"
        $existingLaunch = Join-Path $existingInstallDir "Launch.ps1"
        if (Test-Path $existingLaunch) {
            $launchContent = Get-Content $existingLaunch -Raw -ErrorAction SilentlyContinue
            if ($launchContent) {
                if ($launchContent -match '\$env:DATA_DIR\s*=\s*"([^"]*)"') { $ExistingDataDir = $Matches[1] }
                if ($launchContent -match '\$env:PORT\s*=\s*"([^"]*)"')     { $ExistingPort = $Matches[1] }
                if ($launchContent -match '\$env:TLS_SELF\s*=\s*"1"')       { $ExistingTlsMode = "self" }
                if ($launchContent -match '\$env:ACME_DOMAIN\s*=\s*"([^"]+)"') {
                    $ExistingAcmeDomain = $Matches[1]; $ExistingTlsMode = "acme"
                }
                if ($launchContent -match '\$env:TLS_EXTRA_IPS\s*=\s*"([^"]*)"') { $ExistingExtraIPs = $Matches[1] }
                if ($launchContent -match '\$env:WEBDAV_DISABLED\s*=\s*"1"') { $ExistingWebDavDisabled = "1" }
            }
        }
        # Check running status
        $taskInfo = Get-ScheduledTask -TaskName "YinMoNote" -ErrorAction SilentlyContinue
        if ($taskInfo -and $taskInfo.State -eq "Running") {
            $ExistingStatus = "running"
        } else {
            $ExistingStatus = "stopped"
        }
    }
} else {
    # Windows Service upgrade detection
    $svcQuery = & sc.exe query YinMoNote 2>&1
    if ($LASTEXITCODE -eq 0) {
        $IsUpgrade = $true
        # Read env vars from registry
        $regPath = "HKLM:\SYSTEM\CurrentControlSet\Services\YinMoNote"
        if (Test-Path $regPath) {
            $envMultiStr = (Get-ItemProperty -Path $regPath -Name "Environment" -ErrorAction SilentlyContinue).Environment
            if ($envMultiStr) {
                foreach ($line in $envMultiStr) {
                    if ($line -match '^DATA_DIR=(.+)$')         { $ExistingDataDir = $Matches[1] }
                    if ($line -match '^PORT=(.+)$')             { $ExistingPort = $Matches[1] }
                    if ($line -match '^TLS_SELF=1$')            { $ExistingTlsMode = "self" }
                    if ($line -match '^ACME_DOMAIN=(.+)$')      { $ExistingAcmeDomain = $Matches[1]; $ExistingTlsMode = "acme" }
                    if ($line -match '^TLS_EXTRA_IPS=(.+)$')    { $ExistingExtraIPs = $Matches[1] }
                    if ($line -match '^WEBDAV_DISABLED=1$')     { $ExistingWebDavDisabled = "1" }
                }
            }
        }
        if ($svcQuery -match "RUNNING") {
            $ExistingStatus = "running"
        } else {
            $ExistingStatus = "stopped"
        }
    }
}

# ── Banner ────────────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "================================================"
if ($IsUpgrade) {
    Write-Host "  YinMoNote Upgrade"
} else {
    Write-Host "  YinMoNote Install"
}
Write-Host "================================================"

if ($IsUpgrade) {
    Write-Host ""
    Write-Host "  Found existing installation:"
    Write-Host "    Status:  $ExistingStatus"
    if ($ExistingDataDir)  { Write-Host "    Notes:   $ExistingDataDir" }
    if ($ExistingPort)     { Write-Host "    Port:    $ExistingPort" }
    Write-Host ""
    Write-Host "  The binary will be replaced and the service restarted."
    Write-Host "  Your notes and config are not affected."
    Write-Host ""
    $confirmInput = Read-Host "  Continue? [Y/n]"
    if ($confirmInput -match '^[nN]') {
        Write-Host "  Aborted."
        exit 0
    }
}

# ── Helper: determine default from existing or fallback ──────────────────────
function Coalesce($a, $b) { if ([string]::IsNullOrEmpty($a)) { $b } else { $a } }

# ── Prompt: data directory ────────────────────────────────────────────────────
$promptData = Coalesce $ExistingDataDir $DefaultDataDir
# Validate stored default is an absolute path (starts with drive letter)
if ($promptData -notmatch '^[A-Za-z]:\\') { $promptData = $DefaultDataDir }

Write-Host ""
$DataDir = ""
while ($true) {
    $inputData = Read-Host "Notes directory [$promptData]"
    if ([string]::IsNullOrWhiteSpace($inputData)) { $inputData = $promptData }
    if ($inputData -match '^[A-Za-z]:\\') {
        $DataDir = $inputData
        break
    }
    Write-Host "  Error: please enter an absolute Windows path (must start with a drive letter, e.g. C:\)" -ForegroundColor Yellow
}

# ── Prompt: port ──────────────────────────────────────────────────────────────
$rawExistingPort = Coalesce $ExistingPort $DefaultPort
# Strip any address prefix (e.g. "127.0.0.1:8080" -> "8080", ":8080" -> "8080")
if ($rawExistingPort -match ':(\d+)$') { $rawExistingPort = $Matches[1] }
$promptPort = if ($rawExistingPort -match '^\d+$') { $rawExistingPort } else { $DefaultPort }

$inputPort = Read-Host "Port [$promptPort]"
$Port = if ([string]::IsNullOrWhiteSpace($inputPort)) { $promptPort } else { $inputPort }
# Ensure we have just the port number
if ($Port -match ':(\d+)$') { $Port = $Matches[1] }

# ── Prompt: access mode ───────────────────────────────────────────────────────
Write-Host ""
Write-Host "-- Access Mode ------------------------------------------"
Write-Host "  1) Localhost only -- personal use on this machine"
Write-Host "  2) LAN or public server -- other devices access over the network"

# Infer existing access mode from PORT value
$existingAccess = ""
if ($ExistingPort -match '^(localhost:|127\.0\.0\.1:)') { $existingAccess = "local" }
elseif ($ExistingPort -match '^:') { $existingAccess = "network" }
elseif ($ExistingPort -match '^\d') { $existingAccess = "network" }

$accessDefault = if ($existingAccess -eq "network") { "2" } else { "1" }
$accessInput = Read-Host "  Choice [1/2] (default: $accessDefault)"
$accessChoice = if ([string]::IsNullOrWhiteSpace($accessInput)) { $accessDefault } else { $accessInput }

if ($accessChoice -eq "2") {
    $PortBinding = ":$Port"        # 0.0.0.0:PORT -- all interfaces
} else {
    $PortBinding = "localhost:$Port"   # loopback only
}

# ── Prompt: HTTPS ─────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "-- HTTPS ------------------------------------------------"
Write-Host "  1) No HTTPS -- HTTP only (suitable for local/trusted network)"
Write-Host "  2) Self-signed certificate -- HTTPS without a domain name"
Write-Host "     (download CA cert once per device from /ca.crt)"
Write-Host "  3) Let's Encrypt -- automatic HTTPS with a domain name"
Write-Host "     (requires a valid domain pointing to this server)"

$tlsDefault = switch ($ExistingTlsMode) {
    "self" { "2" }
    "acme" { "3" }
    default { "1" }
}
$tlsInput = Read-Host "  Choice [$tlsDefault]"
$tlsChoice = if ([string]::IsNullOrWhiteSpace($tlsInput)) { $tlsDefault } else { $tlsInput }

$TlsSelf    = ""
$AcmeDomain = ""
$TlsExtraIPs = ""

switch ($tlsChoice) {
    "2" {
        $TlsSelf = "1"
        Write-Host ""
        Write-Host "  The certificate will include all IP addresses currently assigned to"
        Write-Host "  this machine. If your public IP is on an upstream NAT gateway (common"
        Write-Host "  on cloud VPS), it won't be detected automatically -- enter it here."
        $extraPrompt = Coalesce $ExistingExtraIPs ""
        if ($extraPrompt) {
            $extraInput = Read-Host "  Public/extra IPs, comma-separated [$extraPrompt]"
        } else {
            $extraInput = Read-Host "  Public/extra IPs (leave blank if not needed)"
        }
        $TlsExtraIPs = if ([string]::IsNullOrWhiteSpace($extraInput)) { $extraPrompt } else { $extraInput }
    }
    "3" {
        $acmePrompt = Coalesce $ExistingAcmeDomain ""
        if ($acmePrompt) {
            $acmeInput = Read-Host "  Domain name [$acmePrompt]"
        } else {
            $acmeInput = Read-Host "  Domain name"
        }
        $AcmeDomain = if ([string]::IsNullOrWhiteSpace($acmeInput)) { $acmePrompt } else { $acmeInput }
        if ([string]::IsNullOrEmpty($AcmeDomain)) {
            Write-Host "  Error: domain name is required for Let's Encrypt. Falling back to HTTP." -ForegroundColor Yellow
        }
    }
}

# ── Prompt: WebDAV ────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "-- WebDAV -----------------------------------------------"
Write-Host "  Allows mobile apps (Obsidian, iA Writer, etc.) to access notes"
Write-Host "  at /dav/ using the same password as the web app."

$davDefault = if ($ExistingWebDavDisabled -eq "1") { "N" } else { "Y" }
$davInput = Read-Host "  Enable WebDAV? [$davDefault]"
$davChoice = if ([string]::IsNullOrWhiteSpace($davInput)) { $davDefault } else { $davInput }

$WebDavDisabled = ""
if ($davChoice -match '^[nN]') { $WebDavDisabled = "1" }

# ── Config paths ──────────────────────────────────────────────────────────────
$ConfigFile = Join-Path $YinMoNoteDir "config.json"

# ── Summary ───────────────────────────────────────────────────────────────────
Write-Host ""
$modeLabel = if ($InstallMode -eq 2) { "Windows Service" } else { "Task Scheduler" }
Write-Host "  Mode:    $modeLabel"
Write-Host "  Notes:   $DataDir"
Write-Host "  Config:  $ConfigFile"
Write-Host "  Port:    $PortBinding"
if ($PortBinding -match '^localhost:') {
    Write-Host "  Access:  Localhost only"
} else {
    Write-Host "  Access:  LAN / public server"
}
if ($AcmeDomain) {
    Write-Host "  HTTPS:   Let's Encrypt ($AcmeDomain)"
} elseif ($TlsSelf -eq "1") {
    if ($TlsExtraIPs) {
        Write-Host "  HTTPS:   Self-signed certificate (extra IPs: $TlsExtraIPs)"
    } else {
        Write-Host "  HTTPS:   Self-signed certificate"
    }
} else {
    Write-Host "  HTTPS:   Disabled (HTTP only)"
}
if ($WebDavDisabled -eq "1") {
    Write-Host "  WebDAV:  Disabled"
} else {
    Write-Host "  WebDAV:  Enabled (/dav/)"
}
Write-Host ""

# ── Install binary ────────────────────────────────────────────────────────────
if ($InstallMode -eq 2) {
    $InstallDir = Join-Path $env:ProgramFiles "YinMoNote"
} else {
    $InstallDir = Join-Path $env:LOCALAPPDATA "YinMoNote"
}

Write-Host "==> Installing binary to $InstallDir ..."
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$DestExe = Join-Path $InstallDir "yinmonote.exe"

# Stop existing service/task before replacing binary
if ($IsUpgrade) {
    if ($InstallMode -eq 1) {
        Write-Host "==> Stopping existing task..."
        Stop-ScheduledTask -TaskName "YinMoNote" -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 2
    } else {
        Write-Host "==> Stopping existing service..."
        & sc.exe stop YinMoNote 2>&1 | Out-Null
        Start-Sleep -Seconds 3
    }
}

Copy-Item -Path $Binary -Destination $DestExe -Force

# ── Create data / config directories ─────────────────────────────────────────
Write-Host "==> Creating directories..."
New-Item -ItemType Directory -Path $DataDir    -Force | Out-Null
New-Item -ItemType Directory -Path $YinMoNoteDir -Force | Out-Null

# ═══════════════════════════════════════════════════════════════════════════════
# TASK SCHEDULER MODE
# ═══════════════════════════════════════════════════════════════════════════════
if ($InstallMode -eq 1) {

    # Build Launch.ps1 content
    $launchLines = @()
    $launchLines += '# Launch.ps1 -- generated by Install.ps1 -- do not edit manually'
    $launchLines += '# To reconfigure, re-run Install.ps1'
    $launchLines += ''
    $launchLines += "`$env:DATA_DIR    = `"$DataDir`""
    $launchLines += "`$env:CONFIG_FILE = `"$ConfigFile`""
    $launchLines += "`$env:PORT        = `"$PortBinding`""
    if ($TlsSelf -eq "1")                       { $launchLines += "`$env:TLS_SELF    = `"1`"" }
    if (-not [string]::IsNullOrEmpty($AcmeDomain)) { $launchLines += "`$env:ACME_DOMAIN = `"$AcmeDomain`"" }
    if (-not [string]::IsNullOrEmpty($TlsExtraIPs)) { $launchLines += "`$env:TLS_EXTRA_IPS = `"$TlsExtraIPs`"" }
    if ($WebDavDisabled -eq "1")                { $launchLines += "`$env:WEBDAV_DISABLED = `"1`"" }
    $launchLines += ''
    $launchLines += "& `"`$PSScriptRoot\yinmonote.exe`""

    $LaunchScript = Join-Path $InstallDir "Launch.ps1"
    Write-Host "==> Writing $LaunchScript ..."
    $launchLines | Set-Content -Path $LaunchScript -Encoding UTF8

    # Register Task Scheduler task
    Write-Host "==> Registering scheduled task 'YinMoNote'..."

    $action = New-ScheduledTaskAction `
        -Execute "powershell.exe" `
        -Argument "-ExecutionPolicy Bypass -WindowStyle Hidden -File `"$LaunchScript`""

    $trigger = New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME

    $settings = New-ScheduledTaskSettingsSet `
        -ExecutionTimeLimit ([TimeSpan]::Zero) `
        -RestartCount 3 `
        -RestartInterval (New-TimeSpan -Minutes 1) `
        -StartWhenAvailable $true

    Register-ScheduledTask `
        -TaskName "YinMoNote" `
        -Action $action `
        -Trigger $trigger `
        -Settings $settings `
        -RunLevel Limited `
        -Force | Out-Null

    Write-Host "==> Starting task..."
    Start-ScheduledTask -TaskName "YinMoNote"

# ═══════════════════════════════════════════════════════════════════════════════
# WINDOWS SERVICE MODE
# ═══════════════════════════════════════════════════════════════════════════════
} else {

    if ($IsUpgrade) {
        Write-Host "==> Removing old service registration..."
        & sc.exe delete YinMoNote 2>&1 | Out-Null
        Start-Sleep -Seconds 2
    }

    Write-Host "==> Creating Windows service 'YinMoNote'..."
    & sc.exe create YinMoNote `
        binPath= "`"$DestExe`"" `
        start= auto `
        DisplayName= "YinMoNote Note Server"
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Error: sc.exe create failed (exit code $LASTEXITCODE)" -ForegroundColor Red
        exit 1
    }

    & sc.exe description YinMoNote "Self-hosted personal note library"

    # Write environment variables to registry (MultiString)
    Write-Host "==> Configuring service environment..."
    $regPath = "HKLM:\SYSTEM\CurrentControlSet\Services\YinMoNote"
    $envVars = [System.Collections.Generic.List[string]]::new()
    $envVars.Add("DATA_DIR=$DataDir")
    $envVars.Add("CONFIG_FILE=$ConfigFile")
    $envVars.Add("PORT=$PortBinding")
    if ($TlsSelf -eq "1")                           { $envVars.Add("TLS_SELF=1") }
    if (-not [string]::IsNullOrEmpty($AcmeDomain))  { $envVars.Add("ACME_DOMAIN=$AcmeDomain") }
    if (-not [string]::IsNullOrEmpty($TlsExtraIPs)) { $envVars.Add("TLS_EXTRA_IPS=$TlsExtraIPs") }
    if ($WebDavDisabled -eq "1")                    { $envVars.Add("WEBDAV_DISABLED=1") }

    Set-ItemProperty -Path $regPath -Name "Environment" -Value $envVars.ToArray() -Type MultiString

    Write-Host "==> Starting service..."
    & sc.exe start YinMoNote
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Warning: service start returned exit code $LASTEXITCODE -- check Event Viewer for details." -ForegroundColor Yellow
    }
}

# ── Windows Firewall rule (network access mode) ───────────────────────────────
# Only needed when binding to all interfaces (:PORT); localhost-only mode needs
# no rule because Windows Firewall doesn't block loopback traffic.
if ($accessChoice -eq "2") {
    Write-Host "==> Adding Windows Firewall inbound rule for port $Port..."
    $fwResult = & netsh advfirewall firewall add rule `
        name="YinMoNote" `
        dir=in `
        action=allow `
        protocol=TCP `
        localport=$Port 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "  Warning: could not add firewall rule (exit $LASTEXITCODE). You may need to add it manually:" -ForegroundColor Yellow
        Write-Host "    netsh advfirewall firewall add rule name=`"YinMoNote`" dir=in action=allow protocol=TCP localport=$Port"
    }
}

# ── Build access URL ──────────────────────────────────────────────────────────
$PortNum = $Port   # bare port number
if ($AcmeDomain) {
    $BaseUrl = "https://$AcmeDomain"
} elseif ($TlsSelf -eq "1") {
    if ($PortBinding -match '^localhost:') {
        $BaseUrl = "https://localhost:${PortNum}"
    } else {
        $BaseUrl = "https://<server-ip>:${PortNum}"
    }
} else {
    if ($PortBinding -match '^localhost:') {
        $BaseUrl = "http://localhost:${PortNum}"
    } else {
        $BaseUrl = "http://<server-ip>:${PortNum}"
    }
}

# ── Self-signed CA cert instructions ─────────────────────────────────────────
if ($TlsSelf -eq "1") {
    $CaCertPath = Join-Path $YinMoNoteDir "selfca\ca.crt"
    Write-Host ""
    Write-Host "  Self-signed TLS -- install the CA cert once per device:"
    Write-Host ""
    Write-Host "  This machine:"
    Write-Host "    certutil -addstore -user Root `"$CaCertPath`""
    Write-Host "    Or: double-click the file -> Install Certificate -> Current User -> Trusted Root CAs"
    Write-Host ""
    Write-Host "  Remote devices: $BaseUrl/ca.crt"
}

# ── Done ──────────────────────────────────────────────────────────────────────
Write-Host ""
if ($IsUpgrade) {
    Write-Host "  OK  Upgraded successfully -- service restarted" -ForegroundColor Green
} else {
    Write-Host "  OK  Installation complete, service started" -ForegroundColor Green
}
Write-Host ""
Write-Host "  Open:    $BaseUrl"
if ($WebDavDisabled -ne "1") {
    Write-Host "  WebDAV:  $BaseUrl/dav/"
}
Write-Host ""
Write-Host "  Notes:   $DataDir"
Write-Host "  Config:  $ConfigFile"
Write-Host ""

if ($InstallMode -eq 1) {
    Write-Host "  Manage (Task Scheduler):"
    Write-Host "    Start:   Start-ScheduledTask -TaskName YinMoNote"
    Write-Host "    Stop:    Stop-ScheduledTask  -TaskName YinMoNote"
    Write-Host "    Status:  Get-ScheduledTask   -TaskName YinMoNote"
    Write-Host ""
    Write-Host "  Uninstall:"
    Write-Host "    Unregister-ScheduledTask -TaskName `"YinMoNote`" -Confirm:`$false"
    Write-Host "    Then delete: $InstallDir"
} else {
    Write-Host "  Manage (Windows Service):"
    Write-Host "    Start:   sc.exe start YinMoNote"
    Write-Host "    Stop:    sc.exe stop  YinMoNote"
    Write-Host "    Status:  sc.exe query YinMoNote"
    Write-Host ""
    Write-Host "  Uninstall:"
    Write-Host "    sc.exe stop YinMoNote"
    Write-Host "    sc.exe delete YinMoNote"
    Write-Host "    Then delete: $InstallDir"
}

Write-Host "================================================"
