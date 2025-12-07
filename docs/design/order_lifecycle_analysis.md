# 訂單生命週期分析

## 📋 訂單送出後應該做的處理

### 完整流程

```
訂單送出
  ↓
1. 訂單入隊
  ↓
2. 風控驗證（已完成）
  ↓
3. 訂單執行
  ├─ Dry-Run: 模擬執行
  └─ Production: 真實下單
  ↓
4. 訂單狀態追蹤
  ├─ NEW
  ├─ PENDING
  ├─ FILLED
  ├─ CANCELLED
  └─ REJECTED
  ↓
5. 成交處理
  ├─ 更新持倉狀態
  ├─ 更新餘額
  ├─ 計算 PnL
  └─ 發布事件
  ↓
6. 風控更新
  ├─ 更新每日交易數
  ├─ 更新每日盈虧
  └─ 更新總倉位
  ↓
7. 止損追蹤
  ├─ 註冊 SL/TP
  └─ 開始監控
  ↓
8. 數據持久化
  ├─ 訂單記錄
  ├─ 持倉記錄
  └─ 交易歷史
  ↓
9. 事件通知
  ├─ EventOrderSubmitted
  ├─ EventOrderFilled
  ├─ EventPositionUpdated
  └─ EventRiskMetricsUpdated
```

---

## ✅ 已實現功能

### 1. 訂單入隊 ✅
**位置**: `internal/order/queue.go`
```go
orderQueue := order.NewQueue(200)
orderQueue.Enqueue(order)
```
- ✅ FIFO 隊列
- ✅ 異步處理
- ✅ 緩衝區管理

### 2. 風控驗證 ✅
**位置**: `internal/risk/manager.go`
```go
decision := riskMgr.EvaluateSignal(...)
if !decision.Allowed {
    return  // 拒絕
}
```
- ✅ 每日限制檢查
- ✅ 單筆倉位限制
- ✅ SL/TP 計算

### 3. 訂單執行 ✅
**位置**: `internal/order/executor.go`
```go
exec.Handle(ctx, order)
```
- ✅ 多市場路由
- ✅ 訂單提交
- ✅ 錯誤處理

### 4. Dry-Run 模式 ✅ (剛實現)
**位置**: `internal/order/dry_run.go`
```go
dryRunner.Execute(ctx, order)
```
- ✅ 模式切換
- ✅ 模擬執行

### 5. 數據持久化 ✅
**位置**: `pkg/db/orders.go`
```go
database.CreateOrder(ctx, order)
```
- ✅ 訂單入庫
- ✅ 狀態更新

### 6. 持倉更新 ✅ (部分)
**位置**: `internal/state/manager.go`
```go
stateMgr.RecordFill(ctx, symbol, side, qty, price)
```
- ✅ 倉位計算
- ✅ 平均價格

---

## ❌ 缺失功能

### 1. 訂單狀態追蹤 ❌

**問題**: 沒有完整的狀態機

**需要**:
```go
type OrderStatus string

const (
    StatusNew       OrderStatus = "NEW"
    StatusSubmitted OrderStatus = "SUBMITTED"
    StatusPending   OrderStatus = "PENDING"
    StatusFilled    OrderStatus = "FILLED"
    StatusPartial   OrderStatus = "PARTIALLY_FILLED"
    StatusCancelled OrderStatus = "CANCELLED"
    StatusRejected  OrderStatus = "REJECTED"
    StatusExpired   OrderStatus = "EXPIRED"
)

// 狀態轉換
func (o *Order) UpdateStatus(newStatus OrderStatus) error {
    // 驗證轉換合法性
    // 更新數據庫
    // 發布事件
}
```

---

### 2. 成交回報處理 ❌

**問題**: 沒有監聽交易所的成交回報

