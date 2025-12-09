# Risk Control DRY_RUN Test Script for DES Trading System
# Usage: .\test-risk-control.ps1
# Prerequisites: DRY_RUN mode server running on localhost:8080

param(
    [string]$BaseUrl = "http://localhost:8080/api",
    [switch]$Verbose,
    [switch]$SkipSoftLimits,
    [switch]$SkipLayered
)

$headers = @{
    "Content-Type" = "application/json"
}

$testResults = @()
$passed = 0
$failed = 0

function Write-TestHeader {
    param([string]$Title)
    Write-Host ""
    Write-Host "=" * 60 -ForegroundColor Cyan
    Write-Host " $Title" -ForegroundColor Cyan
    Write-Host "=" * 60 -ForegroundColor Cyan
    Write-Host ""
}

function Write-TestResult {
    param(
        [string]$TestId,
        [string]$TestName,
        [bool]$Success,
        [string]$Message = ""
    )
    
    $script:testResults += @{
        Id = $TestId
        Name = $TestName
        Success = $Success
        Message = $Message
    }
    
    if ($Success) {
        $script:passed++
        Write-Host "[$TestId] PASS: $TestName" -ForegroundColor Green
    } else {
        $script:failed++
        Write-Host "[$TestId] FAIL: $TestName" -ForegroundColor Red
        if ($Message) {
            Write-Host "       Reason: $Message" -ForegroundColor Yellow
        }
    }
}

function Invoke-ApiRequest {
    param(
        [string]$Method,
        [string]$Endpoint,
        [hashtable]$Body = $null
    )
    
    try {
        $uri = "$BaseUrl$Endpoint"
        $params = @{
            Uri = $uri
            Method = $Method
            Headers = $headers
            ErrorAction = "Stop"
        }
        
        if ($Body) {
            $params.Body = ($Body | ConvertTo-Json -Depth 5)
        }
        
        $response = Invoke-RestMethod @params
        return @{ Success = $true; Data = $response }
    }
    catch {
        return @{ Success = $false; Error = $_.Exception.Message }
    }
}

function Get-RiskConfig {
    return Invoke-ApiRequest -Method "Get" -Endpoint "/risk"
}

function Update-RiskConfig {
    param([hashtable]$Config)
    return Invoke-ApiRequest -Method "Put" -Endpoint "/risk" -Body $Config
}

function Get-RiskStats {
    return Invoke-ApiRequest -Method "Get" -Endpoint "/risk/stats"
}

function Send-TestSignal {
    param(
        [string]$Symbol = "BTCUSDT",
        [string]$Action = "BUY",
        [double]$Size = 0.01,
        [double]$Price = 50000,
        [string]$StrategyId = "test-strategy"
    )
    
    $signal = @{
        symbol = $Symbol
        action = $Action
        size = $Size
        price = $Price
        strategy_id = $StrategyId
    }
    
    return Invoke-ApiRequest -Method "Post" -Endpoint "/signals" -Body $signal
}

function Get-Balance {
    return Invoke-ApiRequest -Method "Get" -Endpoint "/balance"
}

function Get-Orders {
    return Invoke-ApiRequest -Method "Get" -Endpoint "/orders"
}

# =============================================================================
# Test Suite Start
# =============================================================================

Write-Host ""
Write-Host @"
╔═══════════════════════════════════════════════════════════════╗
║       DES Risk Control System - DRY_RUN Test Suite            ║
║                        Version 1.0                             ║
╚═══════════════════════════════════════════════════════════════╝
"@ -ForegroundColor Magenta

Write-Host "Target: $BaseUrl" -ForegroundColor Gray
Write-Host "Time: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')" -ForegroundColor Gray
Write-Host ""

# -----------------------------------------------------------------------------
# Pre-flight Check
# -----------------------------------------------------------------------------

Write-TestHeader "Pre-flight Checks"

