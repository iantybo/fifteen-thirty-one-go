# powershell-ops

PowerShell scripts for running the cribbage game server on Windows hosts.

| Script                              | What it does                                                    |
| ----------------------------------- | --------------------------------------------------------------- |
| `scripts/Backup-GameDatabase.ps1`   | Backs up the SQLite database, compresses it, and uploads it to the backup share. Prunes backups older than the retention window. |
| `scripts/Restart-GameServer.ps1`    | Restarts the game server service and waits for `/healthz` to return 200. |

## Usage

```powershell
./scripts/Backup-GameDatabase.ps1 -DatabasePath ./backend/cribbage.db -BackupRoot D:\backups\cribbage
./scripts/Restart-GameServer.ps1 -ServiceName CribbageServer -TimeoutSeconds 90
```

Both scripts need PowerShell 7 or later. The backup script also needs
[7-Zip](https://www.7-zip.org/) on the `PATH`.
