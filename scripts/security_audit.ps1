# PayBridge Security Audit Script
# Runs comprehensive security checks and generates a report

param(
    [switch]$Detailed,
    [switch]$FixIssues,
    [string]$OutputFormat = "console" # console, json, html
)

$ErrorActionPreference = "Continue"
$ScriptDir = $PSScriptRoot
$ProjectRoot = Split-Path $ScriptDir -Parent
$ReportFile = Join-Path $ProjectRoot "security-report-$(Get-Date -Format 'yyyyMMdd-HHmmss').txt"

# Colors
$Green = "Green"
$Red = "Red"
$Yellow = "Yellow"
$Cyan = "Cyan"

# Results tracking
$TotalIssues = 0
$CriticalIssues = 0
$HighIssues = 0
$MediumIssues = 0
$LowIssues = 0

function Write-Header {
    param([string]$Text)
    Write-Host ""
    Write-Host "=" -NoNewline -ForegroundColor Cyan
    Write-Host " $Text " -NoNewline -ForegroundColor White
    Write-Host "=" -ForegroundColor Cyan
    Write-Host ""
}

function Write-Success {
    param([string]$Text)
    Write-Host "[OK] " -NoNewline -ForegroundColor Green
    Write-Host $Text
}

function Write-Warning {
    param([string]$Text)
    Write-Host "[WARN] " -NoNewline -ForegroundColor Yellow
    Write-Host $Text
}

function Write-ErrorMsg {
    param([string]$Text)
    Write-Host "[ERROR] " -NoNewline -ForegroundColor Red
    Write-Host $Text
}

function Write-Info {
    param([string]$Text)
    Write-Host "[INFO] " -NoNewline -ForegroundColor Cyan
    Write-Host $Text
}

function Test-CommandExists {
    param([string]$Command)
    return $null -ne (Get-Command $Command -ErrorAction SilentlyContinue)
}

Write-Host @"
╔═══════════════════════════════════════════════════════════════╗
║                 PayBridge Security Audit Tool                 ║
║                        Version 1.0.0                          ║
╚═══════════════════════════════════════════════════════════════╝
"@ -ForegroundColor Cyan

Write-Info "Starting security audit at $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')"
Write-Info "Project root: $ProjectRoot"
Write-Host ""

# ============================================
# 1. Environment & Configuration Checks
# ============================================
Write-Header "1. Environment & Configuration Security"

# Check Go version
Write-Info "Checking Go version..."
$goVersion = go version
if ($goVersion -match "go(\d+\.\d+)") {
    $versionNum = $matches[1]
    if ([double]$versionNum -lt 1.21) {
        Write-ErrorMsg "Go version too old: $goVersion (require 1.21+)"
        $HighIssues++
    } else {
        Write-Success "Go version OK: $goVersion"
    }
}

# Check for .env files in git
Write-Info "Checking for exposed .env files..."
$envFiles = Get-ChildItem -Path $ProjectRoot -Filter ".env" -Recurse -ErrorAction SilentlyContinue
if ($envFiles) {
    Write-ErrorMsg "Found .env files (should not be committed):"
    foreach ($file in $envFiles) {
        Write-Host "  - $($file.FullName)" -ForegroundColor Red
        $CriticalIssues++
    }
} else {
    Write-Success "No .env files in repository"
}

# Check for hardcoded secrets patterns
Write-Info "Scanning for hardcoded secrets..."
$foundSecrets = 0

Get-ChildItem -Path $ProjectRoot -Include "*.go","*.yaml","*.yml" -Recurse -ErrorAction SilentlyContinue | ForEach-Object {
    $content = Get-Content $_.FullName -Raw -ErrorAction SilentlyContinue
    if ($content) {
        # Simple pattern checks (avoiding complex regex)
        if ($content -match 'password\s*=\s*"[^"]{10,}"' -and $content -notmatch 'example|test|change-me|dummy') {
            Write-Warning "Potential password in: $($_.Name)"
            $foundSecrets++
            $HighIssues++
        }
    }
}

if ($foundSecrets -eq 0) {
    Write-Success "No obvious hardcoded secrets found"
} else {
    Write-Warning "Found $foundSecrets potential secret(s)"
}

# Check production config
Write-Info "Validating production configuration..."
$prodConfig = Join-Path $ProjectRoot "configs\config.production.yaml"
if (Test-Path $prodConfig) {
    $content = Get-Content $prodConfig -Raw
    
    if ($content -match 'enable_mock_auth:\s*true') {
        Write-ErrorMsg "Production config has mock_auth enabled!"
        $CriticalIssues++
    } else {
        Write-Success "Mock auth disabled in production config"
    }
    
    if ($content -match 'ssl_mode:\s*disable') {
        Write-ErrorMsg "Production config has SSL disabled!"
        $HighIssues++
    } else {
        Write-Success "SSL configured in production config"
    }
    
    if ($content -match 'allowed_origins:.*\*') {
        Write-ErrorMsg "Production config has CORS wildcard (*)"
        $HighIssues++
    } else {
        Write-Success "CORS properly restricted in production config"
    }
} else {
    Write-Warning "Production config not found: $prodConfig"
    $MediumIssues++
}

