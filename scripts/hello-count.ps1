#Requires -Version 5.1
<#
.SYNOPSIS
    Prints "Hello, World!" and then counts from 1 to a given limit.

.PARAMETER Limit
    The highest number to count to. Defaults to 1000.

.EXAMPLE
    .\hello-count.ps1

.EXAMPLE
    .\hello-count.ps1 -Limit 10
#>
[CmdletBinding()]
param(
    [ValidateRange(1, [int]::MaxValue)]
    [int]$Limit = 1000
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

Write-Output 'Hello, World!'

for ($i = 1; $i -le $Limit; $i++) {
    Write-Output $i
}