Write-Host "Checking server connectivity..." -ForegroundColor Yellow
$balanceCheck = Get-Balance
if ($balanceCheck.Success) {
    Write-Host "Server is running" -ForegroundColor Green
    if ($Verbose) {
        Write-Host "Balance: $($balanceCheck.Data | ConvertTo-Json -Compress)" -ForegroundColor Gray
    }
} else {
    Write-Host "FATAL: Cannot connect to server at $BaseUrl" -ForegroundColor Red
    Write-Host "Please ensure DRY_RUN mode is active:" -ForegroundColor Yellow
    Write-Host "  set DRY_RUN=true" -ForegroundColor White
    Write-Host "  set DRY_RUN_INITIAL_BALANCE=10000" -ForegroundColor White
    Write-Host "  go run ." -ForegroundColor White
    exit 1
}

# Get initial risk config
$initialConfig = Get-RiskConfig
if (-not $initialConfig.Success) {
    Write-Host "WARNING: Could not get initial risk config" -ForegroundColor Yellow
}

# -----------------------------------------------------------------------------
# Test Section 1: Basic Functionality (T1-T3)
# -----------------------------------------------------------------------------

Write-TestHeader "Section 1: Basic Functionality (T1-T3)"

# T1: Normal Order
Write-Host "Testing T1: Normal Order..." -ForegroundColor Yellow
$t1Result = Send-TestSignal -Size 0.01 -Price 50000
if ($t1Result.Success) {
    Write-TestResult -TestId "T1" -TestName "Normal Order" -Success $true
} else {
    Write-TestResult -TestId "T1" -TestName "Normal Order" -Success $false -Message $t1Result.Error
}

Start-Sleep -Milliseconds 500

# T2: Risk Disabled (Global)
Write-Host "Testing T2: Risk Disabled (Global)..." -ForegroundColor Yellow
$disableConfig = @{
    enable_risk = $false
}
$configUpdate = Update-RiskConfig -Config $disableConfig
if ($configUpdate.Success) {
    $t2Order = Send-TestSignal -Size 1.0 -Price 50000  # Large order that would normally fail
    if ($t2Order.Success) {
        Write-TestResult -TestId "T2" -TestName "Risk Disabled Bypass" -Success $true
    } else {
        Write-TestResult -TestId "T2" -TestName "Risk Disabled Bypass" -Success $false -Message "Order failed with risk disabled"
    }
    
    # Re-enable risk
    $reEnableConfig = @{
        enable_risk = $true
    }
    Update-RiskConfig -Config $reEnableConfig | Out-Null
} else {
    Write-TestResult -TestId "T2" -TestName "Risk Disabled Bypass" -Success $false -Message "Could not update config"
}

Start-Sleep -Milliseconds 500

# T3: Strategy Risk Disabled
Write-Host "Testing T3: Strategy Risk Disabled..." -ForegroundColor Yellow
# This would require setting strategy-specific config via API
# For now, mark as manual verification needed
Write-Host "  Note: T3 requires manual verification via strategy config API" -ForegroundColor Gray
$testResults += @{ Id = "T3"; Name = "Strategy Risk Disabled"; Success = $null; Message = "Manual verification required" }

# -----------------------------------------------------------------------------
# Test Section 2: Soft Limits (T4-T6)
# -----------------------------------------------------------------------------

