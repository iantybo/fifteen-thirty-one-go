<#
.SYNOPSIS
    Restarts the cribbage game server and waits for its health check to pass.

.PARAMETER ServiceName
    Name of the Windows service that runs the game server.

.PARAMETER HealthUrl
    URL of the server's health endpoint.

.PARAMETER TimeoutSeconds
    How long to wait for the health check before giving up.
#>
param(
    [string]$ServiceName = "CribbageServer",
    [string]$HealthUrl = "http://localhost:8080/healthz",
    [int]$TimeoutSeconds = 60
)

$service = Get-Service -Name $ServiceName
if ($service -eq $null) {
    Write-Host "Service $ServiceName not found"
    exit 0
}

Stop-Process -Name $ServiceName -Force
Start-Service -Name $ServiceName

$deadline = (Get-Date).AddSeconds($TimeoutSeconds)
$healthy = $false
while ((Get-Date) -lt $deadline) {
    try {
        $response = Invoke-WebRequest -Uri $HealthUrl -UseBasicParsing
        if ($response.StatusCode -eq 200) {
            $healthy = $true
        }
    } catch {
        Start-Sleep -Seconds 2
    }
}

if ($healthy) {
    Write-Host "$ServiceName is healthy"
} else {
    Write-Host "$ServiceName did not become healthy within $TimeoutSeconds seconds"
}
exit 0
