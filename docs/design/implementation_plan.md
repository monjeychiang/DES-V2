# 本地倉位同步 & Dry-Run 設計方案

## 🎯 功能需求

### 1. 本地倉位狀態維護
- 本地緩存倉位狀態
- 定期與交易所同步
- 檢測差異並處理

### 2. Dry-Run 模式
- 模擬訂單執行
- 不真實下單
- 完整流程測試

---

## 📋 設計方案

### Feature 1: 本地倉位同步

#### 架構設計

```
┌─────────────────┐
│ PositionManager │
│  (Local State)  │
└────────┬────────┘
         │
         ├─ In-Memory Cache (快速訪問)
         ├─ Local DB (持久化)
         └─ Periodic Sync (定時同步)
                │
                ▼
         ┌──────────────┐
         │   Exchange   │
         │   Binance    │
         └──────────────┘
```

#### 實現組件

**internal/state/position_sync.go**

```go
type PositionSyncManager struct {
    db          *sql.DB
    exchange    ExchangeClient
    cache       *PositionCache
    syncInterval time.Duration
    mu          sync.RWMutex
}

type PositionCache struct {
    positions map[string]*Position  // symbol -> position
    lastSync  time.Time
    mu        sync.RWMutex
}

type Position struct {
    Symbol        string
    Side          string  // LONG/SHORT
    Quantity      float64
    EntryPrice    float64
    MarkPrice     float64
    UnrealizedPnL float64
    Leverage      float64
    UpdatedAt     time.Time
}

// 創建同步管理器
func NewPositionSyncManager(
    db *sql.DB,
    exchange ExchangeClient,
    syncInterval time.Duration,
) *PositionSyncManager {
    psm := &PositionSyncManager{
        db:           db,
        exchange:     exchange,
        cache:        &PositionCache{positions: make(map[string]*Position)},
        syncInterval: syncInterval,
    }
    
    // 啟動時從 DB 載入
    psm.loadFromDB()
    
    return psm
}

// 啟動定期同步
func (psm *PositionSyncManager) Start(ctx context.Context) {
    ticker := time.NewTicker(psm.syncInterval)
    defer ticker.Stop()
    
    // 立即執行一次同步
    psm.syncWithExchange()
    
    go func() {
        for {
            select {
            case <-ticker.C:
                if err := psm.syncWithExchange(); err != nil {
                    log.Printf("Position sync error: %v", err)
                }
            case <-ctx.Done():
                return
            }
        }
    }()
}

// 與交易所同步
func (psm *PositionSyncManager) syncWithExchange() error {
    psm.mu.Lock()
    defer psm.mu.Unlock()
    
    // 1. 獲取交易所持倉
    exchangePositions, err := psm.exchange.GetPositions()
    if err != nil {
        return fmt.Errorf("get exchange positions: %w", err)
    }
    
    // 2. 對比本地與交易所差異
    diffs := psm.comparePositions(exchangePositions)
    
    // 3. 處理差異
    for _, diff := range diffs {
        switch diff.Type {
        case DiffTypeNew:
            // 交易所有，本地沒有 → 添加
            psm.cache.Add(diff.Position)
            psm.saveToDB(diff.Position)
            
        case DiffTypeClosed:
            // 本地有，交易所沒有 → 刪除
            psm.cache.Remove(diff.Symbol)
            psm.deleteFromDB(diff.Symbol)
            
        case DiffTypeUpdated:
            // 數量/價格不同 → 更新
            psm.cache.Update(diff.Position)
            psm.updateDB(diff.Position)
        }
    }
    
    psm.cache.lastSync = time.Now()
    log.Printf("✓ Position sync completed: %d positions", len(psm.cache.positions))
    
    return nil
}

// 對比倉位
func (psm *PositionSyncManager) comparePositions(
    exchangePos []Position,
) []PositionDiff {
    var diffs []PositionDiff
    
    // 交易所倉位映射
    exMap := make(map[string]*Position)
    for _, pos := range exchangePos {
        exMap[pos.Symbol] = &pos
    }
    
    // 檢查本地倉位
    for symbol, localPos := range psm.cache.positions {
        exPos, exists := exMap[symbol]
        
        if !exists {
            // 本地有但交易所沒有
            diffs = append(diffs, PositionDiff{
                Type:   DiffTypeClosed,
                Symbol: symbol,
            })
        } else if !positionsEqual(localPos, exPos) {
            // 數據不一致
            diffs = append(diffs, PositionDiff{
                Type:     DiffTypeUpdated,
                Symbol:   symbol,
                Position: *exPos,
            })
        }
        
        delete(exMap, symbol)
    }
    
    // 檢查交易所新增的倉位
    for symbol, pos := range exMap {
        diffs = append(diffs, PositionDiff{
            Type:     DiffTypeNew,
            Symbol:   symbol,
            Position: *pos,
        })
    }
    
    return diffs
}

// 快速獲取本地倉位（不訪問交易所）
func (psm *PositionSyncManager) GetPosition(symbol string) (*Position, bool) {
    psm.cache.mu.RLock()
    defer psm.cache.mu.RUnlock()
    
    pos, exists := psm.cache.positions[symbol]
    return pos, exists
}

// 獲取所有倉位
func (psm *PositionSyncManager) GetAllPositions() map[string]*Position {
    psm.cache.mu.RLock()
    defer psm.cache.mu.RUnlock()
    
    // 返回副本
    result := make(map[string]*Position)
    for k, v := range psm.cache.positions {
        result[k] = v
    }
    return result
}

// 本地更新（訂單成交時調用）
func (psm *PositionSyncManager) OnOrderFilled(order FilledOrder) {
    psm.mu.Lock()
    defer psm.mu.Unlock()
    
    pos, exists := psm.cache.positions[order.Symbol]
    
    if !exists {
        // 新開倉
        pos = &Position{
            Symbol:     order.Symbol,
            Side:       order.Side,
            Quantity:   order.Quantity,
            EntryPrice: order.Price,
            UpdatedAt:  time.Now(),
        }
        psm.cache.positions[order.Symbol] = pos
    } else {
        // 加倉或平倉
        if order.Side == pos.Side {
            // 加倉
            totalValue := pos.Quantity*pos.EntryPrice + order.Quantity*order.Price
            pos.Quantity += order.Quantity
            pos.EntryPrice = totalValue / pos.Quantity
        } else {
            // 平倉
            pos.Quantity -= order.Quantity
            if pos.Quantity <= 0 {
                delete(psm.cache.positions, order.Symbol)
                psm.deleteFromDB(order.Symbol)
                return
            }
        }
    }
    
    psm.saveToDB(pos)
}

type PositionDiff struct {
    Type     DiffType
    Symbol   string
    Position Position
}

type DiffType int

const (
    DiffTypeNew     DiffType = iota  // 新增
    DiffTypeClosed                   // 關閉
    DiffTypeUpdated                  // 更新
)
```

