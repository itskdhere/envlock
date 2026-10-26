# EnvLock 1-line installer for Windows (PowerShell)
# Usage: irm https://envlock.itskdhere.com/install.ps1 | iex
# Fallback: irm https://raw.githubusercontent.com/itskdhere/envlock/main/scripts/install.ps1 | iex

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$Repo = "itskdhere/envlock"
$InstallDir = if ($env:ENVLOCK_INSTALL_DIR) { $env:ENVLOCK_INSTALL_DIR } else { "$Home\.envlock\bin" }

Write-Host "Installing EnvLock on Windows..." -ForegroundColor Cyan

# 1. Determine Architecture
$Arch = "amd64"
if ($env:PROCESSOR_ARCHITECTURE -ne "AMD64" -and $env:PROCESSOR_ARCHITEW6432 -ne "AMD64") {
    Write-Error "Unsupported architecture: $env:PROCESSOR_ARCHITECTURE. EnvLock currently supports Windows AMD64."
    return
}

Write-Host "Detected platform: windows/$Arch"

# 2. Determine latest version
if ($env:VERSION) {
    $Tag = $env:VERSION
} else {
    Write-Host "Fetching latest release from GitHub..."
    $Tag = $null

    # Try release redirect first to avoid GitHub API rate limits
    try {
        $Req = [System.Net.HttpWebRequest]::Create("https://github.com/$Repo/releases/latest")
        $Req.AllowAutoRedirect = $false
        $Req.UserAgent = "EnvLock-Installer"
        $Resp = $Req.GetResponse()
        $Location = $Resp.GetResponseHeader("Location")
        $Resp.Close()
        if ($Location) {
            $Tag = $Location.Substring($Location.LastIndexOf('/') + 1).Trim()
        }
    } catch {
        # Redirect did not resolve, fallback to REST API
    }

    if (-not $Tag) {
        try {
            $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing -Headers @{ "User-Agent" = "EnvLock-Installer" }
            if ($Release -and $Release.tag_name) {
                $Tag = $Release.tag_name.Trim()
            }
        } catch {
            # REST API did not resolve
        }
    }

    if (-not $Tag) {
        $Tag = "v0.2.1"
    }
}

$CleanVer = $Tag -replace '^[vV]', ''
$FileName = "envlock_${CleanVer}_windows_amd64.zip"
$DownloadUrl = "https://github.com/$Repo/releases/download/$Tag/$FileName"
$ChecksumUrl = "https://github.com/$Repo/releases/download/$Tag/checksums.txt"

Write-Host "Downloading EnvLock $Tag from $DownloadUrl..."

# 3. Download and extract to temporary directory
$TempZip = Join-Path ([System.IO.Path]::GetTempPath()) $FileName
$TempChecksum = Join-Path ([System.IO.Path]::GetTempPath()) "checksums_$([System.Guid]::NewGuid().ToString('N')).txt"
$TempExtract = Join-Path ([System.IO.Path]::GetTempPath()) "envlock_install_$([System.Guid]::NewGuid().ToString('N'))"

$HasCurl = [bool](Get-Command curl.exe -ErrorAction SilentlyContinue)
$IsInteractive = [Environment]::UserInteractive -and (-not [Console]::IsOutputRedirected)

try {
    try {
        if ($HasCurl) {
            $CurlArgs = if ($IsInteractive) { @("-#", "-fLo", $TempZip, $DownloadUrl) } else { @("-sSL", "-f", "-o", $TempZip, $DownloadUrl) }
            & curl.exe @CurlArgs
            if ($LASTEXITCODE -ne 0) {
                Write-Error "Error: Failed to download release asset from $DownloadUrl"
                return
            }
        } else {
            Invoke-WebRequest -Uri $DownloadUrl -OutFile $TempZip -UseBasicParsing
        }
    } catch {
        Write-Error "Error: Failed to download release asset from $DownloadUrl"
        return
    }

    # Verify SHA-256 Checksum
    Write-Host "Verifying SHA-256 checksum..."
    $ExpectedHash = $null
    try {
        if ($HasCurl) {
            & curl.exe -sSL -f -o $TempChecksum $ChecksumUrl
        } else {
            Invoke-WebRequest -Uri $ChecksumUrl -OutFile $TempChecksum -UseBasicParsing
        }
        if (Test-Path $TempChecksum) {
            $Lines = Get-Content $TempChecksum
            foreach ($line in $Lines) {
                if ($line -match "\s+$([regex]::Escape($FileName))$") {
                    $ExpectedHash = ($line -split '\s+')[0].Trim().ToLower()
                    break
                }
            }
        }
    } catch {
        # Checksums file unavailable (e.g. custom pre-release tag)
    }

    if ($ExpectedHash) {
        $ActualHash = (Get-FileHash -Path $TempZip -Algorithm SHA256).Hash.ToLower()
        if ($ExpectedHash -ne $ActualHash) {
            Write-Error "Error: Checksum verification failed for $FileName!`n  Expected: $ExpectedHash`n  Actual:   $ActualHash"
            return
        }
        Write-Host "Checksum verified successfully ($($ActualHash.Substring(0, 16))...)" -ForegroundColor Green
    }

    Expand-Archive -Path $TempZip -DestinationPath $TempExtract -Force

    # 4. Install binaries (envlock.exe & el.exe)
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }

    Write-Host "Installing to $InstallDir..."
    $EnvlockExe = Join-Path $InstallDir "envlock.exe"
    $ElExe = Join-Path $InstallDir "el.exe"

    Copy-Item (Join-Path $TempExtract "envlock.exe") $EnvlockExe -Force

    # Create link for el.exe -> envlock.exe (HardLink on NTFS; fallback to copy if unsupported)
    if (Test-Path $ElExe) {
        Remove-Item -Path $ElExe -Force -ErrorAction SilentlyContinue
    }

    $LinkType = "copy"
    try {
        New-Item -ItemType HardLink -Path $ElExe -Target $EnvlockExe -Force -ErrorAction Stop | Out-Null
        $LinkType = "hardlink to envlock.exe"
    } catch {
        Copy-Item (Join-Path $TempExtract "el.exe") $ElExe -Force
    }

    Write-Host "`nSuccessfully installed EnvLock into: $InstallDir" -ForegroundColor Green
    Write-Host "  • envlock -> $EnvlockExe" -ForegroundColor Green
    Write-Host "  • el      -> $LinkType" -ForegroundColor Green
    Write-Host ""

    # 5. Add to User PATH if not present
    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $Paths = if ($UserPath) { $UserPath -split ';' } else { @() }
    $NormalizedInstallDir = $InstallDir.TrimEnd('\')
    $AlreadyInPath = $Paths | Where-Object { $_.TrimEnd('\') -ieq $NormalizedInstallDir }

    if (-not $AlreadyInPath) {
        $NewPath = (($Paths + $InstallDir) -join ';').Trim(';')
        [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
        $env:Path = "$env:Path;$InstallDir"
        Write-Host "Notice: Added $InstallDir to your User PATH!" -ForegroundColor Yellow
        Write-Host "You can now run 'envlock' or 'el' from any terminal." -ForegroundColor Yellow
        Write-Host ""
    }

    & $EnvlockExe --version
} finally {
    Remove-Item $TempZip -ErrorAction SilentlyContinue
    Remove-Item $TempExtract -Recurse -Force -ErrorAction SilentlyContinue
    if ($TempChecksum) { Remove-Item $TempChecksum -ErrorAction SilentlyContinue }
}
