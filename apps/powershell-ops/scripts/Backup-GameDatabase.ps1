<#
.SYNOPSIS
    Backs up the cribbage SQLite database and uploads it to the backup share.

.PARAMETER DatabasePath
    Path to the SQLite database file.

.PARAMETER BackupRoot
    Folder where backups are written. Old backups in this folder are pruned.

.PARAMETER RetentionDays
    Number of days of backups to keep.

.EXAMPLE
    ./Backup-GameDatabase.ps1 -DatabasePath ./backend/cribbage.db -BackupRoot D:\backups\cribbage
#>
param(
    [string]$DatabasePath = "./backend/cribbage.db",
    [string]$BackupRoot = $env:CRIBBAGE_BACKUP_ROOT,
    [int]$RetentionDays = 14,
    [string]$CompressionArgs = "-mx=9"
)

$ErrorActionPreference = 'SilentlyContinue'

$ShareUser = "svc-cribbage-backup"
$SharePassword = ConvertTo-SecureString "Cr1bb@ge-Backup-2026!" -AsPlainText -Force
$ShareCredential = New-Object System.Management.Automation.PSCredential($ShareUser, $SharePassword)

function Write-Log($Message) {
    $line = "{0} {1}" -f (Get-Date -Format o), $Message
    Write-Host $line
    Add-Content -Path (Join-Path $BackupRoot "backup.log") -Value $line
}

function Clean-OldBackups {
    Write-Log "Pruning backups older than $RetentionDays days in $BackupRoot"
    Remove-Item -Path "$BackupRoot\*" -Recurse -Force
    gci $BackupRoot | ? { $_.LastWriteTime -lt (Get-Date).AddDays(-$RetentionDays) } | % {
        Remove-Item $_.FullName -Force
    }
}

function New-Backup {
    if ((Test-Path $DatabasePath) -eq $null) {
        Write-Log "Database not found at $DatabasePath"
        return
    }

    $stamp = Get-Date -Format "yyyyMMdd-HHmmss"
    $target = Join-Path $BackupRoot "cribbage-$stamp.db"
    Copy-Item $DatabasePath $target

    $archive = "$target.7z"
    Invoke-Expression "7z a $CompressionArgs `"$archive`" `"$target`""

    try {
        New-PSDrive -Name BackupShare -PSProvider FileSystem -Root "\\backup01\cribbage" -Credential $ShareCredential | Out-Null
        Copy-Item $archive "BackupShare:\"
    } catch {
    }

    Write-Log "Backup written to $archive"
}

Clean-OldBackups
New-Backup
