$ErrorActionPreference = "Stop"
$projectRoot = $PSScriptRoot

function Refresh-Path {
    $env:Path = "$env:USERPROFILE\.bun\bin;" +
        [Environment]::GetEnvironmentVariable("Path", "Machine") + ";" +
        [Environment]::GetEnvironmentVariable("Path", "User") + ";" + $env:Path
}

Refresh-Path
foreach ($tool in @(
    @{ Command = "go"; Package = "GoLang.Go" },
    @{ Command = "gcc"; Package = "BrechtSanders.WinLibs.POSIX.UCRT" }
)) {
    if (-not (Get-Command $tool.Command -ErrorAction SilentlyContinue)) {
        if (-not (Get-Command winget -ErrorAction SilentlyContinue)) {
            throw "Install App Installer to enable winget, then run this installer again."
        }
        & winget install --id $tool.Package --exact --accept-package-agreements --accept-source-agreements
        if ($LASTEXITCODE -ne 0) { throw "Could not install $($tool.Package)." }
        Refresh-Path
    }
}

$bunVersion = (Get-Content "$projectRoot\frontend\package.json" -Raw | ConvertFrom-Json).packageManager -replace "^bun@", ""
$bun = Get-Command bun -ErrorAction SilentlyContinue
if (-not $bun -or ([version](((& bun --version) -split "-")[0])) -lt [version]$bunVersion) {
    & ([scriptblock]::Create((Invoke-RestMethod https://bun.sh/install.ps1))) -Version $bunVersion
    Refresh-Path
}

foreach ($command in @("go", "gcc", "bun")) {
    if (-not (Get-Command $command -ErrorAction SilentlyContinue)) {
        throw "$command is unavailable. Restart your terminal and run this installer again."
    }
}

Write-Host "Installing project dependencies..."
$previousToolchain = $env:GOTOOLCHAIN
try {
    $env:GOTOOLCHAIN = "auto"
    & go -C "$projectRoot\backend" mod download
    if ($LASTEXITCODE -ne 0) { throw "Could not download backend dependencies." }
} finally {
    $env:GOTOOLCHAIN = $previousToolchain
}
& bun install --cwd "$projectRoot\frontend" --frozen-lockfile
if ($LASTEXITCODE -ne 0) { throw "Could not install frontend dependencies." }
Write-Host "Dependencies ready."
