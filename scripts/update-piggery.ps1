# update-piggery.ps1 -- update the local piggery.exe from the fork's latest release.
# Run on the machine AFTER the fork's build-windows workflow published a release:
#   powershell -NoProfile -ExecutionPolicy Bypass -File update-piggery.ps1 [-ForkOwner <github-user>]
# Steps: query latest release asset -> verify sha256 -> rename-swap binary -> restart daemon.
# Never pushes anywhere; the daemon restart re-reads ~/.piggery/config.yaml.
# NOTE: this file is pure ASCII on purpose -- Windows PowerShell 5.1 misreads
# non-BOM UTF-8 files containing non-ASCII characters as ANSI and fails to parse.

param(
    [Parameter(Mandatory = $true)]
    [string]$ForkOwner,                      # your GitHub username (fork owner)
    [string]$Repo = "piggery-winport",
    [string]$TargetPath = "$env:USERPROFILE\.local\bin\piggery.exe",
    [switch]$CheckOnly
)

$ErrorActionPreference = "Stop"

# --- 1. latest release from the FORK (not upstream) ---
$api = "https://api.github.com/repos/$ForkOwner/$Repo/releases/latest"
Write-Host "Querying $api"
$rel = Invoke-RestMethod -Uri $api -Headers @{ "User-Agent" = "piggery-updater" }
$asset = $rel.assets | Where-Object { $_.name -eq "piggery-windows-amd64.exe" }
if (-not $asset) { throw "Release $($rel.tag_name) has no piggery-windows-amd64 asset." }
Write-Host "Latest fork release: $($rel.tag_name)  (asset: $($asset.name))"

# --- 2. current state: fresh install or in-place update ---
if (-not (Test-Path $TargetPath)) {
    Write-Host "No local binary at $TargetPath -- will install fresh from $($rel.tag_name)." -ForegroundColor Yellow
    if ($CheckOnly) { exit 0 }
    $backup = $null
    $current = "(none)"
} else {
    $current = & $TargetPath --version 2>$null
    Write-Host "Current binary:      $current"
    if ($current -like "*$($rel.tag_name)*") {
        Write-Host "Already up to date ($($rel.tag_name)). Nothing to do." -ForegroundColor Green
        exit 0
    }
    if ($CheckOnly) { Write-Host "CheckOnly: would update $current -> $($rel.tag_name)"; exit 0 }
    # rename-swap happens once, in section 4, after download + sha verification.
    # The old daemon keeps running the old binary from memory until step 5 restarts it.
    $backup = "$TargetPath.old"
    if (Test-Path $backup) { Remove-Item $backup -Force }
}

# --- 3. download asset + checksum to temp ---
$tmp = Join-Path $env:TEMP "piggery-update-$PID"
New-Item -ItemType Directory -Path $tmp -Force | Out-Null
try {
    $dl = Join-Path $tmp "piggery-windows-amd64.exe"
    Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $dl -Headers @{ "User-Agent" = "piggery-updater" }

    $shaAsset = $rel.assets | Where-Object { $_.name -eq "piggery-windows-amd64.exe.sha256" }
    if ($shaAsset) {
        $shaFile = Join-Path $tmp "expected.sha256"
        Invoke-WebRequest -Uri $shaAsset.browser_download_url -OutFile $shaFile -Headers @{ "User-Agent" = "piggery-updater" }
        $expected = (Get-Content $shaFile).Split()[0].Trim()
        $actual = (Get-FileHash $dl -Algorithm SHA256).Hash.ToLower()
        if ($actual -ne $expected) { throw "SHA256 mismatch! expected=$expected actual=$actual -- aborting." }
        Write-Host "SHA256 verified: $actual" -ForegroundColor Green
    } else {
        Write-Warning "No .sha256 asset in release -- skipping checksum verification."
    }

    # --- 4. install/swap (running exe holds a write lock, not a rename lock -- proven on omp.exe) ---
    if ($backup) {
        Move-Item $TargetPath $backup
        Move-Item $dl $TargetPath
    } else {
        # fresh install: no old binary to back up
        New-Item -ItemType Directory -Path (Split-Path $TargetPath -Parent) -Force | Out-Null
        Move-Item $dl $TargetPath
    }
    $newVer = & $TargetPath --version
    Write-Host "Swapped: $current -> $newVer" -ForegroundColor Green

    # --- 5. restart daemon to re-read config with the new binary ---
    & "$TargetPath" restart
    Start-Sleep -Seconds 2
    & "$TargetPath" ps --json | ConvertFrom-Json | ForEach-Object {
        Write-Host "Daemon after restart: pid=$($_.pid) version=$($_.version)"
    }
    Write-Host "Update complete: $current -> $newVer" -ForegroundColor Green
    if ($backup) {
        Write-Host "Rollback if needed: copy '$backup' back over '$TargetPath', then '$TargetPath restart'."
    }
}
finally {
    if (Test-Path $tmp) { Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue }
}
