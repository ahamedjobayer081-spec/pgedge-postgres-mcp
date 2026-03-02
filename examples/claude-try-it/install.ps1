# pgEdge MCP Server — one-command installer for Windows
#
# Usage (interactive, in PowerShell):
#   irm https://raw.githubusercontent.com/pgEdge/pgedge-postgres-mcp/main/examples/claude-try-it/install.ps1 | iex
#
# Usage (non-interactive, via Claude Code):
#   $s = irm https://raw.githubusercontent.com/pgEdge/pgedge-postgres-mcp/main/examples/claude-try-it/install.ps1; & ([scriptblock]::Create($s)) -Demo
#
# Usage (with flags, saved locally):
#   .\install.ps1 -Demo
#   .\install.ps1 -OwnDb -DbHost localhost -DbPort 5432 -DbName mydb -DbUser me -DbPass secret
#
# What it does:
#   1. Downloads the pgEdge MCP Server binary for Windows (x86_64)
#   2. Helps you connect to a database (your own or a demo with sample data)
#   3. Configures Claude Code (.mcp.json) and/or Claude Desktop

param(
    [switch]$Demo,
    [switch]$OwnDb,
    [switch]$InstallDocker,
    [string]$DbHost,
    [string]$DbPort,
    [string]$DbName,
    [string]$DbUser,
    [string]$DbPass
)

$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

# --- Configuration --------------------------------------------------------

$InstallDir = Join-Path $env:USERPROFILE ".pgedge"
$BinDir     = Join-Path $InstallDir "bin"
$DemoDir    = Join-Path $InstallDir "demo"
$Repo       = "pgEdge/pgedge-postgres-mcp"
$DemoPort   = 5432

# --- State variables ------------------------------------------------------

$script:Version      = ""
$script:VersionNum   = ""
$script:DbHost       = $DbHost
$script:DbPort       = $DbPort
$script:DbName       = $DbName
$script:DbUser       = $DbUser
$script:DbPass       = $DbPass
$script:DbConfigured = $false
$script:DemoPort     = $DemoPort

# --- Helper functions -----------------------------------------------------

function Write-Info  { param([string]$Msg) Write-Host "  i  $Msg" }
function Write-Ok    { param([string]$Msg) Write-Host "  +  $Msg" -ForegroundColor Green }
function Write-Warn  { param([string]$Msg) Write-Host "  !  $Msg" -ForegroundColor Yellow }
function Write-Fail  { param([string]$Msg) Write-Host "  x  $Msg" -ForegroundColor Red; exit 1 }

function Test-Interactive {
    try {
        return [Environment]::UserInteractive -and -not [Console]::IsInputRedirected
    } catch {
        return $false
    }
}

function Read-Prompt {
    param([string]$Prompt, [string]$Default)
    if (-not (Test-Interactive)) { return $Default }
    $response = Read-Host -Prompt $Prompt
    if ([string]::IsNullOrWhiteSpace($response)) { return $Default }
    return $response
}

# --- Detect platform ------------------------------------------------------

function Get-Platform {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
        Write-Fail "ARM64 Windows is not yet supported. Only x86_64 builds are available."
    }
    if (-not [System.Environment]::Is64BitOperatingSystem) {
        Write-Fail "32-bit Windows is not supported."
    }
    # Windows x86_64 only
    $script:OS   = "windows"
    $script:Arch = "x86_64"
    $script:Ext  = "zip"
}

# --- Get latest release version -------------------------------------------

function Get-LatestVersion {
    try {
        $release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
        $script:Version = $release.tag_name
    } catch {
        Write-Fail "Could not determine latest release version: $_"
    }
    if (-not $script:Version) {
        Write-Fail "Could not determine latest release version"
    }
    $script:VersionNum = $script:Version.TrimStart('v')
}

# --- Download and install binary ------------------------------------------