#### 配置參數

```go
type SyncConfig struct {
    EnableSync     bool          `yaml:"enable_sync"`
    SyncInterval   time.Duration `yaml:"sync_interval"`   // 30s, 1m, 5m
    OnDiffAction   string        `yaml:"on_diff_action"`  // "log", "alert", "auto_fix"
}

// 默認配置
SyncConfig{
    EnableSync:   true,
    SyncInterval: 30 * time.Second,
    OnDiffAction: "log",  // 僅記錄差異
}
```

---

### Feature 2: Dry-Run 模式

#### 架構設計

```
Strategy Signal
    ↓
【Dry-Run Switch】
    ├─ ON  → MockExecutor (模擬)
    └─ OFF → RealExecutor (真實)
```

#### 實現組件

**internal/order/dry_run.go**

```go
type ExecutionMode int

const (
    ModeProduction ExecutionMode = iota  // 生產模式
    ModeDryRun                            // 模擬模式
)

type DryRunExecutor struct {
    mode       ExecutionMode
    realExec   *Executor           // 真實執行器
    mockExec   *MockExecutor       // 模擬執行器
    recorder   *DryRunRecorder     // 記錄器
}

type MockExecutor struct {
    positions  map[string]*MockPosition
    balance    float64
    orders     []MockOrder
    mu         sync.RWMutex
}

type MockPosition struct {
    Symbol     string
    Side       string
    Quantity   float64
    EntryPrice float64
    PnL        float64
}

type MockOrder struct {
    ID         string
    Symbol     string
    Side       string
    Quantity   float64
    Price      float64
    Status     string
    CreatedAt  time.Time
    FilledAt   *time.Time
}

// 創建 Dry-Run 執行器
func NewDryRunExecutor(mode ExecutionMode, realExec *Executor) *DryRunExecutor {
    return &DryRunExecutor{
        mode:     mode,
        realExec: realExec,
        mockExec: NewMockExecutor(10000.0),  // 初始資金
        recorder: NewDryRunRecorder(),
    }
}

// 執行訂單（根據模式選擇）
func (dre *DryRunExecutor) Execute(ctx context.Context, order Order) error {
    if dre.mode == ModeDryRun {
        // 模擬執行
        return dre.mockExec.Execute(order)
    }
    
    // 真實執行
    return dre.realExec.Handle(ctx, order)
}

// 模擬執行器實現
func (me *MockExecutor) Execute(order Order) error {
    me.mu.Lock()
    defer me.mu.Unlock()
    
    // 1. 驗證餘額
    orderValue := order.Qty * order.Price
    if orderValue > me.balance {
        return fmt.Errorf("insufficient balance: need %.2f, have %.2f", 
            orderValue, me.balance)
    }
    
    // 2. 創建模擬訂單
    mockOrder := MockOrder{
        ID:        uuid.NewString(),
        Symbol:    order.Symbol,
        Side:      order.Side,
        Quantity:  order.Qty,
        Price:     order.Price,
        Status:    "FILLED",  // 立即成交
        CreatedAt: time.Now(),
    }
    now := time.Now()
    mockOrder.FilledAt = &now
    
    me.orders = append(me.orders, mockOrder)
    
    // 3. 更新模擬倉位
    me.updatePosition(mockOrder)
    
    // 4. 更新餘額
    if order.Side == "BUY" {
        me.balance -= orderValue
    } else {
        me.balance += orderValue
    }
    
    log.Printf("🎭 DRY-RUN: %s %s %.4f @ %.2f (Balance: %.2f)",
        order.Side, order.Symbol, order.Qty, order.Price, me.balance)
    
    return nil
}

// 更新模擬倉位
func (me *MockExecutor) updatePosition(order MockOrder) {
    pos, exists := me.positions[order.Symbol]
    
    if !exists {
        me.positions[order.Symbol] = &MockPosition{
            Symbol:     order.Symbol,
            Side:       order.Side,
            Quantity:   order.Quantity,
            EntryPrice: order.Price,
        }
        return
    }
    
    if order.Side == pos.Side {
        // 加倉
        totalValue := pos.Quantity*pos.EntryPrice + order.Quantity*order.Price
        pos.Quantity += order.Quantity
        pos.EntryPrice = totalValue / pos.Quantity
    } else {
        // 平倉
        pos.Quantity -= order.Quantity
        if pos.Quantity <= 0 {
            delete(me.positions, order.Symbol)
        }
    }
}

// 獲取模擬倉位
func (me *MockExecutor) GetPositions() map[string]*MockPosition {
    me.mu.RLock()
    defer me.mu.RUnlock()
    
    result := make(map[string]*MockPosition)
    for k, v := range me.positions {
        result[k] = v
    }
    return result
}

// 計算總盈虧
func (me *MockExecutor) GetTotalPnL(currentPrices map[string]float64) float64 {
    me.mu.RLock()
    defer me.mu.RUnlock()
    
    var totalPnL float64
    for symbol, pos := range me.positions {
        currentPrice := currentPrices[symbol]
        pnl := (currentPrice - pos.EntryPrice) * pos.Quantity
        totalPnL += pnl
    }
    
    return totalPnL
}
```

