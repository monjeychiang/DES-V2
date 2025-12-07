# API Testing Script for DES Trading System
# Usage: .\test-api.ps1

$baseUrl = "http://localhost:8080/api"
$headers = @{
    "Content-Type" = "application/json"
}

Write-Host "===== DES Trading System API Test =====" -ForegroundColor Cyan
Write-Host ""

# Test 1: Get Strategies
Write-Host "[1] Testing GET /strategies..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$baseUrl/strategies" -Method Get -Headers $headers
    Write-Host "Success: Retrieved $($response.Count) strategies" -ForegroundColor Green
    $response | ConvertTo-Json -Depth 3 | Write-Host
    
    # Store first strategy ID for later tests
    if ($response.Count -gt 0) {
        $strategyId = $response[0].id
        $strategyName = $response[0].name
        Write-Host "Using strategy: $strategyName (ID: $strategyId)" -ForegroundColor Gray
    }
}
catch {
    Write-Host "Failed: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

# Test 2: Get Orders
Write-Host "[2] Testing GET /orders..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$baseUrl/orders" -Method Get -Headers $headers
    Write-Host "Success: Retrieved $($response.Count) orders" -ForegroundColor Green
    $response | Select-Object -First 3 | ConvertTo-Json -Depth 2 | Write-Host
}
catch {
    Write-Host "Failed: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

# Test 3: Get Positions
Write-Host "[3] Testing GET /positions..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$baseUrl/positions" -Method Get -Headers $headers
    Write-Host "Success: Retrieved $($response.Count) positions" -ForegroundColor Green
    $response | ConvertTo-Json -Depth 2 | Write-Host
}
catch {
    Write-Host "Failed: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

# Test 4: Get Balance
Write-Host "[4] Testing GET /balance..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$baseUrl/balance" -Method Get -Headers $headers
    Write-Host "Success: Balance retrieved" -ForegroundColor Green
    $response | ConvertTo-Json | Write-Host
}
catch {
    Write-Host "Failed: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

# Test 5: Get Risk Metrics
Write-Host "[5] Testing GET /risk..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$baseUrl/risk" -Method Get -Headers $headers
    Write-Host "Success: Risk metrics retrieved" -ForegroundColor Green
    $response | ConvertTo-Json | Write-Host
}
catch {
    Write-Host "Failed: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

# Strategy Control Tests (only if we have a strategy ID)
if ($strategyId) {
    Write-Host "===== Strategy Control Tests =====" -ForegroundColor Cyan
    Write-Host ""
    
    # Test 6: Pause Strategy
    Write-Host "[6] Testing POST /strategies/$strategyId/pause..." -ForegroundColor Yellow
    try {
        $response = Invoke-RestMethod -Uri "$baseUrl/strategies/$strategyId/pause" -Method Post -Headers $headers
        Write-Host "Success: Strategy paused" -ForegroundColor Green
        $response | ConvertTo-Json | Write-Host
        Start-Sleep -Seconds 1
    }
    catch {
        Write-Host "Failed: $($_.Exception.Message)" -ForegroundColor Red
    }
    Write-Host ""
    
    # Test 7: Start/Resume Strategy
    Write-Host "[7] Testing POST /strategies/$strategyId/start..." -ForegroundColor Yellow
    try {
        $response = Invoke-RestMethod -Uri "$baseUrl/strategies/$strategyId/start" -Method Post -Headers $headers
        Write-Host "Success: Strategy started" -ForegroundColor Green
        $response | ConvertTo-Json | Write-Host
        Start-Sleep -Seconds 1
    }
    catch {
        Write-Host "Failed: $($_.Exception.Message)" -ForegroundColor Red
    }
    Write-Host ""
    
    # Test 8: Update Strategy Parameters
    Write-Host "[8] Testing PUT /strategies/$strategyId/params..." -ForegroundColor Yellow
    try {
        $newParams = @{
            fast = 10
            slow = 30
            size = 0.01
        }
        $body = $newParams | ConvertTo-Json
        $response = Invoke-RestMethod -Uri "$baseUrl/strategies/$strategyId/params" -Method Put -Headers $headers -Body $body
        Write-Host "Success: Parameters updated" -ForegroundColor Green
        $response | ConvertTo-Json | Write-Host
        Start-Sleep -Seconds 1
    }
    catch {
        Write-Host "Failed: $($_.Exception.Message)" -ForegroundColor Red
    }
    Write-Host ""
}

Write-Host "===== Test Complete =====" -ForegroundColor Cyan
Write-Host ""
Write-Host "Note: Destructive tests (stop, panic) are not included by default." -ForegroundColor Yellow