function Install-Binary {
    $assetName = "pgedge-postgres-mcp-server_$($script:VersionNum)_$($script:OS)_$($script:Arch).$($script:Ext)"
    $url = "https://github.com/$Repo/releases/download/$($script:Version)/$assetName"
    $tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

    Write-Info "Downloading pgEdge MCP Server $($script:Version) ($($script:OS)/$($script:Arch))..."

    try {
        Invoke-WebRequest -Uri $url -OutFile (Join-Path $tmpDir $assetName) -UseBasicParsing
    } catch {
        Write-Fail "Download failed. Check your internet connection."
    }

    New-Item -ItemType Directory -Path $BinDir -Force | Out-Null

    $extractDir = Join-Path $tmpDir "extracted"
    Expand-Archive -Path (Join-Path $tmpDir $assetName) -DestinationPath $extractDir -Force

    # Find the binary in the extracted archive
    $binaryName = "pgedge-postgres-mcp.exe"
    $source = Get-ChildItem -Path $extractDir -Recurse -Filter $binaryName | Select-Object -First 1
    if (-not $source) {
        $source = Get-ChildItem -Path $extractDir -Recurse -Filter "pgedge-postgres-mcp*" | Select-Object -First 1
    }
    if (-not $source) { Write-Fail "Binary not found in archive" }

    Copy-Item $source.FullName (Join-Path $BinDir $binaryName) -Force
    Remove-Item $tmpDir -Recurse -Force

    Write-Ok "Binary installed: $(Join-Path $BinDir $binaryName)"
}

# --- Docker detection and installation ------------------------------------

function Test-DockerDesktopInstalled {
    # Check standard install path
    if (Test-Path "C:\Program Files\Docker\Docker\Docker Desktop.exe") { return $true }
    # Check registry uninstall key (machine-wide and per-user installs)
    $reg = Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\Docker Desktop" -ErrorAction SilentlyContinue
    if ($reg) { return $true }
    $reg = Get-ItemProperty "HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\Docker Desktop" -ErrorAction SilentlyContinue
    if ($reg) { return $true }
    # Check CLI on PATH (original method, still useful)
    if (Get-Command docker -ErrorAction SilentlyContinue) { return $true }
    return $false
}

function Test-DockerRunning {
    # Named pipe is reliable and doesn't need docker CLI on PATH
    if (Test-Path "\\.\pipe\docker_engine") { return $true }
    # Fallback to docker info if CLI is on PATH
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) { return $false }
    $prevPref = $ErrorActionPreference
    $ErrorActionPreference = 'SilentlyContinue'
    try {
        $null = & docker info 2>$null
        return $LASTEXITCODE -eq 0
    } catch { return $false }
    finally { $ErrorActionPreference = $prevPref }
}

function Install-DockerDesktop {
    # If Docker is already installed but just not running, don't reinstall
    if (Test-DockerDesktopInstalled) {
        Write-Host ""
        Write-Warn "Docker is installed but not running."
        Write-Host ""
        Write-Host "  Please open Docker Desktop from the Start menu and wait"
        Write-Host "  for it to finish starting (the whale icon in the taskbar"
        Write-Host "  will stop animating when it's ready)."
        Write-Host ""
        Write-Host "  Then re-run this installer."
        Write-Host ""
        exit 0
    }

    Write-Host ""
    Write-Info "Installing Docker Desktop..."
    Write-Host ""

    if (Get-Command winget -ErrorAction SilentlyContinue) {
        Write-Info "Installing via winget (this may take several minutes)..."
        & winget install -e --id Docker.DockerDesktop --no-upgrade --accept-source-agreements --accept-package-agreements
        Write-Host ""
        Write-Info "Docker Desktop installed. Please open Docker Desktop from"
        Write-Info "the Start menu, wait for it to start, then re-run this installer."
        exit 0
    } else {
        Write-Host ""
        Write-Host "  Docker Desktop needs to be installed manually."
        Write-Host ""
        Write-Host "  1. Download it from: https://www.docker.com/products/docker-desktop/"
        Write-Host "  2. Run the installer and follow the prompts"
        Write-Host "  3. Launch Docker Desktop and wait for it to start"
        Write-Host "  4. Re-run this installer"
        Write-Host ""
        exit 0
    }
}

# --- Database choice ------------------------------------------------------