if (-not $SkipSoftLimits) {
    Write-TestHeader "Section 2: Soft Limits (T4-T6)"
    
    # Configure for soft limit testing
    $softLimitConfig = @{
        enable_risk = $true
        max_daily_loss = 500
        warning_threshold = 0.8
        caution_threshold = 0.9
        caution_size_ratio = 0.5
    }
    $configResult = Update-RiskConfig -Config $softLimitConfig
    
    if ($configResult.Success) {
        Write-Host "Soft limit config applied" -ForegroundColor Green
        
        # T4: 80% Warning
        Write-Host "Testing T4: 80% Warning Level..." -ForegroundColor Yellow
        Write-Host "  (Requires accumulated loss of ~$400 to trigger)" -ForegroundColor Gray
        Write-Host "  Check server logs for warning messages after trades" -ForegroundColor Gray
        $testResults += @{ Id = "T4"; Name = "80% Warning Level"; Success = $null; Message = "Check server logs" }
        
        # T5: 90% Caution (Size Reduction)
        Write-Host "Testing T5: 90% Caution (Size Reduction)..." -ForegroundColor Yellow
        Write-Host "  (Requires accumulated loss of ~$450 to trigger)" -ForegroundColor Gray
        Write-Host "  Order size should be reduced to 50%" -ForegroundColor Gray
        $testResults += @{ Id = "T5"; Name = "90% Size Reduction"; Success = $null; Message = "Check order size" }
        
        # T6: 100% Rejection
        Write-Host "Testing T6: 100% Rejection..." -ForegroundColor Yellow
        Write-Host "  (Requires accumulated loss >= $500 to trigger)" -ForegroundColor Gray
        $testResults += @{ Id = "T6"; Name = "100% Rejection"; Success = $null; Message = "Manual verification" }
        
    } else {
        Write-Host "Could not apply soft limit config" -ForegroundColor Red
        Write-TestResult -TestId "T4" -TestName "80% Warning Level" -Success $false -Message "Config failed"
        Write-TestResult -TestId "T5" -TestName "90% Size Reduction" -Success $false -Message "Config failed"
        Write-TestResult -TestId "T6" -TestName "100% Rejection" -Success $false -Message "Config failed"
    }
} else {
    Write-Host "Skipping Soft Limits tests (use -SkipSoftLimits:$false to enable)" -ForegroundColor Yellow
}

# -----------------------------------------------------------------------------
# Test Section 3: Layered Risk Control (T7-T9)
# -----------------------------------------------------------------------------

if (-not $SkipLayered) {
    Write-TestHeader "Section 3: Layered Risk Control (T7-T9)"
    
    Write-Host "T7-T9 tests require strategy-specific configuration" -ForegroundColor Yellow
    Write-Host "Use the API to configure per-strategy risk settings:" -ForegroundColor Gray
    Write-Host "  PUT /api/risk/strategy/{strategy_id}" -ForegroundColor White
    Write-Host ""
    
    $testResults += @{ Id = "T7"; Name = "Strategy Position Limit"; Success = $null; Message = "Manual config required" }
    $testResults += @{ Id = "T8"; Name = "Strategy SL/TP Override"; Success = $null; Message = "Manual config required" }
    $testResults += @{ Id = "T9"; Name = "Multi-Strategy Independence"; Success = $null; Message = "Manual config required" }
} else {
    Write-Host "Skipping Layered tests (use -SkipLayered:$false to enable)" -ForegroundColor Yellow
}

# -----------------------------------------------------------------------------
# Test Section 4: Get Risk Stats
# -----------------------------------------------------------------------------

Write-TestHeader "Risk Statistics"

$stats = Get-RiskStats
if ($stats.Success) {
    Write-Host "Current Risk Stats:" -ForegroundColor Green
    $stats.Data | ConvertTo-Json -Depth 3 | Write-Host
} else {
    Write-Host "Could not retrieve risk stats" -ForegroundColor Yellow
}

# -----------------------------------------------------------------------------
# Test Summary
# -----------------------------------------------------------------------------

Write-TestHeader "Test Summary"

Write-Host "Results:" -ForegroundColor Cyan
Write-Host "  Passed:  $passed" -ForegroundColor Green
Write-Host "  Failed:  $failed" -ForegroundColor $(if ($failed -gt 0) { "Red" } else { "Gray" })
Write-Host "  Manual:  $($testResults | Where-Object { $null -eq $_.Success } | Measure-Object).Count" -ForegroundColor Yellow
Write-Host ""

if ($testResults.Count -gt 0) {
    Write-Host "Detailed Results:" -ForegroundColor Cyan
    Write-Host "-" * 60
    foreach ($result in $testResults) {
        $status = if ($null -eq $result.Success) { "MANUAL" } elseif ($result.Success) { "PASS" } else { "FAIL" }
        $color = if ($null -eq $result.Success) { "Yellow" } elseif ($result.Success) { "Green" } else { "Red" }
        Write-Host "[$($result.Id)] $status - $($result.Name)" -ForegroundColor $color
    }
}

Write-Host ""
Write-Host "Test completed at $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')" -ForegroundColor Gray

# Exit with appropriate code
if ($failed -gt 0) {
    exit 1
} else {
    exit 0
}
