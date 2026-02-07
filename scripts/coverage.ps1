# coverage.ps1 - Calculate combined test coverage (unit + integration)

Write-Host "=== Combined Test Coverage ===" -ForegroundColor Cyan
Write-Host ""

# Set test database environment variables
$env:TEST_DB_HOST = 'localhost'
$env:TEST_DB_PORT = '5433'
$env:TEST_DB_NAME = 'wallethub_test'
$env:TEST_DB_USER = 'postgres'
$env:TEST_DB_PASSWORD = 'postgres'

Write-Host "Step 1: Running unit tests (without integration tag)..." -ForegroundColor Yellow
go test -coverprofile="coverage_unit.out" -covermode=atomic ./... 2>&1 | Out-Null
if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ Unit tests passed" -ForegroundColor Green
} else {
    Write-Host "✗ Unit tests failed" -ForegroundColor  Red
}

Write-Host ""
Write-Host "Step 2: Running integration tests (with integration tag)..." -ForegroundColor Yellow
go test -tags=integration -coverprofile="coverage_integration.out" -covermode=atomic ./internal/application/usecases/... 2>&1 | Out-Null
if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ Integration tests passed" -ForegroundColor Green
} else {
    Write-Host "✗ Integration tests failed (might be expected if DB not running)" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "Step 3: Combining coverage profiles..." -ForegroundColor Yellow

# Merge coverage files
$unitLines = Get-Content coverage_unit.out
$integrationLines = Get-Content coverage_integration.out | Select-Object -Skip 1  # Skip mode line

$unitLines + $integrationLines | Set-Content coverage_combined.out

Write-Host "✓ Coverage files merged" -ForegroundColor Green

Write-Host ""
Write-Host "=== Coverage Report ===" -ForegroundColor Cyan

# Generate coverage report
go tool cover -func=coverage_combined.out | Select-Object -Last 15

Write-Host ""
$totalCoverage = (go tool cover -func=coverage_combined.out | Select-String "total" | Out-String).Trim()
Write-Host $totalCoverage -ForegroundColor Green

Write-Host ""
Write-Host "Coverage files:" -ForegroundColor Cyan
Write-Host "  - coverage_unit.out (unit tests only)"
Write-Host "  - coverage_integration.out (integration tests only)"
Write-Host "  - coverage_combined.out (merged)"
Write-Host ""
Write-Host "To generate HTML report run: go tool cover -html=coverage_combined.out -o coverage.html" -ForegroundColor Yellow