function Select-Database {
    if ($Demo) {
        Start-DemoDatabase
        return
    }
    if ($OwnDb) {
        Set-OwnDatabase
        return
    }
    if ($InstallDocker) {
        Install-DockerDesktop
        Start-DemoDatabase
        return
    }

    # Non-interactive — output choices for Claude
    if (-not (Test-Interactive)) {
        Write-Host ""
        Write-Host "DATABASE_CHOICE_NEEDED"
        Write-Host "The MCP server needs a PostgreSQL database to connect to."
        Write-Host "Options:"
        Write-Host "  1. Demo database - sample Northwind data, requires Docker"
        Write-Host "  2. Your own database - provide connection details"
        Write-Host ""
        Write-Host "Re-run with flags:"
        Write-Host "  -Demo                              (start demo database with Docker)"
        Write-Host "  -InstallDocker                     (install Docker first, then demo)"
        Write-Host "  -OwnDb -DbHost HOST -DbPort PORT -DbName DB -DbUser USER -DbPass PASS"
        Write-Host ""
        $script:DbConfigured = $false
        return
    }

    # Interactive
    Write-Host ""
    Write-Host "  The MCP server needs a PostgreSQL database to connect to."
    Write-Host ""
    Write-Host "  Which would you like?"
    Write-Host ""
    Write-Host "    1) Load a sample database (Northwind - customers, orders, products)"
    Write-Host "       Requires Docker. Great for trying things out."
    Write-Host ""
    Write-Host "    2) Connect to my own PostgreSQL database"
    Write-Host "       You'll provide the connection details."
    Write-Host ""

    $choice = Read-Prompt "  Enter 1 or 2" "1"

    switch ($choice) {
        "1" { Start-DemoDatabase }
        "2" { Set-OwnDatabase }
        default { Write-Info "Defaulting to sample database..."; Start-DemoDatabase }
    }
}

# --- Demo database setup --------------------------------------------------

function Start-DemoDatabase {
    if (Test-DockerRunning) {
        Start-DemoPostgres
        return
    }

    # Docker installed but not running?
    if (Test-DockerDesktopInstalled) {
        Write-Host ""
        Write-Warn "Docker is installed but not running."
        Write-Host ""
        Write-Host "  Please open Docker Desktop from the Start menu and wait"
        Write-Host "  for it to finish starting, then re-run this installer."
        Write-Host ""

        if (-not (Test-Interactive)) {
            Write-Host "DOCKER_NOT_RUNNING"
            Write-Host "Start Docker Desktop, then re-run with: -Demo"
            $script:DbConfigured = $false
            return
        }

        Write-Host "  Options:"
        Write-Host ""
        Write-Host "    1) I'll start Docker Desktop and re-run this later"
        Write-Host "    2) Connect to my own database instead"
        Write-Host ""

        $choice = Read-Prompt "  Enter 1 or 2" "1"
        switch ($choice) {
            "2" { Set-OwnDatabase }
            default {
                Write-Host ""
                Write-Host "  Open Docker Desktop, wait for it to start, then re-run this installer."
                Write-Host ""
                $script:DbConfigured = $false
            }
        }
        return
    }

    # Docker not installed at all
    Write-Host ""
    Write-Warn "Docker is not installed."
    Write-Host ""
    Write-Host "  The sample database runs in a Docker container."
    Write-Host "  Docker Desktop is free and takes about 5 minutes to install."
    Write-Host ""

    if (-not (Test-Interactive)) {
        Write-Host "DOCKER_NOT_FOUND"
        Write-Host "To install Docker and set up the demo, re-run with: -InstallDocker"
        Write-Host "To skip the demo and use your own database, re-run with: -OwnDb -DbHost ... -DbPort ... -DbName ... -DbUser ... -DbPass ..."
        $script:DbConfigured = $false
        return
    }

    Write-Host "  Would you like me to install Docker for you?"
    Write-Host ""
    Write-Host "    1) Yes, install Docker"
    Write-Host "    2) No, I'll connect to my own database instead"
    Write-Host "    3) No, I'll install Docker myself and re-run this later"
    Write-Host ""

    $choice = Read-Prompt "  Enter 1, 2, or 3" "3"

    switch ($choice) {
        "1" { Install-DockerDesktop; Start-DemoPostgres }
        "2" { Set-OwnDatabase }
        default {
            Write-Host ""
            Write-Host "  To install Docker Desktop:"
            Write-Host "    https://www.docker.com/products/docker-desktop/"
            Write-Host ""
            Write-Host "  After installing, re-run this installer."
            Write-Host ""
            $script:DbConfigured = $false
        }
    }
}

# --- Find a free port -----------------------------------------------------

function Find-FreePort {
    foreach ($port in 5432, 5433, 5434, 5435, 5436) {
        $inUse = $null
        try {
            $inUse = Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue
        } catch {}
        if (-not $inUse) { return $port }
    }
    # Last resort: let .NET pick
    try {
        $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
        $listener.Start()
        $port = $listener.LocalEndpoint.Port
        $listener.Stop()
        return $port
    } catch {
        return 0
    }
}

