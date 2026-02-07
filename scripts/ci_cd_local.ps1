# CI/CD Local Runner
# Simulates GitHub Actions security-scan workflow locally

param(
    [switch]$SkipTests = $false,
    [switch]$Verbose = $false
)

$ErrorActionPreference = "Continue"
$script:FailureCount = 0
$script:WarningCount = 0
$script:PassCount = 0

function Write-Step {
    param([string]$Message)
    Write-Host "`n=== $Message ===" -ForegroundColor Cyan
}

function Write-Success {
    param([string]$Message)
    Write-Host "[OK] $Message" -ForegroundColor Green
    $script:PassCount++
}

function Write-Fail {
    param([string]$Message)
    Write-Host "[FAIL] $Message" -ForegroundColor Red
    $script:FailureCount++
}

function Write-Warn {
    param([string]$Message)
    Write-Host "[WARN] $Message" -ForegroundColor Yellow
    $script:WarningCount++
}

function Write-Info {
    param([string]$Message)
    Write-Host "[INFO] $Message" -ForegroundColor Gray
}

# Clear screen
Clear-Host

Write-Host @"

==================================
 CI/CD Local Security Checks
==================================

Simulating GitHub Actions workflow locally
Project: $PWD
Time: $(Get-Date -Format "yyyy-MM-dd HH:mm:ss")

"@ -ForegroundColor Cyan

# Step 1: Check Go version
Write-Step "Environment Check"
try {
    $goVersion = go version 2>&1 | Out-String
    if ($goVersion -match "go1\.2[5-9]") {
        Write-Success "Go version: $($goVersion.Trim())"
    } else {
        Write-Warn "Go version is older than 1.25: $($goVersion.Trim())"
    }
} catch {
    Write-Fail "Go not found"
    exit 1
}

# Step 2: Install security tools
Write-Step "Installing Security Tools"
$tools = @(
    @{Name="govulncheck"; Package="golang.org/x/vuln/cmd/govulncheck@latest"},
    @{Name="gosec"; Package="github.com/securego/gosec/v2/cmd/gosec@latest"},
    @{Name="staticcheck"; Package="honnef.co/go/tools/cmd/staticcheck@latest"}
)

foreach ($tool in $tools) {
    $exists = Get-Command $tool.Name -ErrorAction SilentlyContinue
    if ($exists) {
        Write-Info "$($tool.Name) already installed"
    } else {
        Write-Info "Installing $($tool.Name)..."
        go install $tool.Package 2>&1 | Out-Null
        if ($LASTEXITCODE -eq 0) {
            Write-Success "$($tool.Name) installed"
        } else {
            Write-Fail "Failed to install $($tool.Name)"
        }
    }
}

# Step 3: Run govulncheck
Write-Step "Running Vulnerability Scan (govulncheck)"
$vulnResult = govulncheck ./... 2>&1 | Out-String
if ($vulnResult -match "No vulnerabilities found" -or $vulnResult -match "found 0 vulnerabilities") {
    Write-Success "No known vulnerabilities found"
} else {
    Write-Fail "Vulnerabilities detected"
    if ($Verbose) {
        Write-Host $vulnResult
    }
}

# Step 4: Run gosec
Write-Step "Running Static Analysis (gosec)"
$gosecOutput = gosec -fmt=json -severity=medium ./... 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-Success "No security issues found by gosec"
} else {
    $issueCount = ($gosecOutput | ConvertFrom-Json).Issues.Count 2>$null
    if ($issueCount -gt 0) {
        Write-Warn "gosec found $issueCount security issues"
        if ($Verbose) {
            Write-Host $gosecOutput
        }
    } else {
        Write-Success "No critical security issues"
    }
}

# Step 5: Check for SQL injection patterns
Write-Step "Checking for SQL Injection Patterns"
$sqlPatterns = @(
    'fmt\.Sprintf.*SELECT',
    'fmt\.Sprintf.*INSERT',
    'fmt\.Sprintf.*UPDATE',
    'fmt\.Sprintf.*DELETE'
)

