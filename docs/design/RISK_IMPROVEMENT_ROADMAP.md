# 風控系統改進路線圖

> 版本: 3.0  
> 日期: 2025-12-09  
> 狀態: **Phase 1-5 已完成**

## 進度總覽

| Phase | 內容 | 狀態 |
|-------|------|------|
| **1-2** | 軟限制 + QuickCheck + Metrics | ✅ 完成 |
| **3** | 分層風控 (全局/策略) | ✅ 完成 |
| **4** | 曝險含 pending + Lock 順序 | ✅ 完成 |
| **5** | 單一入口 + SL/TP per-strategy | ✅ 完成 |
| **6** | 狀態機 + Jitter + Rate limit + 冪等 | 📋 可延後 |

---

## 已完成功能 ✅

### Phase 1-2: 軟限制 + 快速檢查

| 功能 | 說明 |
|------|------|
| 軟限制閾值 | 80%/90%/100% 分級警告/縮單/拒絕 |
| 風控 Metrics | 檢查/拒絕/延遲計數 |
| QuickCheck | 快速預檢，無需鎖定餘額 |
| FailureMode | FAIL_CLOSE / FAIL_LIMIT |

### Phase 3: 分層風控

| 功能 | 說明 |
|------|------|
| 全局設定 | MaxTotalExposure, MaxDailyLoss, MaxDailyTrades |
| 策略設定 | MaxPositionSize, SL/TP 覆蓋全局 |
| 策略設定表 | strategy_risk_configs DB |

### Phase 4: 曝險計算 + Lock 順序

| 功能 | 說明 |
|------|------|
| PendingNotional | OrderQueue 支援 pending 訂單 notional 計算 |
| Evaluate-before-Lock | 先評估再鎖定，減少 Lock/Unlock 分支 |

### Phase 5: 單一入口 + SL/TP 重構

| 功能 | 說明 |
|------|------|
| EvaluateFull | 整合 QuickCheck + EvaluateSignalWithStrategy |
| StopLossPosition.StrategyID | 支援 per-strategy SL/TP 追蹤 |
| strategyKey() | 使用 (strategyID, symbol) 作為 key |

---

## Phase 6: 可延後 📋

| 項目 | 說明 | 預估 |
|------|------|------|
| Order 狀態機 | RECEIVED→EVALUATED→ENQUEUED→... | 1h |
| Retry jitter | backoff 加隨機延遲 | 15min |
| Rate limit | 每 exchange 限流 | 30min |
| 冪等機制 | RequestID 去重 | 30min |
| Circuit Breaker | 連續失敗熔斷 | 40min |
| EventBus 改進 | 事件持久化 | 1h |

---

## 當前流程 (v5.0)

```
Signal → EvaluateFull() → (失敗) → 直接返回
                        → (成功) → Lock(finalSize) → SL/TP → Enqueue
```

### EvaluateFull 內部流程

```
EvaluateFull()
├─ QuickCheck (快速預檢)
│   ├─ DailyTrades 限制
│   └─ DailyLoss 限制 (含軟限制)
│
└─ EvaluateSignalWithStrategy (完整評估)
    ├─ 全局檢查 (不可繞過)
    │   ├─ MaxTotalExposure
    │   └─ DailyLoss/Trades (軟限制)
    └─ 策略檢查
        ├─ MaxPositionSize
        └─ OrderSize 限制
```

---

## 修改檔案摘要

| 檔案 | Phase | 變更 |
|------|-------|------|
| `types.go` | 1-2 | 軟限制閾值, FailureMode, QuickCheckResult |
| `manager.go` | 1-5 | QuickCheck, EvaluateFull, GetRiskStats |
| `stoploss.go` | 5 | StrategyID, strategyKey() |
| `queue.go` | 4 | PendingNotional() |
| `persistent_queue.go` | 4 | PendingNotional() |
| `main.go` | 4-5 | EvaluateFull, Evaluate-before-Lock |