# --- Remove old demo containers ------------------------------------------

function Remove-OldDemos {
    try {
        $old = & docker ps -a --filter "name=pgedge-demo-" --format '{{.Names}}' 2>$null
    } catch { return }
    if (-not $old) { return }
    foreach ($name in $old) {
        if ([string]::IsNullOrWhiteSpace($name)) { continue }
        Write-Info "Removing old demo container: $name"
        & docker stop $name 2>$null | Out-Null
        & docker rm -v $name 2>$null | Out-Null
    }
}

# --- Start demo Postgres container ----------------------------------------

function Start-DemoPostgres {
    # Clean up any old demo containers from previous installs
    Remove-OldDemos

    # Generate a unique container name for this run
    $ContainerName = "pgedge-demo-$([DateTimeOffset]::UtcNow.ToUnixTimeSeconds())"

    $script:DemoPort = Find-FreePort
    if ($script:DemoPort -eq 0) {
        Write-Warn "Could not find a free port for the demo database."
        $script:DbConfigured = $false
        return
    }

    if ($script:DemoPort -ne 5432) {
        Write-Info "Port 5432 is in use (probably an existing Postgres instance)."
        Write-Info "Using port $($script:DemoPort) for the demo database instead."
    }

    New-Item -ItemType Directory -Path $DemoDir -Force | Out-Null

    # Write docker-compose.yml
    # Note: $$POSTGRES_USER inside the configs content is docker-compose syntax
    # (double-dollar escapes the dollar sign). Port is interpolated directly.
    $composeContent = @"
services:
  postgres:
    image: ghcr.io/pgedge/pgedge-postgres:17-spock5-standard
    container_name: PGEDGE_CONTAINER_NAME
    command: postgres -c listen_addresses='*' -c shared_preload_libraries='pg_stat_statements'
    environment:
      POSTGRES_USER: demo
      POSTGRES_PASSWORD: demo123
      POSTGRES_DB: northwind
    volumes:
      - pgdata:/var/lib/postgresql/data
    configs:
      - source: load-northwind
        target: /docker-entrypoint-initdb.d/01-load-northwind.sh
        mode: 0755
      - source: enable-extensions
        target: /docker-entrypoint-initdb.d/02-enable-extensions.sh
        mode: 0755
    ports:
      - "$($script:DemoPort):5432"
    restart: unless-stopped
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U demo -d northwind"]
      interval: 5s
      timeout: 5s
      retries: 10
      start_period: 30s

volumes:
  pgdata:
    driver: local

configs:
  load-northwind:
    content: |-
      #!/usr/bin/env bash
      set -e
      echo "Loading Northwind dataset..."
      curl -fsSL -o /tmp/northwind.sql https://downloads.pgedge.com/platform/examples/northwind/northwind.sql
      psql -v ON_ERROR_STOP=1 --username "`$`$POSTGRES_USER" --dbname "`$`$POSTGRES_DB" -f /tmp/northwind.sql
      rm -f /tmp/northwind.sql
      echo "Northwind dataset loaded"
  enable-extensions:
    content: |-
      #!/usr/bin/env bash
      set -e
      psql -v ON_ERROR_STOP=1 --username "`$`$POSTGRES_USER" --dbname "`$`$POSTGRES_DB" \
        -c "CREATE EXTENSION IF NOT EXISTS pg_stat_statements;"
      echo "Extensions enabled"
"@

    $composeContent = $composeContent -replace 'PGEDGE_CONTAINER_NAME', $ContainerName
    $composeContent | Set-Content (Join-Path $DemoDir "docker-compose.yml") -Encoding UTF8

    Write-Host ""
    Write-Info "Starting demo Postgres ($ContainerName) with Northwind sample data on port $($script:DemoPort)..."
    Write-Info "(first run downloads the image - this may take a minute)"
    Write-Host ""

    $composeFile = Join-Path $DemoDir "docker-compose.yml"
    $started = $false
    try {
        & docker compose -f $composeFile up -d 2>$null
        if ($LASTEXITCODE -eq 0) { $started = $true }
    } catch {}

    if (-not $started) {
        try {
            & docker-compose -f $composeFile up -d 2>$null
            if ($LASTEXITCODE -eq 0) { $started = $true }
        } catch {}
    }

    if (-not $started) {
        Write-Warn "Failed to start demo database."
        $script:DbConfigured = $false
        return
    }

    Write-Info "Waiting for database to be ready..."
    for ($i = 0; $i -lt 24; $i++) {
        try {
            & docker exec $ContainerName pg_isready -U demo -d northwind 2>$null | Out-Null
            if ($LASTEXITCODE -eq 0) {
                Write-Ok "Demo database ready (northwind on localhost:$($script:DemoPort), container: $ContainerName)"
                $script:DbHost = "localhost"; $script:DbPort = "$($script:DemoPort)"
                $script:DbName = "northwind"; $script:DbUser = "demo"
                $script:DbPass = "demo123"; $script:DbConfigured = $true
                return
            }
        } catch {}
        Start-Sleep -Seconds 5
    }

    Write-Warn "Database is still starting. It may need another minute."
    $script:DbHost = "localhost"; $script:DbPort = "$($script:DemoPort)"
    $script:DbName = "northwind"; $script:DbUser = "demo"
    $script:DbPass = "demo123"; $script:DbConfigured = $true
}