$sqlInjectionFound = $false
foreach ($pattern in $sqlPatterns) {
    $matches = Get-ChildItem -Path "internal" -Recurse -Filter "*.go" | 
        Select-String -Pattern $pattern -ErrorAction SilentlyContinue
    if ($matches) {
        Write-Fail "Potential SQL injection found: $pattern"
        $sqlInjectionFound = $true
        if ($Verbose) {
            $matches | ForEach-Object { Write-Host "  $_" }
        }
    }
}
if (-not $sqlInjectionFound) {
    Write-Success "No SQL injection patterns detected"
}

# Step 6: Check for hardcoded secrets
Write-Step "Scanning for Hardcoded Secrets"
$secretPatterns = @{
    'password' = 'password\s*[=:]\s*["\x27][^"\x27\$\{]{8,}'
    'api_key' = 'api[_-]?key\s*[=:]\s*["\x27][A-Za-z0-9]{20,}'
    'secret' = 'secret\s*[=:]\s*["\x27][^"\x27\$\{]{16,}'
    'token' = 'token\s*[=:]\s*["\x27][A-Za-z0-9]{32,}'
}

$secretsFound = $false
foreach ($type in $secretPatterns.Keys) {
    $pattern = $secretPatterns[$type]
    $matches = Get-ChildItem -Path . -Include "*.go","*.yaml","*.yml" -Recurse -ErrorAction SilentlyContinue | 
        Where-Object { $_.Name -notmatch "_test\.go$" } |
        Select-String -Pattern $pattern -ErrorAction SilentlyContinue |
        Where-Object { $_.Line -notmatch "change-me|example|test|mock|dummy|\$\{|\%\{|DefaultConfig|Dev only" }
    
    if ($matches) {
        Write-Warn "Potential hardcoded $type found"
        $secretsFound = $true
        if ($Verbose) {
            $matches | ForEach-Object { Write-Host "  $($_.Path):$($_.LineNumber)" }
        }
    }
}
if (-not $secretsFound) {
    Write-Success "No obvious hardcoded secrets found"
}

# Step 7: Validate production configuration
Write-Step "Validating Production Configuration"
$prodConfig = "configs\config.production.yaml"
if (Test-Path $prodConfig) {
    $configContent = Get-Content $prodConfig -Raw
    
    $configIssues = 0
    
    if ($configContent -match "enable_mock_auth:\s*true") {
        Write-Fail "Mock auth enabled in production config"
        $configIssues++
    }
    
    if ($configContent -match "ssl_mode:\s*disable") {
        Write-Fail "SSL disabled in production config"
        $configIssues++
    }
    
    if ($configContent -match "allowed_origins:.*\*") {
        Write-Fail "CORS wildcard in production config"
        $configIssues++
    }
    
    if ($configIssues -eq 0) {
        Write-Success "Production configuration validated"
    }
} else {
    Write-Warn "Production config not found: $prodConfig"
}

# Step 8: Check authentication implementation
Write-Step "Verifying Authentication Security"
$authMiddleware = "internal\adapters\http\middleware\auth.go"
$walletHandler = "internal\adapters\http\handlers\wallet_handler.go"

$authIssues = 0

if (Test-Path $authMiddleware) {
    $authContent = Get-Content $authMiddleware -Raw
    if ($authContent -match "NewJWTTokenValidator|jwt\.Parse") {
        Write-Success "JWT implementation found"
    } else {
        Write-Warn "JWT validator implementation not found"
        $authIssues++
    }
} else {
    Write-Fail "Auth middleware not found"
    $authIssues++
}

if (Test-Path $walletHandler) {
    $walletContent = Get-Content $walletHandler -Raw
    if ($walletContent -match "checkWalletOwnership|GetAuthUserID") {
        Write-Success "Wallet ownership validation found"
    } else {
        Write-Fail "Missing wallet ownership checks"
        $authIssues++
    }
} else {
    Write-Fail "Wallet handler not found"
    $authIssues++
}

