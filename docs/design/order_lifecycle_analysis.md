# 訂單生命週期分析

> **更新日期**: 2025-12-08  
> **狀態**: 已審核校正

## 📋 訂單送出後處理流程

```
訂單送出
  ↓
1. 訂單入隊
  ↓
2. 風控驗證
  ↓
3. 訂單執行
  ├─ Dry-Run: 模擬執行
  └─ Production: 真實下單
  ↓
4. 訂單狀態追蹤
  ├─ NEW → PENDING → FILLED
  └─ CANCELLED / REJECTED
  ↓
5. 成交處理
  ├─ 更新持倉狀態
  ├─ 更新餘額
  ├─ 計算 PnL
  └─ 發布事件
  ↓
6. 風控更新
  ↓
7. 止損追蹤
  ↓
8. 數據持久化
```

---

## 📊 功能完成度 (已更新)

| 功能 | 狀態 | 檔案位置 |
|------|------|----------|
| 訂單入隊 | ✅ 完成 | `internal/order/queue.go` |
| 風控驗證 | ✅ 完成 | `internal/risk/manager.go` |
| 訂單執行 | ✅ 完成 | `internal/order/executor.go` |
| Dry-Run 模式 | ✅ 完成 | `internal/order/dry_run.go` |
| 數據持久化 | ✅ 完成 | `pkg/db/models.go` |
| **餘額管理** | ✅ 完成 | `internal/balance/manager.go` |
| **User Data Stream (Spot)** | ✅ 完成 | `internal/order/user_stream_spot.go` |
| **User Data Stream (Futures)** | ✅ 完成 | `internal/order/user_stream_futures.go` |
| **成交事件發布** | ✅ 完成 | `EventOrderFilled` 多處發布 |
| **部分成交處理** | ✅ 完成 | `filled_qty` 欄位追蹤 |
| 持倉更新 | ✅ 完成 | `internal/state/manager.go` |
| 風控指標更新 | ⚠️ 待確認 | `UpdateMetrics` 調用點待查 |
| 止損追蹤 | ✅ 完成 | `internal/risk/stop_loss.go` |

**總體完成度**: ~90%

---

## ✅ 已實現功能詳情

### 1. 餘額管理 ✅

**位置**: `internal/balance/manager.go`

```go
type Manager struct {
    exchange     ExchangeClient
    cache        *BalanceCache
    syncInterval time.Duration
}

// 方法
Lock(amount float64) error     // 鎖定餘額
Unlock(amount float64)         // 解鎖餘額
Deduct(amount float64)         // 扣除 (成交後)
Add(amount float64)            // 增加 (賣出)
GetBalance() Balance           // 取得快照
Sync(ctx context.Context) error // 同步交易所
```

### 2. User Data Stream ✅

**Spot**: `internal/order/user_stream_spot.go`
**Futures**: `internal/order/user_stream_futures.go`

功能：
- Listen Key 管理
- WebSocket 連線
- Execution Report 解析
- 訂單狀態更新
- 事件發布 (`EventOrderFilled`)

### 3. 成交事件 ✅

**定義**: `internal/events/types.go`
```go
EventOrderFilled Event = "order.filled"
```

**發布位置**:
- `executor.go` (同步執行)
- `dry_run.go` (模擬執行)
- `user_stream_spot.go` (Spot 成交回報)
- `user_stream_futures.go` (Futures 成交回報)

### 4. 成交事件訂閱 ✅

**位置**: `main.go:239`
```go
filledSub, unsubFilled := bus.Subscribe(events.EventOrderFilled, 100)
```

---

## ⚠️ 待確認項目

### 風控指標更新

`riskMgr.UpdateMetrics` 方法存在，但調用位置待確認：

```go
// risk/manager.go
func (m *Manager) UpdateMetrics(result TradeResult) error
```

**建議**: 在 `EventOrderFilled` 處理中調用

---

## 💡 優化建議

### 短期
1. 確認 `UpdateMetrics` 調用點
2. 補充對賬機制 (定期)

### 中期
1. 改進錯誤重試機制
2. 增強 WebSocket 重連

---

## 結論

**原分析文檔評估過於保守**。系統已實現大部分關鍵功能：

- ✅ 餘額管理 (Lock/Unlock/Deduct/Add)
- ✅ User Data Stream (Spot + Futures)
- ✅ 成交事件發布與訂閱
- ✅ 部分成交處理

主要缺漏已在後續開發中補完。