# --- Database connection test ---------------------------------------------

function Test-DbConnection {
    param([string]$Host_, [int]$Port)
    Write-Info "Testing connection to ${Host_}:${Port}..."
    try {
        $tcp = New-Object System.Net.Sockets.TcpClient
        $completed = $tcp.ConnectAsync($Host_, $Port).Wait(3000)
        if ($completed -and $tcp.Connected) {
            $tcp.Close()
            Write-Ok "Connection to ${Host_}:${Port} succeeded"
            return $true
        }
        $tcp.Close()
    } catch {
        try { if ($tcp) { $tcp.Close() } } catch {}
    }
    return $false
}

function Confirm-OwnDbConnection {
    if (Test-DbConnection $script:DbHost ([int]$script:DbPort)) { return }

    Write-Host ""
    Write-Warn "Could not reach $($script:DbHost):$($script:DbPort) (TCP connection failed)"
    Write-Host ""

    if (-not (Test-Interactive)) {
        Write-Warn "Continuing anyway — verify your connection details are correct."
        return
    }

    Write-Host "  What would you like to do?"
    Write-Host ""
    Write-Host "    1) Re-enter connection details"
    Write-Host "    2) Continue anyway (I'll fix it later)"
    Write-Host ""

    $choice = Read-Prompt "  Enter 1 or 2" "2"
    switch ($choice) {
        "1" {
            # Clear previous values so Set-OwnDatabase re-prompts
            $script:DbHost = ""; $script:DbPort = ""; $script:DbName = ""
            $script:DbUser = ""; $script:DbPass = ""
            Set-OwnDatabase
            return
        }
        default { Write-Warn "Continuing — you can update .mcp.json later with the correct details." }
    }
}

# --- Own database setup ---------------------------------------------------

function Set-OwnDatabase {
    # If connection details were provided via flags
    if ($script:DbHost -and $script:DbName -and $script:DbUser) {
        if (-not $script:DbPort) { $script:DbPort = "5432" }
        $script:DbConfigured = $true
        Write-Ok "Using database: $($script:DbName) on $($script:DbHost):$($script:DbPort)"
        Confirm-OwnDbConnection
        return
    }

    if (-not (Test-Interactive)) {
        Write-Host ""
        Write-Host "OWN_DATABASE_CHOSEN"
        Write-Host "Re-run with connection details:"
        Write-Host "  -OwnDb -DbHost HOST -DbPort PORT -DbName DB -DbUser USER -DbPass PASS"
        $script:DbConfigured = $false
        return
    }

    Write-Host ""
    Write-Host "  Enter your PostgreSQL connection details:"
    Write-Host ""

    $script:DbHost = Read-Prompt "  Host [localhost]" "localhost"
    $script:DbPort = Read-Prompt "  Port [5432]" "5432"
    $script:DbName = Read-Prompt "  Database name" ""
    if (-not $script:DbName) { Write-Warn "Database name is required."; $script:DbConfigured = $false; return }
    $script:DbUser = Read-Prompt "  Username" ""
    if (-not $script:DbUser) { Write-Warn "Username is required."; $script:DbConfigured = $false; return }
    $script:DbPass = Read-Prompt "  Password" ""

    $script:DbConfigured = $true
    Write-Ok "Using database: $($script:DbName) on $($script:DbHost):$($script:DbPort)"
    Confirm-OwnDbConnection
}

