# Applies backend/migrations/*.cql in order and records each in psbc.schema_migrations.
# Run from the project root:
#   powershell -ExecutionPolicy Bypass -File .\infrastructure\scripts\migrate.ps1
param(
  [string]$Container = "psbc-scylla",
  [string]$Keyspace = "psbc"
)
$ErrorActionPreference = "Continue"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$dir = Join-Path $root "backend\migrations"

function Invoke-Cql([string]$cql) {
  $out = docker exec $Container cqlsh -e $cql 2>&1 | Out-String
  if ($LASTEXITCODE -ne 0) { throw "cqlsh failed: $out" }
  return $out
}

Invoke-Cql "CREATE TABLE IF NOT EXISTS $Keyspace.schema_migrations (version text PRIMARY KEY, description text, applied_at timestamp);" | Out-Null

Get-ChildItem $dir -Filter *.cql | Sort-Object Name | ForEach-Object {
  $file = $_
  $version = $file.BaseName.Split("_")[0]
  $check = Invoke-Cql "SELECT version FROM $Keyspace.schema_migrations WHERE version = '$version';"
  if ($check -match "\(1 rows\)") {
    Write-Host "skip   $($file.Name) (already applied)"
    return
  }
  Write-Host "apply  $($file.Name)"
  docker cp $file.FullName "${Container}:/tmp/$($file.Name)" | Out-Null
  $out = docker exec $Container cqlsh -f "/tmp/$($file.Name)" 2>&1 | Out-String
  if ($LASTEXITCODE -ne 0) { Write-Host $out; throw "Migration $($file.Name) failed" }
  Invoke-Cql "INSERT INTO $Keyspace.schema_migrations (version, description, applied_at) VALUES ('$version', '$($file.BaseName)', toTimestamp(now()));" | Out-Null
}
Write-Host "Migrations finished."
