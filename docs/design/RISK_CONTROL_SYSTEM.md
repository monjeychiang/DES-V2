# 風控系統設計文檔

> 版本: 5.0  
> 日期: 2025-12-09  
> 狀態: **已實作 (Phase 1-5)**

## 1. 系統概覽

### 1.1 流程圖 (v5.0 - Evaluate-before-Lock)

```
Signal → EvaluateFull() → (失敗) → 直接返回
                        → (成功) → Lock(finalSize) → SL/TP → Enqueue → Executor → Exchange
                              ↓          ↓              ↓         ↓           ↓
                         [最終金額]  [per-strategy]   [WAL]   [指數退避]   [API]
```

### 1.2 設計原則

| 原則 | 說明 |
|------|------|
| **保護不阻礙** | 能調整就不拒絕，能警告就不強制 |
| **分層控制** | 全局 + 策略兩層獨立開關 |
| **軟限制** | 80% 警告 / 90% 縮小 / 100% 拒絕 |
| **單一入口** | EvaluateFull() 整合所有風控檢查 |

---

## 2. 分層架構

```
                    ┌─────────────────────────────┐
                    │      Global RiskConfig       │ ← 系統級上限
                    │   • 總曝險, 每日虧損/交易     │
                    │   • 軟限制閾值               │
                    │   • EnableRisk 總開關        │
                    └──────────────┬──────────────┘
                                   │
                      ┌────────────┼────────────┐
                      ▼            ▼            ▼
              ┌────────────┐┌────────────┐┌────────────┐
              │ Strategy A ││ Strategy B ││ Strategy C │
              │ • SL/TP    ││ • SL/TP    ││ • SL/TP    │
              │ • 倉位上限 ││ • 倉位上限 ││ • 倉位上限 │
              │ • per-SL/TP││ • per-SL/TP││ • per-SL/TP│
              └────────────┘└────────────┘└────────────┘
```

### 2.1 全局設定

| 設定 | 說明 | 預設值 |
|------|------|--------|
| `MaxTotalExposure` | 帳戶總曝險上限 | 5000.0 |
| `MaxDailyLoss` | 每日最大虧損 | 500.0 |
| `MaxDailyTrades` | 每日交易次數上限 | 20 |
| `WarningThreshold` | 警告閾值 | 0.8 |
| `CautionThreshold` | 縮單閾值 | 0.9 |
| `FailureMode` | 失敗模式 | FAIL_CLOSE |

### 2.2 策略設定

| 設定 | 說明 | 預設值 |
|------|------|--------|
| `MaxPositionSize` | 策略倉位上限 | 1000.0 |
| `StopLoss` | 止損 (覆蓋全局) | nil |
| `TakeProfit` | 止盈 (覆蓋全局) | nil |
| `EnableRisk` | 策略風控開關 | true |

---

## 3. 風控檢查流程

### 3.1 EvaluateFull (推薦入口)

```go
// 單一入口整合 QuickCheck + 完整評估
decision := riskMgr.EvaluateFull(signal, position, account, strategyID)
if !decision.Allowed {
    log.Printf("拒絕: %s", decision.Reason)
    return
}
// 評估通過後再鎖定餘額
balanceMgr.Lock(decision.AdjustedSize * price)
```

### 3.2 軟限制邏輯

| 使用率 | LimitLevel | 行為 |
|--------|------------|------|
| < 80% | NORMAL | 正常交易 |
| 80-90% | WARNING | 正常 + 警告 |
| 90-100% | CAUTION | 訂單縮半 |
| ≥ 100% | LIMIT | 拒絕 |

### 3.3 SL/TP Per-Strategy

```go
// 每策略獨立追蹤止損止盈
stopLossMgr.AddPosition(risk.StopLossPosition{
    StrategyID: sig.StrategyID, // 新增
    Symbol:     sig.Symbol,
    ...
})
// 內部使用 (strategyID, symbol) 作為 key
```

---

## 4. API 參考

### Manager 方法

| 方法 | 說明 |
|------|------|
| **`EvaluateFull()`** | **推薦入口** - 整合 QuickCheck + 評估 |
| `QuickCheck()` | 快速預檢 |
| `EvaluateSignalWithStrategy()` | 分層風控評估 |
| `GetConfig()` | 取得全局設定 |
| `GetStrategyConfig(id)` | 取得策略設定 |
| `GetRiskStats()` | 取得監控統計 |

### OrderQueue 方法

| 方法 | 說明 |
|------|------|
| `Enqueue()` | 訂單入隊 |
| `PendingNotional()` | 取得 pending 訂單總 notional |
| `Drain()` | 消費訂單 |

### 監控指標

| 指標 | 說明 |
|------|------|
| `checks_total` | 風控檢查總次數 |
| `rejections_total` | 拒絕總次數 |
| `warnings_total` | 警告總次數 |
| `avg_latency_ms` | 平均檢查延遲 |

---

## 5. 檔案索引

| 檔案 | 內容 |
|------|------|
| `internal/risk/types.go` | 類型定義、常數、預設值 |
| `internal/risk/manager.go` | EvaluateFull, QuickCheck, 風控核心邏輯 |
| `internal/risk/stoploss.go` | 止損止盈管理 (per-strategy) |
| `internal/order/queue.go` | 訂單佇列介面 (含 PendingNotional) |
| `internal/order/persistent_queue.go` | WAL 持久化實作 |
| `pkg/db/schema.go` | strategy_risk_configs 表 |
| `main.go` | 信號處理流程 (EvaluateFull + Evaluate-before-Lock) |