**需要**:
```go
// User Data Stream 監聽
func (exec *Executor) listenUserStream() {
    stream := exchange.UserDataStream()
    
    for event := range stream {
        switch e := event.(type) {
        case OrderUpdate:
            exec.handleOrderUpdate(e)
        case ExecutionReport:
            exec.handleExecution(e)
        }
    }
}

func (exec *Executor) handleExecution(report ExecutionReport) {
    // 1. 更新訂單狀態
    order := exec.getOrder(report.OrderID)
    order.Status = report.Status
    order.FilledQty = report.FilledQty
    
    // 2. 更新持倉
    if report.Status == "FILLED" {
        stateMgr.RecordFill(...)
    }
    
    // 3. 發布事件
    bus.Publish(EventOrderFilled, ...)
}
```

---

### 3. 風控指標更新 ❌

**問題**: 交易完成後沒有更新風控指標

**需要**:
```go
// 在訂單成交後
func onOrderFilled(order Order) {
    // 計算本次盈虧
    pnl := calculatePnL(order)
    
    // 更新風控指標
    riskMgr.UpdateMetrics(risk.TradeResult{
        Symbol: order.Symbol,
        Side:   order.Side,
        Size:   order.Qty,
        Price:  order.Price,
        PnL:    pnl,
    })
}
```

**當前狀態**: `UpdateMetrics` 方法存在但**沒有被調用**

---

### 4. 止損追蹤註冊 ❌

**問題**: 訂單成交後沒有自動註冊到 StopLossManager

**當前**: main.go 中在風控階段註冊，但**訂單可能被拒絕或失敗**

**需要**: 在**實際成交後**註冊
```go
func onOrderFilled(order Order, fillPrice float64) {
    // 註冊止損追蹤
    stopLossMgr.AddPosition(risk.StopLossPosition{
        Symbol:         order.Symbol,
        Side:           order.Side,
        EntryPrice:     fillPrice,
        CurrentPrice:   fillPrice,
        StopLoss:       order.StopPrice,
        TakeProfit:     order.ActivationPrice,
        TrailingStop:   config.UseTrailingStop,
        TrailingOffset: config.TrailingPercent,
    })
}
```

---

### 5. 事件發布 ❌ (不完整)

**當前**: 只有風控拒絕事件

**需要**: 完整的訂單事件
```go
// 訂單提交
bus.Publish(EventOrderSubmitted, order)

// 訂單接受
bus.Publish(EventOrderAccepted, order)

// 訂單成交
bus.Publish(EventOrderFilled, FilledOrder{
    OrderID:   order.ID,
    Symbol:    order.Symbol,
    Side:      order.Side,
    Quantity:  order.Qty,
    Price:     fillPrice,
    Fee:       fee,
    Timestamp: time.Now(),
})

// 訂單拒絕
bus.Publish(EventOrderRejected, RejectedOrder{
    OrderID: order.ID,
    Reason:  reason,
})
```

---

### 6. 餘額更新 ❌

**問題**: 沒有追蹤可用餘額和鎖定餘額

**需要**:
```go
type BalanceManager struct {
    total    float64  // 總餘額
    locked   float64  // 鎖定（掛單中）
    available float64 // 可用
}

// 下單時鎖定
func (bm *BalanceManager) Lock(amount float64) error {
    if amount > bm.available {
        return ErrInsufficientBalance
    }
    bm.locked += amount
    bm.available -= amount
    return nil
}

// 成交後釋放
func (bm *BalanceManager) UnlockAndUpdate(locked, actual float64) {
    bm.locked -= locked
    bm.available += (locked - actual)
}
```

---

### 7. 對賬與修正 ❌

**問題**: 沒有與交易所對賬機制

**需要**:
```go
// 定期對賬
func (exec *Executor) reconcile() {
    // 1. 獲取交易所訂單列表
    exchangeOrders := exchange.GetOpenOrders()
    
    // 2. 對比本地訂單
    localOrders := database.GetOpenOrders()
    
    // 3. 找出差異
    for _, local := range localOrders {
        if !existsInExchange(local, exchangeOrders) {
            // 本地有但交易所沒有 → 可能已成交或取消
            // 需要查詢並更新
        }
    }
}
```

---

### 8. 部分成交處理 ❌

**問題**: 沒有處理部分成交情況

**需要**:
```go
type Order struct {
    Qty        float64  // 總量
    FilledQty  float64  // 已成交量
    Status     string   // 狀態
}

// 部分成交更新
func (o *Order) UpdateFill(qty float64) {
    o.FilledQty += qty
    
    if o.FilledQty >= o.Qty {
        o.Status = "FILLED"
    } else {
        o.Status = "PARTIALLY_FILLED"
    }
}
```

