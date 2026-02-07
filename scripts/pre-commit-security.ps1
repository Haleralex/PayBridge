#!/usr/bin/env pwsh
# PayBridge Pre-commit Security Hook
# Add this to .git/hooks/pre-commit and make executable

$ErrorActionPreference = "Stop"

function Write-Message {
    param([string]$Text, [string]$Color = "White")
    Write-Host $Text -ForegroundColor $Color
}

Write-Message "🔒 Running pre-commit security checks..." "Cyan"
Write-Host ""

$IssuesFound = $false

# Check 1: Scan for secrets
Write-Message "1. Checking for exposed secrets..." "Yellow"
$secretPatterns = @(
    'password\s*=\s*["\'][^"\']{8,}["\']',
    'secret\s*=\s*["\'][^"\']{16,}["\']',
    'api[_-]?key\s*=\s*["\'][^"\']{20,}["\']',
    '[A-Za-z0-9]{32,}'
)

$stagedFiles = git diff --cached --name-only --diff-filter=ACM
foreach ($file in $stagedFiles) {
    if ($file -match '\.(go|yaml|yml|json|env)$' -and (Test-Path $file)) {
        $content = Get-Content $file -Raw
        foreach ($pattern in $secretPatterns) {
            if ($content -match $pattern -and $content -notmatch 'example|test|change-me|dummy') {
                Write-Message "  ❌ Potential secret found in: $file" "Red"
                $IssuesFound = $true
            }
        }
    }
}

if (-not $IssuesFound) {
    Write-Message "  ✓ No secrets detected" "Green"
}

# Check 2: SQL injection patterns
Write-Message "2. Checking for SQL injection vulnerabilities..." "Yellow"
$sqlIssues = $false
foreach ($file in $stagedFiles) {
    if ($file -match '\.go$' -and (Test-Path $file)) {
        $content = Get-Content $file
        $lineNum = 0
        foreach ($line in $content) {
            $lineNum++
            if ($line -match 'fmt\.Sprintf.*SELECT|INSERT|UPDATE|DELETE' -or
                $line -match 'query.*\+.*\$') {
                Write-Message "  ❌ Potential SQL injection in $file:$lineNum" "Red"
                Write-Message "     $line" "Red"
                $sqlIssues = $true
                $IssuesFound = $true
            }
        }
    }
}

if (-not $sqlIssues) {
    Write-Message "  ✓ No SQL injection patterns found" "Green"
}

# Check 3: Mock auth in production
Write-Message "3. Checking for mock auth in production..." "Yellow"
$mockAuthIssues = $false
foreach ($file in $stagedFiles) {
    if ($file -match 'config\.production\.(yaml|yml)$' -and (Test-Path $file)) {
        $content = Get-Content $file -Raw
        if ($content -match 'enable_mock_auth:\s*true') {
            Write-Message "  ❌ Mock auth enabled in production config!" "Red"
            $mockAuthIssues = $true
            $IssuesFound = $true
        }
    }
}

if (-not $mockAuthIssues) {
    Write-Message "  ✓ No mock auth in production config" "Green"
}

# Check 4: Missing ownership checks
Write-Message "4. Checking for ownership validation..." "Yellow"
$ownershipIssues = $false
foreach ($file in $stagedFiles) {
    if ($file -match 'handler.*\.go$' -and (Test-Path $file)) {
        $content = Get-Content $file -Raw
        if ($content -match 'func.*\(h.*Handler\).*(Credit|Debit|Transfer)' -and
            $content -notmatch 'GetAuthUserID|checkWalletOwnership') {
            Write-Message "  ⚠️  Potential missing ownership check in: $file" "Yellow"
            Write-Message "     Please verify ownership validation is present" "Yellow"
            # Don't fail commit, just warn
        }
    }
}

Write-Message "  ✓ Ownership check scan complete" "Green"

# Check 5: Run quick tests
Write-Message "5. Running quick tests..." "Yellow"
Push-Location (Split-Path $PSScriptRoot -Parent)
$testOutput = go test -short ./internal/adapters/http/middleware ./internal/adapters/http/handlers 2>&1
$testExitCode = $LASTEXITCODE
Pop-Location

if ($testExitCode -ne 0) {
    Write-Message "  ❌ Tests failed!" "Red"
    Write-Host $testOutput
    $IssuesFound = $true
} else {
    Write-Message "  ✓ Tests passed" "Green"
}

Write-Host ""
Write-Host "═══════════════════════════════════════════" -ForegroundColor Cyan

if ($IssuesFound) {
    Write-Host ""
    Write-Message "❌ Pre-commit checks FAILED" "Red"
    Write-Host ""
    Write-Message "Security issues detected. Please fix before committing." "Yellow"
    Write-Host ""
    Write-Message "To bypass (NOT RECOMMENDED):" "Gray"
    Write-Message "  git commit --no-verify" "Gray"
    Write-Host ""
    exit 1
} else {
    Write-Host ""
    Write-Message "✅ Pre-commit checks PASSED" "Green"
    Write-Host ""
    Write-Message "Your commit is security-approved! 🎉" "Cyan"
    Write-Host ""
    exit 0
}