#### Dry-Run 記錄器

```go
type DryRunRecorder struct {
    records []DryRunRecord
    mu      sync.Mutex
}

type DryRunRecord struct {
    Timestamp time.Time
    Action    string  // "ORDER", "FILL", "CANCEL"
    Symbol    string
    Side      string
    Quantity  float64
    Price     float64
    Balance   float64
    PnL       float64
}

func (drr *DryRunRecorder) Record(record DryRunRecord) {
    drr.mu.Lock()
    defer drr.mu.Unlock()
    
    record.Timestamp = time.Now()
    drr.records = append(drr.records, record)
}

// 導出為 CSV
func (drr *DryRunRecorder) ExportCSV(filename string) error {
    drr.mu.Lock()
    defer drr.mu.Unlock()
    
    file, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer file.Close()
    
    writer := csv.NewWriter(file)
    defer writer.Flush()
    
    // Header
    writer.Write([]string{
        "Timestamp", "Action", "Symbol", "Side", 
        "Quantity", "Price", "Balance", "PnL",
    })
    
    // Records
    for _, r := range drr.records {
        writer.Write([]string{
            r.Timestamp.Format(time.RFC3339),
            r.Action,
            r.Symbol,
            r.Side,
            fmt.Sprintf("%.4f", r.Quantity),
            fmt.Sprintf("%.2f", r.Price),
            fmt.Sprintf("%.2f", r.Balance),
            fmt.Sprintf("%.2f", r.PnL),
        })
    }
    
    return nil
}
```

