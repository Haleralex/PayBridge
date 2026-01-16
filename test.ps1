# PowerShell test runner for PayBridge
# Alternative to Makefile for Windows

param(
    [Parameter(Position=0)]
    [string]$Command = "help"
)

function Show-Help {
    Write-Host ""
    Write-Host "PayBridge Test Runner" -ForegroundColor Green
    Write-Host "=====================" -ForegroundColor Green
    Write-Host ""
    Write-Host "Usage: .\test.ps1 <command>" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Commands:" -ForegroundColor Yellow
    Write-Host "  test              - Run all tests"
    Write-Host "  test-unit         - Run unit tests (fast)"
    Write-Host "  test-integration  - Run integration tests (requires DB)"
    Write-Host "  test-coverage     - Run tests with coverage report"
    Write-Host "  test-coverage-func- Show coverage by function"
    Write-Host "  test-bench        - Run benchmarks"
    Write-Host "  test-verbose      - Run tests with verbose output"
    Write-Host "  test-ci           - Run tests for CI"
    Write-Host ""
    Write-Host "Examples:" -ForegroundColor Yellow
    Write-Host "  .\test.ps1 test"
    Write-Host "  .\test.ps1 test-unit"
    Write-Host "  .\test.ps1 test-coverage"
    Write-Host ""
}

function Test-All {
    Write-Host "Running all tests..." -ForegroundColor Cyan
    go test -v -race -cover ./...
}

function Test-Unit {
    Write-Host "Running unit tests (domain + application)..." -ForegroundColor Cyan
    go test -v -short -race ./internal/domain/... ./internal/application/...
}

function Test-Integration {
    Write-Host "Running integration tests (requires PostgreSQL)..." -ForegroundColor Cyan
    if (-not $env:DATABASE_URL) {
        Write-Host "DATABASE_URL not set, using default" -ForegroundColor Yellow
        $env:DATABASE_URL = "postgres://postgres:postgres@localhost:5432/paybridge?sslmode=disable"
    }
    go test -v -race ./internal/infrastructure/...
}

function Test-Coverage {
    Write-Host "Generating coverage report..." -ForegroundColor Cyan
    $coverFile = "coverage.txt"
    go test -v -race "-coverprofile=$coverFile" ./...
    
    if ($LASTEXITCODE -eq 0) {
        go tool cover "-html=$coverFile" -o coverage.html
        Write-Host "Coverage report created: coverage.html" -ForegroundColor Green
        
        # Open in browser
        Start-Process coverage.html
    } else {
        Write-Host "Tests failed" -ForegroundColor Red
        exit 1
    }
}

function Test-Coverage-Func {
    Write-Host "Coverage by function..." -ForegroundColor Cyan
    $coverFile = "coverage.txt"
    go test "-coverprofile=$coverFile" ./...
    if ($LASTEXITCODE -eq 0) {
        go tool cover "-func=$coverFile"
    }
}

function Test-Bench {
    Write-Host "Running benchmarks..." -ForegroundColor Cyan
    go test -bench=. -benchmem ./...
}

function Test-Verbose {
    $coverFile = "coverage.txt"
    go test -v -race -cover "-coverprofile=$coverFile" ./...
    if ($LASTEXITCODE -eq 0) {
        go tool cover "-func=$coverFile"
        go tool cover -func=coverage.out
    }
}

function Test-CI {
    $coverFile = "coverage.txt"
    go test -v -race "-coverprofile=$coverFile" -covermode=atomic ./...
    if ($LASTEXITCODE -eq 0) {
        go tool cover "-func=$coverFile"
        
        # Check minimum coverage
        $coverage = go tool cover "-func=$coverFile"
        $coverage = go tool cover -func=coverage.out | Select-String "total:" | ForEach-Object { $_.ToString() -replace '.*\s+(\d+\.\d+)%.*', '$1' }
        Write-Host "Total coverage: $coverage%" -ForegroundColor Cyan
        
        if ([double]$coverage -lt 70) {
            Write-Host "Coverage below 70%!" -ForegroundColor Red
            exit 1
        } else {
            Write-Host "Coverage OK: $coverage%" -ForegroundColor Green
        }
    } else {
        Write-Host "Tests failed" -ForegroundColor Red
        exit 1
    }
}

# Main switch
switch ($Command.ToLower()) {
    "test" { Test-All }
    "test-unit" { Test-Unit }
    "test-integration" { Test-Integration }
    "test-coverage" { Test-Coverage }
    "test-coverage-func" { Test-Coverage-Func }
    "test-bench" { Test-Bench }
    "test-verbose" { Test-Verbose }
    "test-ci" { Test-CI }
    "help" { Show-Help }
    default {
        Write-Host "Unknown command: $Command" -ForegroundColor Red
        Write-Host ""
        Show-Help
        exit 1
    }
}