# ============================================
# 2. Code Security Analysis
# ============================================
Write-Header "2. Code Security Analysis"

# Check for SQL injection vulnerabilities
Write-Info "Scanning for SQL injection vulnerabilities..."
$sqlInjectionCount = 0
Get-ChildItem -Path (Join-Path $ProjectRoot "internal") -Filter "*.go" -Recurse | ForEach-Object {
    $content = Get-Content $_.FullName
    $lineNum = 0
    foreach ($line in $content) {
        $lineNum++
        # Look for string concatenation in SQL queries
        if ($line -match '(Query|Exec).*fmt\.Sprintf' -or 
            $line -match 'query.*\+' -or
            $line -match '"SELECT.*%s' -or
            $line -match '"INSERT.*%s') {
            Write-ErrorMsg "Potential SQL injection at $($_.Name):$lineNum"
            Write-Host "  $line" -ForegroundColor Red
            $sqlInjectionCount++
            $CriticalIssues++
        }
    }
}

if ($sqlInjectionCount -eq 0) {
    Write-Success "No SQL injection patterns detected"
} else {
    Write-ErrorMsg "Found $sqlInjectionCount potential SQL injection vulnerabilities"
}

# Check for auth bypass patterns
Write-Info "Checking for authentication bypass patterns..."
$bypassCount = 0
Get-ChildItem -Path (Join-Path $ProjectRoot "internal\adapters\http") -Filter "*handler*.go" -Recurse | ForEach-Object {
    $content = Get-Content $_.FullName -Raw
    
    # Check if sensitive operations skip auth
    if ($content -match 'SkipPaths' -and $content -match 'wallets|transactions') {
        Write-ErrorMsg "Financial endpoints in auth skip list: $($_.Name)"
        $bypassCount++
        $CriticalIssues++
    }
    
    # Check for missing ownership validation
    if ($content -match 'func.*Handler.*Credit|Debit|Transfer' -and
        $content -notmatch 'GetAuthUserID' -and
        $content -notmatch 'checkWalletOwnership') {
        Write-Warning "Potential missing ownership check in $($_.Name)"
        $bypassCount++
        $MediumIssues++
    }
}

if ($bypassCount -eq 0) {
    Write-Success "No authentication bypass patterns found"
}

# Check for XSS vulnerabilities in frontend
Write-Info "Checking frontend for XSS vulnerabilities..."
$xssCount = 0
$webappIndex = Join-Path $ProjectRoot "webapp\index.html"
if (Test-Path $webappIndex) {
    $content = Get-Content $webappIndex -Raw
    if ($content -match '\.innerHTML\s*=') {
        Write-Warning "Found .innerHTML usage (potential XSS)"
        $xssCount++
        $MediumIssues++
    }
}

if ($xssCount -eq 0) {
    Write-Success "No obvious XSS patterns in frontend"
}

# ============================================
# 3. Dependency Security
# ============================================
Write-Header "3. Dependency Vulnerability Scan"

Write-Info "Checking for Go module vulnerabilities..."
if (Test-CommandExists "govulncheck") {
    Push-Location $ProjectRoot
    $vulnOutput = govulncheck ./... 2>&1
    Pop-Location
    
    if ($LASTEXITCODE -eq 0 -or $vulnOutput -match "No vulnerabilities found") {
        Write-Success "No known vulnerabilities in dependencies"
    } else {
        Write-ErrorMsg "Found vulnerabilities in dependencies:"
        Write-Host $vulnOutput -ForegroundColor Red
        $HighIssues++
    }
} else {
    Write-Warning "govulncheck not installed. Install with: go install golang.org/x/vuln/cmd/govulncheck@latest"
    $LowIssues++
}

# Check for outdated dependencies
Write-Info "Checking for outdated dependencies..."
Push-Location $ProjectRoot
$outdated = go list -u -m -json all 2>&1 | ConvertFrom-Json -ErrorAction SilentlyContinue
Pop-Location

# ============================================
# 4. Test Coverage for Security
# ============================================
Write-Header "4. Security Test Coverage"

Write-Info "Running security tests..."
Push-Location $ProjectRoot
$testOutput = go test -cover ./internal/adapters/http/middleware ./internal/adapters/http/handlers 2>&1
Pop-Location

$coverageMatch = ($testOutput | Select-String -Pattern "coverage:\s*(\d+\.\d+)%").Matches
if ($coverageMatch) {
    $coverage = [double]$coverageMatch[0].Groups[1].Value
    if ($coverage -lt 70) {
        Write-Warning "Test coverage below 70%: $coverage%"
        $MediumIssues++
    } elseif ($coverage -lt 80) {
        Write-Warning "Test coverage below 80%: $coverage%"
        $LowIssues++
    } else {
        Write-Success "Test coverage OK: $coverage%"
    }
}