# --- Configure Claude Code ------------------------------------------------

function Set-ClaudeCodeConfig {
    $mcpJson = Join-Path $env:USERPROFILE ".claude.json"
    $binaryPath = (Join-Path $BinDir "pgedge-postgres-mcp.exe").Replace('\', '/')

    $pgHost = if ($script:DbHost) { $script:DbHost } else { "localhost" }
    $pgPort = if ($script:DbPort) { $script:DbPort } else { "5432" }
    $pgDb   = if ($script:DbName) { $script:DbName } else { "your_database" }
    $pgUser = if ($script:DbUser) { $script:DbUser } else { "your_user" }
    $pgPass = if ($script:DbPass) { $script:DbPass } else { "your_password" }

    if (Test-Path $mcpJson) {
        try {
            $raw = Get-Content $mcpJson -Raw
            $existing = $raw | ConvertFrom-Json

            # Convert PSCustomObject to allow adding properties
            if (-not $existing.mcpServers) {
                $existing | Add-Member -NotePropertyName "mcpServers" -NotePropertyValue (New-Object PSObject) -Force
            }

            # Add or overwrite the pgedge entry
            $pgedgeObj = New-Object PSObject
            $pgedgeObj | Add-Member -NotePropertyName "command" -NotePropertyValue $binaryPath
            $envObj = New-Object PSObject
            $envObj | Add-Member -NotePropertyName "PGHOST"     -NotePropertyValue $pgHost
            $envObj | Add-Member -NotePropertyName "PGPORT"     -NotePropertyValue $pgPort
            $envObj | Add-Member -NotePropertyName "PGDATABASE" -NotePropertyValue $pgDb
            $envObj | Add-Member -NotePropertyName "PGUSER"     -NotePropertyValue $pgUser
            $envObj | Add-Member -NotePropertyName "PGPASSWORD" -NotePropertyValue $pgPass
            $pgedgeObj | Add-Member -NotePropertyName "env" -NotePropertyValue $envObj

            if ($existing.mcpServers.PSObject.Properties["pgedge"]) {
                $existing.mcpServers.pgedge = $pgedgeObj
            } else {
                $existing.mcpServers | Add-Member -NotePropertyName "pgedge" -NotePropertyValue $pgedgeObj
            }

            $existing | ConvertTo-Json -Depth 10 | Set-Content $mcpJson -Encoding UTF8
            Write-Ok "Claude Code: configured in ~/.claude.json (available in all projects)"
            return
        } catch {
            # Fall through to overwrite
        }
    }

    # Write fresh config
    $config = New-Object PSObject
    $mcpServers = New-Object PSObject
    $pgedgeObj = New-Object PSObject
    $pgedgeObj | Add-Member -NotePropertyName "command" -NotePropertyValue $binaryPath
    $envObj = New-Object PSObject
    $envObj | Add-Member -NotePropertyName "PGHOST"     -NotePropertyValue $pgHost
    $envObj | Add-Member -NotePropertyName "PGPORT"     -NotePropertyValue $pgPort
    $envObj | Add-Member -NotePropertyName "PGDATABASE" -NotePropertyValue $pgDb
    $envObj | Add-Member -NotePropertyName "PGUSER"     -NotePropertyValue $pgUser
    $envObj | Add-Member -NotePropertyName "PGPASSWORD" -NotePropertyValue $pgPass
    $pgedgeObj | Add-Member -NotePropertyName "env" -NotePropertyValue $envObj
    $mcpServers | Add-Member -NotePropertyName "pgedge" -NotePropertyValue $pgedgeObj
    $config | Add-Member -NotePropertyName "mcpServers" -NotePropertyValue $mcpServers

    $config | ConvertTo-Json -Depth 10 | Set-Content $mcpJson -Encoding UTF8
    Write-Ok "Claude Code: configured in ~/.claude.json (available in all projects)"
}

# --- Configure Claude Desktop ---------------------------------------------

function Set-ClaudeDesktopConfig {
    $binaryPath = (Join-Path $BinDir "pgedge-postgres-mcp.exe").Replace('\', '/')
    $configFile = Join-Path $env:APPDATA "Claude\claude_desktop_config.json"
    $configDir = Split-Path $configFile

    if (-not (Test-Path $configDir)) {
        Write-Info "Claude Desktop not detected - skipping config"
        return
    }

    $pgHost = if ($script:DbHost) { $script:DbHost } else { "localhost" }
    $pgPort = if ($script:DbPort) { $script:DbPort } else { "5432" }
    $pgDb   = if ($script:DbName) { $script:DbName } else { "your_database" }
    $pgUser = if ($script:DbUser) { $script:DbUser } else { "your_user" }
    $pgPass = if ($script:DbPass) { $script:DbPass } else { "your_password" }

    # Build the pgedge entry as PSCustomObject (PS 5.1 compatible)
    $pgedgeObj = New-Object PSObject
    $pgedgeObj | Add-Member -NotePropertyName "command" -NotePropertyValue $binaryPath
    $envObj = New-Object PSObject
    $envObj | Add-Member -NotePropertyName "PGHOST"     -NotePropertyValue $pgHost
    $envObj | Add-Member -NotePropertyName "PGPORT"     -NotePropertyValue $pgPort
    $envObj | Add-Member -NotePropertyName "PGDATABASE" -NotePropertyValue $pgDb
    $envObj | Add-Member -NotePropertyName "PGUSER"     -NotePropertyValue $pgUser
    $envObj | Add-Member -NotePropertyName "PGPASSWORD" -NotePropertyValue $pgPass
    $pgedgeObj | Add-Member -NotePropertyName "env" -NotePropertyValue $envObj

    $config = $null
    if (Test-Path $configFile) {
        try {
            $raw = Get-Content $configFile -Raw
            $config = $raw | ConvertFrom-Json
        } catch {
            $config = $null
        }
    }

    if (-not $config) {
        $config = New-Object PSObject
    }

    if (-not $config.mcpServers) {
        $config | Add-Member -NotePropertyName "mcpServers" -NotePropertyValue (New-Object PSObject) -Force
    }

    if ($config.mcpServers.PSObject.Properties["pgedge"]) {
        $config.mcpServers.pgedge = $pgedgeObj
    } else {
        $config.mcpServers | Add-Member -NotePropertyName "pgedge" -NotePropertyValue $pgedgeObj
    }

    New-Item -ItemType Directory -Path $configDir -Force | Out-Null
    $config | ConvertTo-Json -Depth 10 | Set-Content $configFile -Encoding UTF8

    Write-Ok "Claude Desktop: configured (restart Claude Desktop to activate)"
}

# --- Summary --------------------------------------------------------------

function Write-Summary {
    Write-Host ""
    Write-Host ([char]0x2550) * 54 # horizontal double line
    Write-Host "  Installation complete!"
    Write-Host ([char]0x2550) * 54
    Write-Host ""
    Write-Host "  Binary:   $(Join-Path $BinDir 'pgedge-postgres-mcp.exe')"

    if ($script:DbConfigured) {
        Write-Host "  Database: $($script:DbName) on $($script:DbHost):$($script:DbPort) ($($script:DbUser))"
        Write-Host ""
        Write-Host "  Try asking Claude:"
        Write-Host '    "What tables are in my database?"'
        Write-Host '    "Show me the top 10 products by sales"'
        Write-Host '    "Which customers have placed more than 5 orders?"'
    } else {
        Write-Host "  Database: not yet configured"
        Write-Host ""
        Write-Host "  To configure later, edit:"
        Write-Host "    Claude Code:    ~/.claude.json"
        Write-Host "    Claude Desktop: $env:APPDATA\Claude\claude_desktop_config.json"
    }

    Write-Host ""
    Write-Host "  Claude Code:    ready - start a new conversation"
    Write-Host "  Claude Desktop: restart the app, then start chatting"
    Write-Host ""
    Write-Host ([char]0x2550) * 54
    Write-Host ""
}

# --- Main -----------------------------------------------------------------

function Main {
    Write-Host ""
    Write-Host ([char]0x2550) * 54
    Write-Host "  pgEdge MCP Server - Installer"
    Write-Host ([char]0x2550) * 54
    Write-Host ""
    Write-Host "  This will install the pgEdge MCP Server so you can"
    Write-Host "  query PostgreSQL databases using natural language"
    Write-Host "  in Claude Code or Claude Desktop."
    Write-Host ""

    $script:DbConfigured = $false

    Get-Platform
    Get-LatestVersion
    Install-Binary

    Write-Host ""
    Select-Database

    Write-Host ""
    Set-ClaudeCodeConfig
    Set-ClaudeDesktopConfig

    Write-Summary
}

Main