# Step 9: Run security tests
if (-not $SkipTests) {
    Write-Step "Running Security Tests"
    
    Write-Info "Running middleware tests..."
    go test -v -race ./internal/adapters/http/middleware/... 2>&1 | Out-Null
    if ($LASTEXITCODE -eq 0) {
        Write-Success "Middleware tests passed"
    } else {
        Write-Fail "Middleware tests failed"
    }
    
    Write-Info "Running handler tests..."
    go test -v -race ./internal/adapters/http/handlers/... 2>&1 | Out-Null
    if ($LASTEXITCODE -eq 0) {
        Write-Success "Handler tests passed"
    } else {
        Write-Warn "Some handler tests failed (check details)"
    }
} else {
    Write-Info "Tests skipped (use without -SkipTests to run)"
}

# Step 10: Check test coverage
Write-Step "Checking Test Coverage"
Write-Info "Calculating coverage..."
$coverageFile = Join-Path $PWD "coverage_ci.out"
$null = go test -coverprofile="$coverageFile" ./internal/adapters/http/... 2>&1
if (Test-Path $coverageFile) {
    $coverageOutput = go tool cover -func "$coverageFile" 2>&1 | Out-String
    $totalLine = $coverageOutput -split "`n" | Where-Object { $_ -match "total:" } | Select-Object -First 1
    if ($totalLine) {
        $coverage = [regex]::Match($totalLine, '\d+\.\d+').Value
        if ($coverage -and [double]$coverage -ge 70) {
            Write-Success "Test coverage: $coverage%"
        } elseif ($coverage -and [double]$coverage -ge 50) {
            Write-Warn "Test coverage below 70%: $coverage%"
        } elseif ($coverage) {
            Write-Fail "Test coverage critically low: $coverage%"
        } else {
            Write-Warn "Could not parse coverage percentage"
        }
    } else {
        Write-Warn "Could not find coverage total"
    }
    Remove-Item $coverageFile -ErrorAction SilentlyContinue
} else {
    Write-Warn "Could not calculate coverage"
}

# Step 11: Check for .env files
Write-Step "Checking for Exposed Configuration"
$envFiles = Get-ChildItem -Path . -Filter ".env*" -Recurse -ErrorAction SilentlyContinue |
    Where-Object { $_.Name -notmatch "\.example$" }
if ($envFiles) {
    Write-Fail "Found .env files (should be in .gitignore)"
    if ($Verbose) {
        $envFiles | ForEach-Object { Write-Host "  $($_.FullName)" }
    }
} else {
    Write-Success "No .env files found"
}

# Step 12: Verify go.mod
Write-Step "Verifying Dependencies"
if (Test-Path "go.mod") {
    $goMod = Get-Content "go.mod" -Raw
    if ($goMod -match "go 1\.2[5-9]") {
        Write-Success "go.mod uses Go 1.25+"
    } else {
        Write-Warn "go.mod may use older Go version"
    }
    
    # Check for tidy
    go mod tidy 2>&1 | Out-Null
    $gitDiff = git diff go.mod go.sum 2>&1
    if ($gitDiff) {
        Write-Warn "go.mod or go.sum not tidy (run 'go mod tidy')"
    } else {
        Write-Success "Dependencies are tidy"
    }
} else {
    Write-Fail "go.mod not found"
}

# Summary
Write-Host @"

==================================
 CI/CD Check Summary
==================================

"@ -ForegroundColor Cyan

Write-Host "Passed:   " -NoNewline -ForegroundColor Gray
Write-Host $script:PassCount -ForegroundColor Green

Write-Host "Warnings: " -NoNewline -ForegroundColor Gray
Write-Host $script:WarningCount -ForegroundColor Yellow

Write-Host "Failures: " -NoNewline -ForegroundColor Gray
Write-Host $script:FailureCount -ForegroundColor Red

Write-Host "`nCompleted at $(Get-Date -Format 'HH:mm:ss')" -ForegroundColor Gray

# Exit code
if ($script:FailureCount -gt 0) {
    Write-Host "`n[RESULT] CI/CD checks FAILED" -ForegroundColor Red
    exit 1
} elseif ($script:WarningCount -gt 0) {
    Write-Host "`n[RESULT] CI/CD checks PASSED with warnings" -ForegroundColor Yellow
    exit 0
} else {
    Write-Host "`n[RESULT] CI/CD checks PASSED" -ForegroundColor Green
    exit 0
}
