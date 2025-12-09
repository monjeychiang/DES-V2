# DRY_RUN Environment Setup Script
# Usage: .\setup-dry-run.ps1
# Sets up environment and starts the trading system in DRY_RUN mode

param(
    [double]$InitialBalance = 10000,
    [switch]$ResetDB
)

Write-Host ""
Write-Host @"
╔═══════════════════════════════════════════════════════════════╗
║             DRY_RUN Environment Setup                          ║
╚═══════════════════════════════════════════════════════════════╝
"@ -ForegroundColor Cyan

$projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)

Write-Host "Project Root: $projectRoot" -ForegroundColor Gray
Write-Host ""

# Set environment variables
Write-Host "Setting environment variables..." -ForegroundColor Yellow
$env:DRY_RUN = "true"
$env:DRY_RUN_INITIAL_BALANCE = $InitialBalance.ToString()

Write-Host "  DRY_RUN = true" -ForegroundColor Green
Write-Host "  DRY_RUN_INITIAL_BALANCE = $InitialBalance" -ForegroundColor Green
Write-Host ""

# Reset database if requested
if ($ResetDB) {
    Write-Host "Resetting database..." -ForegroundColor Yellow
    $dbPath = Join-Path $projectRoot "trading.db"
    if (Test-Path $dbPath) {
        Remove-Item $dbPath -Force
        Write-Host "  Removed existing database" -ForegroundColor Green
    }
}

# Configure risk settings for testing
Write-Host "Recommended risk settings for testing:" -ForegroundColor Cyan
Write-Host @"
{
    "enable_risk": true,
    "max_total_exposure": 5000,
    "max_daily_loss": 500,
    "max_daily_trades": 20,
    "warning_threshold": 0.8,
    "caution_threshold": 0.9,
    "caution_size_ratio": 0.5,
    "default_stop_loss": 0.02,
    "default_take_profit": 0.05,
    "use_daily_trade_limit": true,
    "use_daily_loss_limit": true
}
"@ -ForegroundColor White
Write-Host ""

# Start instruction
Write-Host "To start the trading system in DRY_RUN mode:" -ForegroundColor Yellow
Write-Host ""
Write-Host "  cd $projectRoot" -ForegroundColor White
Write-Host "  go run ." -ForegroundColor White
Write-Host ""

Write-Host "Or run directly:" -ForegroundColor Yellow
Write-Host ""
Write-Host "  Push-Location '$projectRoot'; go run .; Pop-Location" -ForegroundColor White
Write-Host ""

# Prompt to start
$confirm = Read-Host "Start the trading system now? (y/n)"
if ($confirm -eq "y") {
    Write-Host ""
    Write-Host "Starting trading system in DRY_RUN mode..." -ForegroundColor Green
    Write-Host "Press Ctrl+C to stop" -ForegroundColor Gray
    Write-Host ""
    
    Push-Location $projectRoot
    go run .
    Pop-Location
}
else {
    Write-Host ""
    Write-Host "Setup complete. Run the commands above to start." -ForegroundColor Gray
}
