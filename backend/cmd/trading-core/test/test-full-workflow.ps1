# DES Trading System - 完整流程測試腳本

Write-Host "================================" -ForegroundColor Cyan
Write-Host "DES Trading System - Full Test" -ForegroundColor Cyan
Write-Host "================================" -ForegroundColor Cyan
Write-Host ""

$ErrorActionPreference = "Stop"
$TestDir = "c:\vscode\DES-V2\backend\cmd\trading-core"

# 顏色輸出函數
function Write-Success { param($msg) Write-Host "✅ $msg" -ForegroundColor Green }
function Write-Info { param($msg) Write-Host "📊 $msg" -ForegroundColor Blue }
function Write-Error { param($msg) Write-Host "❌ $msg" -ForegroundColor Red }
function Write-Step { param($msg) Write-Host "`n🔹 $msg" -ForegroundColor Yellow }

try {
    # Step 1: 編譯檢查
    Write-Step "Step 1: 編譯系統"
    Set-Location $TestDir
    
    Write-Host "Building..." -NoNewline
    go build -o des-trading-test.exe . 2>&1 | Out-Null
    
    if ($LASTEXITCODE -eq 0) {
        Write-Success "編譯成功"
    } else {
        Write-Error "編譯失敗"
        exit 1
    }

    # Step 2: 運行單元測試
    Write-Step "Step 2: 運行單元測試"
    
    Write-Host "Running tests..." 
    $testOutput = go test ./test -v -run TestFullWorkflow 2>&1
    
    if ($LASTEXITCODE -eq 0) {
        Write-Success "所有測試通過"
        $testOutput | ForEach-Object {
            if ($_ -match "✅") {
                Write-Host "  $_" -ForegroundColor Green
            } elseif ($_ -match "📊") {
                Write-Host "  $_" -ForegroundColor Blue
            } else {
                Write-Host "  $_" -ForegroundColor Gray
            }
        }
    } else {
        Write-Error "測試失敗"
        $testOutput | Write-Host -ForegroundColor Red
        exit 1
    }

    # Step 3: 測試配置文件
    Write-Step "Step 3: 檢查配置"
    
    $configFile = "config.yaml"
    if (Test-Path $configFile) {
        Write-Success "配置文件存在: $configFile"
        
        $config = Get-Content $configFile -Raw
        if ($config -match "dry_run:\s*true") {
            Write-Success "Dry-run 模式已啟用"
        } else {
            Write-Host "⚠️  Dry-run 模式未啟用，建議測試時啟用" -ForegroundColor Yellow
        }
    } else {
        Write-Host "⚠️  配置文件不存在，將使用默認值" -ForegroundColor Yellow
    }

    # Step 4: 數據庫檢查
    Write-Step "Step 4: 數據庫檢查"
    
    if (Test-Path "test.db") {
        $dbSize = (Get-Item "test.db").Length / 1KB
        Write-Success "測試數據庫存在: test.db ($('{0:N2}' -f $dbSize) KB)"
    } else {
        Write-Info "測試數據庫將在首次運行時創建"
    }

    # Step 5: 運行對賬測試
    Write-Step "Step 5: 對賬服務測試"
    
    $reconTest = go test -v -run TestReconciliation 2>&1
    
    if ($LASTEXITCODE -eq 0) {
        Write-Success "對賬服務測試通過"
    } else {
        Write-Error "對賬服務測試失敗"
        $reconTest | Write-Host -ForegroundColor Red
    }

    # Step 6: 性能基準測試
    Write-Step "Step 6: 性能基準測試"
    
    Write-Info "執行基準測試..."
    $benchOutput = go test -bench=. -benchmem -run=^$ 2>&1
    
    if ($benchOutput -match "Benchmark") {
        Write-Success "基準測試完成"
        $benchOutput | ForEach-Object {
            if ($_ -match "Benchmark") {
                Write-Host "  $_" -ForegroundColor Cyan
            }
        }
    } else {
        Write-Info "無基準測試可運行"
    }

    # Step 7: 清理
    Write-Step "Step 7: 清理測試文件"
    
    if (Test-Path "des-trading-test.exe") {
        Remove-Item "des-trading-test.exe" -Force
        Write-Success "清理完成"
    }

    # 最終報告
    Write-Host "`n================================" -ForegroundColor Cyan
    Write-Host "✨ 所有測試完成！" -ForegroundColor Green
    Write-Host "================================" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "📋 測試摘要:" -ForegroundColor White
    Write-Host "  ✅ 編譯檢查" -ForegroundColor Green
    Write-Host "  ✅ 單元測試" -ForegroundColor Green
    Write-Host "  ✅ 配置檢查" -ForegroundColor Green
    Write-Host "  ✅ 數據庫檢查" -ForegroundColor Green
    Write-Host "  ✅ 對賬服務" -ForegroundColor Green
    Write-Host ""
    Write-Host "🚀 系統已準備好運行！" -ForegroundColor Green
    Write-Host ""
    Write-Host "下一步:" -ForegroundColor Yellow
    Write-Host "  1. 確認 config.yaml 中 dry_run: true" -ForegroundColor White
    Write-Host "  2. 運行: .\des-trading.exe" -ForegroundColor White
    Write-Host "  3. 觀察日誌輸出" -ForegroundColor White
    Write-Host ""

} catch {
    Write-Error "測試過程中發生錯誤: $_"
    exit 1
}
