# Soft Limits Test Helper - Simulate Daily Losses
# Usage: .\test-soft-limits.ps1 -TargetLossPercent 85
# This script helps simulate accumulated daily losses for testing soft limit behavior

param(
    [string]$BaseUrl = "http://localhost:8080/api",
    [int]$TargetLossPercent = 80,
    [double]$MaxDailyLoss = 500,
    [switch]$WatchMode
)

$headers = @{
    "Content-Type" = "application/json"
}

function Write-Banner {
    Write-Host ""
    Write-Host @"
╔═══════════════════════════════════════════════════════════════╗
║           Soft Limits Test Helper - DRY_RUN Mode              ║
╚═══════════════════════════════════════════════════════════════╝
"@ -ForegroundColor Magenta
}

function Get-CurrentMetrics {
    try {
        $response = Invoke-RestMethod -Uri "$BaseUrl/risk" -Method Get -Headers $headers
        return $response
    }
    catch {
        return $null
    }
}

function Update-DailyLoss {
    param([double]$LossAmount)
    
    # This simulates a losing trade to accumulate daily losses
    $trade = @{
        symbol = "BTCUSDT"
        pnl    = - $LossAmount
        fee    = 0.5
    }
    
    try {
        $response = Invoke-RestMethod -Uri "$BaseUrl/risk/trade" -Method Post -Headers $headers -Body ($trade | ConvertTo-Json)
        return @{ Success = $true; Data = $response }
    }
    catch {
        return @{ Success = $false; Error = $_.Exception.Message }
    }
}

function Send-TestSignal {
    param(
        [double]$Size = 0.01,
        [double]$Price = 50000
    )
    
    $signal = @{
        symbol      = "BTCUSDT"
        action      = "BUY"
        size        = $Size
        price       = $Price
        strategy_id = "soft-limit-test"
    }
    
    try {
        $response = Invoke-RestMethod -Uri "$BaseUrl/signals" -Method Post -Headers $headers -Body ($signal | ConvertTo-Json)
        return @{ Success = $true; Data = $response }
    }
    catch {
        return @{ Success = $false; Error = $_.Exception.Message }
    }
}

function Show-RiskLevel {
    param([double]$Percentage)
    
    $bar = ""
    $barLength = 50
    $filled = [math]::Floor($barLength * $Percentage / 100)
    
    for ($i = 0; $i -lt $barLength; $i++) {
        if ($i -lt $filled) {
            $bar += "█"
        }
        else {
            $bar += "░"
        }
    }
    
    $color = "Green"
    $level = "NORMAL"
    if ($Percentage -ge 100) {
        $color = "Red"
        $level = "LIMIT"
    }
    elseif ($Percentage -ge 90) {
        $color = "DarkYellow"
        $level = "CAUTION"
    }
    elseif ($Percentage -ge 80) {
        $color = "Yellow"
        $level = "WARNING"
    }
    
    Write-Host ""
    Write-Host "Daily Loss Usage: [$bar] $($Percentage.ToString("F1"))%" -ForegroundColor $color
    Write-Host "Level: $level" -ForegroundColor $color
    Write-Host ""
}

# =============================================================================
# Main Script
# =============================================================================

Write-Banner

Write-Host "Configuration:" -ForegroundColor Cyan
Write-Host "  Target Loss: $TargetLossPercent%" -ForegroundColor White
Write-Host "  Max Daily Loss: `$$MaxDailyLoss" -ForegroundColor White
Write-Host "  API Endpoint: $BaseUrl" -ForegroundColor White
Write-Host ""

# Check server connectivity
$metrics = Get-CurrentMetrics
if (-not $metrics) {
    Write-Host "ERROR: Cannot connect to server" -ForegroundColor Red
    Write-Host "Please ensure DRY_RUN mode is active" -ForegroundColor Yellow
    exit 1
}

$targetLoss = $MaxDailyLoss * ($TargetLossPercent / 100)

Write-Host "Target accumulated loss: `$$($targetLoss.ToString("F2"))" -ForegroundColor Cyan
Write-Host ""

