# Copyright 2022 Namespace Labs Inc; All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.

#Requires -Version 5.1

<#
.SYNOPSIS
    Installs the Namespace nsc CLI (and its credential helpers) on Windows.

.DESCRIPTION
    Downloads the windows release archive for nsc, extracts it, copies the
    executables into an install directory and ensures that directory is on the
    user's PATH.

.PARAMETER Version
    Version to install (for example 0.0.530). Defaults to the latest release.

.PARAMETER Dir
    Directory to install the executables into. Overrides NS_ROOT and
    NS_INSTALL_DIR. Defaults to %LOCALAPPDATA%\ns\bin.

.PARAMETER DryRun
    Print the actions that would be taken without downloading or installing.

.EXAMPLE
    ./install_nsc.ps1

.EXAMPLE
    ./install_nsc.ps1 -Version 0.0.530 -Dir C:\tools\ns
#>

[CmdletBinding()]
param(
    [string]$Version,
    [string]$Dir,
    [switch]$DryRun
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
# Speeds up Invoke-WebRequest downloads considerably.
$ProgressPreference = 'SilentlyContinue'

$toolName = 'nsc'
$dockerCredHelperName = 'docker-credential-nsc'
$bazelCredHelperName = 'bazel-credential-nsc'

function Get-Architecture {
    $procArch = $env:PROCESSOR_ARCHITECTURE
    # 32-bit process on a 64-bit host reports x86; the real arch is in
    # PROCESSOR_ARCHITEW6432.
    if ($procArch -eq 'x86' -and $env:PROCESSOR_ARCHITEW6432) {
        $procArch = $env:PROCESSOR_ARCHITEW6432
    }

    switch ($procArch) {
        'AMD64' { return 'amd64' }
        'ARM64' { return 'arm64' }
        default { return $null }
    }
}

function Add-ToUserPath {
    param([string]$Directory)

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $entries = @()
    if ($userPath) {
        $entries = @($userPath -split ';' | Where-Object { $_ -ne '' })
    }

    if ($entries -notcontains $Directory) {
        $newPath = (@($entries) + $Directory) -join ';'
        [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
        Write-Host "Added $Directory to your user PATH. Restart your terminal for it to take effect."
    }

    # Make it available in the current session too.
    if (($env:Path -split ';') -notcontains $Directory) {
        $env:Path = "$env:Path;$Directory"
    }
}

Write-Host "Executing Namespace's installation script..."

$architecture = Get-Architecture
if (-not $architecture) {
    Write-Error "Unsupported platform architecture '$($env:PROCESSOR_ARCHITECTURE)'. Available only on amd64 and arm64 currently."
    exit 1
}

Write-Host "Detected windows/$architecture as the host platform"

# Resolve the install directory.
if ($Dir) {
    $binDir = $Dir
} elseif ($env:NS_ROOT) {
    $binDir = Join-Path $env:NS_ROOT 'bin'
} elseif ($env:NS_INSTALL_DIR) {
    $binDir = $env:NS_INSTALL_DIR
} else {
    $binDir = Join-Path $env:LOCALAPPDATA 'ns\bin'
}

# Resolve the version to install.
if (-not $Version) {
    Write-Host "Querying latest version..."
    $body = "{`"$toolName`":{}}"
    $response = Invoke-WebRequest -Method Post -ContentType 'application/json' `
        -Body $body -Uri 'https://get.namespace.so/nsl.versions.VersionsService/GetLatest' `
        -UseBasicParsing
    if ($response.Content -match '"version"\s*:\s*"([^"]+)"') {
        $Version = $Matches[1] -replace '^v', ''
    }

    if (-not $Version) {
        Write-Error "Failed to query latest version"
        exit 1
    }

    Write-Host "Latest version: $Version"
}

$archive = "${toolName}_${Version}_windows_${architecture}.zip"
$downloadUri = "https://get.namespace.so/packages/$toolName/v$Version/$archive"

Write-Host "Downloading and installing Namespace $Version from $downloadUri"

if ($DryRun) {
    Write-Host "DRY RUN: would download $downloadUri"
    Write-Host "DRY RUN: would install $toolName, $dockerCredHelperName and $bazelCredHelperName to $binDir"
    Write-Host "DRY RUN: would ensure $binDir is on your PATH"
    Write-Host "Installation complete (dry run)"
    return
}

$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("nsc-install-" + [System.Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

try {
    $zipPath = Join-Path $tempDir $archive

    $headers = @{}
    if ($env:CI) {
        $headers['CI'] = $env:CI
    }

    Invoke-WebRequest -Uri $downloadUri -OutFile $zipPath -UserAgent 'install_nsc.ps1' -Headers $headers -UseBasicParsing
    Expand-Archive -Path $zipPath -DestinationPath $tempDir -Force

    if (-not (Test-Path $binDir)) {
        New-Item -ItemType Directory -Path $binDir -Force | Out-Null
    }

    foreach ($bin in @($toolName, $dockerCredHelperName, $bazelCredHelperName)) {
        $exe = "$bin.exe"
        $src = Join-Path $tempDir $exe
        $dst = Join-Path $binDir $exe
        Copy-Item -Path $src -Destination $dst -Force
        Write-Host "[OK] Installed $exe to $dst"
    }
} finally {
    Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
}

Add-ToUserPath $binDir

Write-Host "Installation complete"