---

## 📊 功能完成度

| 功能 | 狀態 | 完成度 |
|------|------|--------|
| 訂單入隊 | ✅ 完成 | 100% |
| 風控驗證 | ✅ 完成 | 100% |
| 訂單執行 | ✅ 完成 | 100% |
| Dry-Run | ✅ 完成 | 100% |
| 數據持久化 | ✅ 完成 | 100% |
| **訂單狀態追蹤** | ⚠️ 簡單 | 30% |
| **成交回報** | ❌ 缺失 | 0% |
| **風控更新** | ⚠️ 方法存在未調用 | 20% |
| **止損註冊** | ⚠️ 時機不對 | 50% |
| **事件發布** | ⚠️ 不完整 | 40% |
| **餘額管理** | ❌ 缺失 | 0% |
| **對賬機制** | ❌ 缺失 | 0% |
| **部分成交** | ❌ 缺失 | 0% |

**總體完成度**: ~50%

---

## 🎯 關鍵缺失

### 最重要的3個缺失

1. **成交回報監聽** ⚠️⚠️⚠️
   - 當前：下單後不知道是否成交
   - 影響：無法及時更新狀態

2. **餘額管理** ⚠️⚠️
   - 當前：不追蹤可用餘額
   - 影響：可能超額下單

3. **對賬機制** ⚠️
   - 當前：本地與交易所可能不一致
   - 影響：數據準確性

---

## 💡 建議優先實施

### Phase 1: 成交回報 (關鍵)
```go
// User Data Stream 集成
func setupUserDataStream() {
    // 監聽訂單更新
    // 更新訂單狀態
    // 觸發後續處理
}
```

### Phase 2: 完整事件流
```go
// 發布所有關鍵事件
OrderSubmitted → OrderAccepted → OrderFilled
         ↓             ↓              ↓
    持倉更新      鎖定資金      釋放資金
                              更新風控
                              註冊止損
```

### Phase 3: 餘額與對賬
```go
// 餘額管理器
// 定期對賬
```

---

## 🔧 快速修復建議

### 1. 補充事件定義
```go
// internal/events/types.go
const (
    EventOrderSubmitted  = "order.submitted"
    EventOrderAccepted   = "order.accepted"
    EventOrderFilled     = "order.filled"
    EventOrderRejected   = "order.rejected"
    EventPositionUpdated = "position.updated"
)
```

### 2. Executor 發布事件
```go
// executor.go Handle() 中
func (e *Executor) Handle(ctx context.Context, o Order) error {
    // 提交
    e.bus.Publish(EventOrderSubmitted, o)
    
    // 執行...
    result, err := gateway.SubmitOrder(...)
    if err != nil {
        e.bus.Publish(EventOrderRejected, ...)
        return err
    }
    
    // 成功
    e.bus.Publish(EventOrderAccepted, result)
    
    // 如果立即成交
    if result.Status == "FILLED" {
        e.bus.Publish(EventOrderFilled, ...)
    }
}
```

### 3. Main.go 監聽成交事件
```go
// 監聽成交事件
filledStream := bus.Subscribe(EventOrderFilled, 100)
go func() {
    for msg := range filledStream {
        filled := msg.(FilledOrder)
        
        // 1. 更新持倉
        stateMgr.RecordFill(ctx, filled.Symbol, filled.Side, filled.Quantity, filled.Price)
        
        // 2. 更新風控
        riskMgr.UpdateMetrics(...)
        
        // 3. 註冊止損（如果是開倉）
        if isOpening(filled) {
            stopLossMgr.AddPosition(...)
        }
    }
}()
```

---

## ✅ 結論

**當前狀態**: 
- 基礎訂單流程完成
- 關鍵的成交後處理**缺失**

**需要補完**:
1. User Data Stream 監聽
2. 完整事件發布
3. 成交後處理流程
4. 餘額與對賬

**建議**: 先補完 User Data Stream，這是實盤運行的**必需功能**