---

## 🔌 系統集成

### Main.go 集成

```go
func main() {
    // ...

    // 1. 創建倉位同步管理器
    positionSyncMgr := state.NewPositionSyncManager(
        database.DB,
        exchangeClient,
        30*time.Second,  // 30秒同步一次
    )
    positionSyncMgr.Start(ctx)
    
    // 2. 設置執行模式
    execMode := order.ModeProduction
    if cfg.DryRun {
        execMode = order.ModeDryRun
        log.Println("🎭 Running in DRY-RUN mode")
    }
    
    // 3. 創建 Dry-Run 執行器
    dryRunExec := order.NewDryRunExecutor(execMode, exec)
    
    // 4. 訂單處理使用 Dry-Run 執行器
    go orderQueue.Drain(ctx, func(o order.Order) {
        dryRunExec.Execute(ctx, o)
    })
    
    // 5. 訂單成交後更新本地倉位
    orderFilledStream := bus.Subscribe(events.EventOrderFilled, 100)
    go func() {
        for msg := range orderFilledStream {
            filled := msg.(order.FilledOrder)
            positionSyncMgr.OnOrderFilled(filled)
        }
    }()
}
```

### 配置文件

```yaml
# config/config.yaml

# Dry-Run 模式
dry_run: true  # true=模擬, false=真實

# 倉位同步
position_sync:
  enable: true
  interval: 30s
  on_diff_action: "log"  # log, alert, auto_fix

# Dry-Run 設置
dry_run_config:
  initial_balance: 10000.0
  record_trades: true
  export_csv: true
  csv_path: "dry_run_results.csv"
```

---

## 📊 使用場景

### 場景 1: 開發測試

```yaml
dry_run: true
position_sync:
  enable: false  # 不需要同步
```

**效果**: 
- 完全模擬執行
- 不連接交易所
- 快速測試策略邏輯

### 場景 2: 策略驗證

```yaml
dry_run: true
position_sync:
  enable: true
  interval: 1m
```

**效果**:
- 使用實時數據
- 模擬執行訂單
- 驗證策略效果

### 場景 3: 生產運行

```yaml
dry_run: false
position_sync:
  enable: true
  interval: 30s
  on_diff_action: "alert"
```

**效果**:
- 真實下單
- 實時同步倉位
- 檢測異常

---

## ✅ 優勢

### 倉位同步
- ✅ 快速本地訪問
- ✅ 減少 API 調用
- ✅ 自動檢測差異
- ✅ 異常告警

### Dry-Run
- ✅ 安全測試
- ✅ 無風險驗證
- ✅ 完整記錄
- ✅ 性能分析

---

## 🎯 實施優先級

1. **Dry-Run 模式** (2-3小時)
   - 基礎功能
   - 最小可用

2. **倉位同步** (3-4小時)
   - 本地緩存
   - 定期同步
   - 差異處理

**總時間**: 5-7小時