if ($WatchMode) {
    Write-Host "Watch Mode - Displaying real-time risk levels" -ForegroundColor Yellow
    Write-Host "Press Ctrl+C to exit" -ForegroundColor Gray
    Write-Host ""
    
    while ($true) {
        $metrics = Get-CurrentMetrics
        if ($metrics -and $metrics.metrics) {
            $currentLoss = $metrics.metrics.daily_losses
            $percentage = ($currentLoss / $MaxDailyLoss) * 100
            
            Clear-Host
            Write-Banner
            Write-Host "Real-time Risk Level Monitor" -ForegroundColor Cyan
            Write-Host "Current Daily Losses: `$$($currentLoss.ToString("F2"))" -ForegroundColor White
            Show-RiskLevel -Percentage $percentage
            
            Write-Host "Press Ctrl+C to exit" -ForegroundColor Gray
        }
        Start-Sleep -Seconds 2
    }
}
else {
    Write-Host "Interactive Test Mode" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Commands:" -ForegroundColor Cyan
    Write-Host "  1 - Check current risk level" -ForegroundColor White
    Write-Host "  2 - Simulate $50 loss" -ForegroundColor White
    Write-Host "  3 - Simulate $100 loss" -ForegroundColor White
    Write-Host "  4 - Test signal (should see warning/reduction/rejection)" -ForegroundColor White
    Write-Host "  5 - Set loss to 80% (warning threshold)" -ForegroundColor Yellow
    Write-Host "  6 - Set loss to 90% (caution threshold)" -ForegroundColor DarkYellow
    Write-Host "  7 - Set loss to 100% (limit threshold)" -ForegroundColor Red
    Write-Host "  q - Quit" -ForegroundColor Gray
    Write-Host ""
    
    while ($true) {
        $userInput = Read-Host "Enter command"
        
        switch ($userInput) {
            "1" {
                $metrics = Get-CurrentMetrics
                if ($metrics -and $metrics.metrics) {
                    $currentLoss = $metrics.metrics.daily_losses
                    $percentage = ($currentLoss / $MaxDailyLoss) * 100
                    Write-Host "Current Daily Losses: `$$($currentLoss.ToString("F2"))" -ForegroundColor White
                    Show-RiskLevel -Percentage $percentage
                }
                else {
                    Write-Host "Could not retrieve metrics" -ForegroundColor Red
                }
            }
            "2" {
                Write-Host "Simulating $50 loss..." -ForegroundColor Yellow
                $result = Update-DailyLoss -LossAmount 50
                if ($result.Success) {
                    Write-Host "Loss recorded" -ForegroundColor Green
                }
                else {
                    Write-Host "Note: Direct loss API may not be available. Use manual trades." -ForegroundColor Yellow
                }
            }
            "3" {
                Write-Host "Simulating $100 loss..." -ForegroundColor Yellow
                $result = Update-DailyLoss -LossAmount 100
                if ($result.Success) {
                    Write-Host "Loss recorded" -ForegroundColor Green
                }
                else {
                    Write-Host "Note: Direct loss API may not be available. Use manual trades." -ForegroundColor Yellow
                }
            }
            "4" {
                Write-Host "Sending test signal (size: 0.1 BTC)..." -ForegroundColor Yellow
                $result = Send-TestSignal -Size 0.1 -Price 50000
                if ($result.Success) {
                    Write-Host "Signal processed:" -ForegroundColor Green
                    $result.Data | ConvertTo-Json | Write-Host
                }
                else {
                    Write-Host "Signal result: $($result.Error)" -ForegroundColor Yellow
                }
            }
            "5" {
                Write-Host "Setting loss to 80% ($($MaxDailyLoss * 0.8))..." -ForegroundColor Yellow
                # This would require direct DB access or metrics API
                Write-Host "Note: Use manual trades to accumulate losses" -ForegroundColor Gray
            }
            "6" {
                Write-Host "Setting loss to 90% ($($MaxDailyLoss * 0.9))..." -ForegroundColor DarkYellow
                Write-Host "Note: Use manual trades to accumulate losses" -ForegroundColor Gray
            }
            "7" {
                Write-Host "Setting loss to 100% ($MaxDailyLoss)..." -ForegroundColor Red
                Write-Host "Note: Use manual trades to accumulate losses" -ForegroundColor Gray
            }
            "q" {
                Write-Host "Exiting..." -ForegroundColor Gray
                exit 0
            }
            default {
                Write-Host "Unknown command: $userInput" -ForegroundColor Red
            }
        }
        Write-Host ""
    }
}