# ============================================
# 5. Docker Security
# ============================================
Write-Header "5. Docker Security"

$dockerfile = Join-Path $ProjectRoot "Dockerfile"
if (Test-Path $dockerfile) {
    $dockerContent = Get-Content $dockerfile -Raw
    
    # Check for non-root user
    if ($dockerContent -match 'USER\s+(?!root)') {
        Write-Success "Docker container runs as non-root user"
    } else {
        Write-Warning "Docker container may run as root"
        $MediumIssues++
    }
    
    # Check for pinned versions
    if ($dockerContent -match 'FROM.*:latest') {
        Write-Warning "Dockerfile uses :latest tag (not pinned)"
        $LowIssues++
    } else {
        Write-Success "Docker image versions are pinned"
    }
}

# Check docker-compose for secrets
Write-Info "Checking docker-compose for security issues..."
$dockerCompose = Join-Path $ProjectRoot "docker-compose.yml"
if (Test-Path $dockerCompose) {
    $composeContent = Get-Content $dockerCompose -Raw
    
    if ($composeContent -match 'POSTGRES_PASSWORD:\s*postgres') {
        Write-Warning "Default PostgreSQL password in docker-compose"
        $MediumIssues++
    }
}

# ============================================
# 6. Final Report
# ============================================
Write-Header "Security Audit Summary"

$TotalIssues = $CriticalIssues + $HighIssues + $MediumIssues + $LowIssues

Write-Host ""
Write-Host "Issue Severity Breakdown:" -ForegroundColor Cyan
Write-Host "  CRITICAL: $CriticalIssues" -ForegroundColor $(if ($CriticalIssues -gt 0) { "Red" } else { "Green" })
Write-Host "  HIGH:     $HighIssues" -ForegroundColor $(if ($HighIssues -gt 0) { "Red" } else { "Green" })
Write-Host "  MEDIUM:   $MediumIssues" -ForegroundColor $(if ($MediumIssues -gt 0) { "Yellow" } else { "Green" })
Write-Host "  LOW:      $LowIssues" -ForegroundColor $(if ($LowIssues -gt 0) { "Yellow" } else { "Green" })
Write-Host "  ──────────────────────" -ForegroundColor Gray
Write-Host "  TOTAL:    $TotalIssues" -ForegroundColor $(if ($TotalIssues -gt 0) { "Yellow" } else { "Green" })
Write-Host ""

# Calculate security score
$MaxScore = 100
$Deductions = ($CriticalIssues * 20) + ($HighIssues * 10) + ($MediumIssues * 5) + ($LowIssues * 2)
$SecurityScore = [Math]::Max(0, $MaxScore - $Deductions)

Write-Host "Security Score: $SecurityScore / $MaxScore" -ForegroundColor $(
    if ($SecurityScore -ge 80) { "Green" }
    elseif ($SecurityScore -ge 60) { "Yellow" }
    else { "Red" }
)
Write-Host ""

# Recommendations
if ($CriticalIssues -gt 0) {
    Write-Host "⚠️  CRITICAL ISSUES FOUND - DO NOT DEPLOY TO PRODUCTION" -ForegroundColor Red -BackgroundColor Black
    Write-Host ""
}

if ($TotalIssues -gt 0) {
    Write-Host "Recommendations:" -ForegroundColor Cyan
    if ($CriticalIssues -gt 0) {
        Write-Host "  1. Fix all CRITICAL issues immediately" -ForegroundColor Red
    }
    if ($HighIssues -gt 0) {
        Write-Host "  2. Address HIGH severity issues before deployment" -ForegroundColor Yellow
    }
    if ($MediumIssues -gt 0) {
        Write-Host "  3. Plan to fix MEDIUM issues in next sprint" -ForegroundColor Yellow
    }
    if ($LowIssues -gt 0) {
        Write-Host "  4. Address LOW issues when convenient" -ForegroundColor Gray
    }
} else {
    Write-Host "✓ No security issues found! Good job! 🎉" -ForegroundColor Green
}

Write-Host ""
Write-Info "Audit completed at $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')"

# Save report
$reportContent = @"
PayBridge Security Audit Report
Generated: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')

SUMMARY
=======
Critical Issues: $CriticalIssues
High Issues:     $HighIssues
Medium Issues:   $MediumIssues
Low Issues:      $LowIssues
Total Issues:    $TotalIssues

Security Score:  $SecurityScore / $MaxScore

"@

$reportContent | Out-File -FilePath $ReportFile -Encoding UTF8
Write-Info "Report saved to: $ReportFile"

# Exit with appropriate code
if ($CriticalIssues -gt 0) {
    exit 2  # Critical issues
} elseif ($HighIssues -gt 0) {
    exit 1  # High issues
} else {
    exit 0  # Success
}
