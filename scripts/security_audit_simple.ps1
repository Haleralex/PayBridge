# PayBridge Security Audit - Simplified Version
param(
    [switch]$Quick
)

$ErrorActionPreference = "Continue"
$ProjectRoot = (Get-Location).Path
$timestamp = Get-Date -Format "yyyyMMdd-HHmmss"

# Counters
$CriticalCount = 0
$HighCount = 0
$MediumCount = 0
$LowCount = 0

Write-Host ""
Write-Host "==================================" -ForegroundColor Cyan
Write-Host " PayBridge Security Audit        " -ForegroundColor Cyan
Write-Host "==================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Project: $ProjectRoot" -ForegroundColor Gray
Write-Host "Time: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')" -ForegroundColor Gray
Write-Host ""

# 1. Environment Check
Write-Host "--- Environment Check ---" -ForegroundColor Yellow
Write-Host ""

$goVer = go version 2>&1
Write-Host "[INFO] Go version: $goVer" -ForegroundColor Cyan

# 2. Config Security
Write-Host ""
Write-Host "--- Configuration Security ---" -ForegroundColor Yellow
Write-Host ""

$prodConfig = "configs\config.production.yaml"
if (Test-Path $prodConfig) {
    $prodContent = Get-Content $prodConfig -Raw
    
    if ($prodContent -match "enable_mock_auth:\s*true") {
        Write-Host "[CRITICAL] Mock auth enabled in production!" -ForegroundColor Red
        $CriticalCount++
    } else {
        Write-Host "[OK] Mock auth disabled" -ForegroundColor Green
    }
    
    if ($prodContent -match "ssl_mode:\s*disable") {
        Write-Host "[HIGH] SSL disabled in production config" -ForegroundColor Red
        $HighCount++
    } else {
        Write-Host "[OK] SSL configured" -ForegroundColor Green
    }
    
    if ($prodContent -match "allowed_origins:[\s\S]*\*") {
        Write-Host "[HIGH] CORS wildcard in production" -ForegroundColor Red
        $HighCount++
    } else {
        Write-Host "[OK] CORS properly configured" -ForegroundColor Green
    }
} else {
    Write-Host "[WARN] Production config not found" -ForegroundColor Yellow
}

# 3. Code Security
Write-Host ""
Write-Host "--- Code Security ---" -ForegroundColor Yellow
Write-Host ""

Write-Host "[INFO] Scanning for SQL injection patterns..." -ForegroundColor Cyan
$sqlIssues = 0
$files = Get-ChildItem -Path "internal" -Filter "*.go" -Recurse -ErrorAction SilentlyContinue
foreach ($file in $files) {
    $lines = Get-Content $file.FullName
    $lineNum = 0
    foreach ($line in $lines) {
        $lineNum++
        if (($line -match "fmt\.Sprintf") -and ($line -match "SELECT|INSERT|UPDATE|DELETE")) {
            Write-Host "[CRITICAL] Potential SQL injection: $($file.Name):$lineNum" -ForegroundColor Red
            $sqlIssues++
            $CriticalCount++
        }
    }
}

if ($sqlIssues -eq 0) {
    Write-Host "[OK] No SQL injection patterns found" -ForegroundColor Green
}

# 4. Secrets Check
Write-Host ""
Write-Host "[INFO] Checking for .env files..." -ForegroundColor Cyan
$envFiles = Get-ChildItem -Path $ProjectRoot -Filter ".env" -ErrorAction SilentlyContinue
if ($envFiles) {
    Write-Host "[CRITICAL] Found .env file in repository!" -ForegroundColor Red
    $CriticalCount++
} else {
    Write-Host "[OK] No .env files" -ForegroundColor Green
}

# 5. Dependencies
Write-Host ""
Write-Host "--- Dependencies ---" -ForegroundColor Yellow
Write-Host ""

if (Get-Command "govulncheck" -ErrorAction SilentlyContinue) {
    Write-Host "[INFO] Running vulnerability scan..." -ForegroundColor Cyan
    $vulnOutput = govulncheck ./... 2>&1 | Out-String
    if ($vulnOutput -match "No vulnerabilities") {
        Write-Host "[OK] No known vulnerabilities" -ForegroundColor Green
    } else {
        Write-Host "[HIGH] Vulnerabilities found" -ForegroundColor Red
        $HighCount++
    }
} else {
    Write-Host "[WARN] govulncheck not installed" -ForegroundColor Yellow
    $LowCount++
}

# 6. Tests
Write-Host ""
Write-Host "--- Security Tests ---" -ForegroundColor Yellow
Write-Host ""

Write-Host "[INFO] Running quick tests..." -ForegroundColor Cyan
$testResult = go test -short ./internal/adapters/http/middleware 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-Host "[OK] Middleware tests passed" -ForegroundColor Green
} else {
    Write-Host "[WARN] Some tests failed" -ForegroundColor Yellow
    $LowCount++
}

# Summary
Write-Host ""
Write-Host "==================================" -ForegroundColor Cyan
Write-Host " Security Summary                " -ForegroundColor Cyan
Write-Host "==================================" -ForegroundColor Cyan
Write-Host ""

$total = $CriticalCount + $HighCount + $MediumCount + $LowCount

Write-Host "Issue Breakdown:" -ForegroundColor White
Write-Host "  CRITICAL: $CriticalCount" -ForegroundColor $(if ($CriticalCount -gt 0) {"Red"} else {"Green"})
Write-Host "  HIGH:     $HighCount" -ForegroundColor $(if ($HighCount -gt 0) {"Red"} else {"Green"})
Write-Host "  MEDIUM:   $MediumCount" -ForegroundColor $(if ($MediumCount -gt 0) {"Yellow"} else {"Green"})
Write-Host "  LOW:      $LowCount" -ForegroundColor $(if ($LowCount -gt 0) {"Yellow"} else {"Green"})
Write-Host "  ----------------------"
Write-Host "  TOTAL:    $total"
Write-Host ""

# Calculate score
$deductions = ($CriticalCount * 20) + ($HighCount * 10) + ($MediumCount * 5) + ($LowCount * 2)
$score = [Math]::Max(0, 100 - $deductions)

$scoreColor = "Green"
if ($score -lt 60) { $scoreColor = "Red" }
elseif ($score -lt 80) { $scoreColor = "Yellow" }

Write-Host "Security Score: $score / 100" -ForegroundColor $scoreColor
Write-Host ""

if ($CriticalCount -gt 0) {
    Write-Host "*** CRITICAL ISSUES FOUND - DO NOT DEPLOY ***" -ForegroundColor Red -BackgroundColor Black
    Write-Host ""
}

# Recommendations
if ($total -gt 0) {
    Write-Host "Recommendations:" -ForegroundColor Cyan
    if ($CriticalCount -gt 0) {
        Write-Host "  1. Fix CRITICAL issues immediately" -ForegroundColor Red
    }
    if ($HighCount -gt 0) {
        Write-Host "  2. Address HIGH issues before deployment" -ForegroundColor Yellow
    }
    Write-Host ""
}

Write-Host "Audit completed at $(Get-Date -Format 'HH:mm:ss')" -ForegroundColor Gray
Write-Host ""

# Exit code
if ($CriticalCount -gt 0) {
    exit 2
} elseif ($HighCount -gt 0) {
    exit 1
} else {
    exit 0
}
