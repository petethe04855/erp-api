$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root

$env:PORT = if ($env:PORT) { $env:PORT } else { "8080" }
$env:GOCACHE = if ($env:GOCACHE) { $env:GOCACHE } else { Join-Path $root ".gocache" }
New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null

$process = $null
$pendingRestart = $false

function Stop-Server {
    if ($script:process -and -not $script:process.HasExited) {
        Write-Host "Stopping API..."
        Stop-Process -Id $script:process.Id -Force
        $script:process.WaitForExit()
    }
}

function Start-Server {
    Stop-Server
    Write-Host "Starting API on port $env:PORT..."
    $script:process = Start-Process -FilePath "go" `
        -ArgumentList @("run", "./cmd/server") `
        -WorkingDirectory $root `
        -NoNewWindow `
        -PassThru
}

$watcher = New-Object System.IO.FileSystemWatcher
$watcher.Path = $root
$watcher.Filter = "*.go"
$watcher.IncludeSubdirectories = $true
$watcher.EnableRaisingEvents = $true

$action = {
    $path = $Event.SourceEventArgs.FullPath
    if ($path -like "*\tmp\*" -or $path -like "*\.git\*") {
        return
    }

    $script:pendingRestart = $true
}

$events = @(
    Register-ObjectEvent $watcher Changed -Action $action
    Register-ObjectEvent $watcher Created -Action $action
    Register-ObjectEvent $watcher Deleted -Action $action
    Register-ObjectEvent $watcher Renamed -Action $action
)

try {
    Start-Server
    Write-Host "Watching Go files. Press q then Enter to stop."

    while ($true) {
        Start-Sleep -Milliseconds 300

        if ([Console]::KeyAvailable) {
            $key = [Console]::ReadKey($true)
            if ($key.KeyChar -eq "q") {
                break
            }
        }

        if ($pendingRestart) {
            $pendingRestart = $false
            Start-Sleep -Milliseconds 300
            Write-Host "Change detected. Restarting API..."
            Start-Server
        }
    }
}
finally {
    Stop-Server
    $events | ForEach-Object { Unregister-Event -SubscriptionId $_.Id -ErrorAction SilentlyContinue }
    $watcher.Dispose()
}
