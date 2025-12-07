# 交易 API 錯誤檢查報告

**檢查日期**: 2025-11-27  
**檢查範圍**: 所有交易相關 API (現貨 + U本位合約 + 幣本位合約)

---

## 🔍 發現的問題總結

### 嚴重性分級
- 🔴 **Critical**: 導致無法交易或安全問題
- 🟡 **Warning**: 功能不完整但可基本使用
- 🟢 **Info**: 優化建議

---

## ✅ 正確的部分 (先肯定)

### 簽名實現 - 完全正確 ✅

查看 `pkg/exchange/binance/binance.go` 第 181-184 行：

```go
func sign(data, secret string) string {
    h := hmac.New(sha256.New, []byte(secret))
    h.Write([]byte(data))
    return hex.EncodeToString(h.Sum(nil))
}
```

**驗證結果**:
- ✅ 使用 HMAC-SHA256 算法（符合官方規範）
- ✅ Hex 編碼輸出
- ✅ 簽名邏輯完全正確

### 簽名請求處理 - 正確 ✅

`doSigned()` 方法處理：
- ✅ 正確添加 `timestamp` 和 `recvWindow`
- ✅ GET/DELETE 請求簽名在 query string
- ✅ POST 請求簽名在 request body
- ✅ X-MBX-APIKEY header 正確設置

### 基礎訂單功能 - 可用 ✅

- ✅ 市價單 (MARKET)
- ✅ 限價單 (LIMIT)
- ✅ TimeInForce (GTC/IOC/FOK)
- ✅ 下單 (SubmitOrder)
- ✅ 撤單 (CancelOrder)
- ✅ 查詢訂單 (GetOrder, GetOpenOrders, GetAllOrders)
- ✅ 賬戶查詢 (GetAccountInfo)

---

## 🟡 現貨交易 API 問題 (`pkg/exchange/binance/`)

### 1. 🟡 缺少止損/止盈訂單類型

**當前支持**:
- ✅ MARKET
- ✅ LIMIT

**缺少的訂單類型**:
- ❌ `STOP_LOSS` - 止損市價單
- ❌ `STOP_LOSS_LIMIT` - 止損限價單
- ❌ `TAKE_PROFIT` - 止盈市價單
- ❌ `TAKE_PROFIT_LIMIT` - 止盈限價單
- ❌ `LIMIT_MAKER` - 只做 Maker 的限價單

**影響**:
- 無法設置自動止損/止盈
- 需要手動監控價格並下單
- 策略風控能力受限

**官方參數** (止損限價單示例):
```
type: STOP_LOSS_LIMIT
price: 69500         # 限價價格
stopPrice: 70000     # 觸發價格
timeInForce: GTC
```

---

### 2. 🟡 OrderRequest 缺少字段

**當前 OrderRequest** (`types.go`):
```go
type OrderRequest struct {
    Symbol      string
    Side        Side
    Type        OrderType
    Qty         float64
    Price       float64
    TimeInForce TimeInForce
    ClientID    string
    ReduceOnly  bool
    PositionSide string
    Market       MarketType
    Leverage     int
}
```

**缺少字段**:
- ❌ `StopPrice` (float64) - 止損/止盈觸發價格
- ❌ `IcebergQty` (float64) - 冰山訂單可見數量
- ❌ `StopLimitPrice` (float64) - 止損限價單的限價
- ❌ `StopLimitTimeInForce` (TimeInForce) - 止損限價單的 TIF

---

### 3. 🟡 缺少冰山訂單支持

**什麼是冰山訂單**:
- 將大單分成多個小單
- 僅顯示部分數量在訂單簿
- 隱藏真實交易意圖

**缺少參數**:
- `icebergQty` - 可見數量

**使用限制**:
- 冰山訂單必須是 LIMIT 類型
- timeInForce 必須是 GTC

---

### 4. 🟢 缺少批量撤單方法

**當前**:
```go
CancelOrder(ctx, symbol, exchangeOrderID) // 單個撤單
```

**缺少**:
```go
CancelAllOpenOrders(ctx, symbol) // 撤銷所有掛單
```

**影響**: 
- 緊急情況需要逐個撤單
- 效率較低

**官方端點**: `DELETE /api/v3/openOrders`

---

### 5. 🟡 缺少 User Data Stream

**問題**: 沒有實現實時訂單更新推送

**官方流程**:
1. POST `/api/v3/userDataStream` 創建 Listen Key
2. WebSocket 連接 `wss://stream.binance.com:9443/ws/<listenKey>`
3. 接收訂單更新事件

**事件類型**:
- `executionReport` - 訂單狀態變化
- `outboundAccountPosition` - 賬戶餘額變化
- `balanceUpdate` - 餘額更新

**影響**:
- 只能通過輪詢查詢訂單狀態
- 無法實時響應成交
- 增加 API 調用次數

---

## 🟡 期貨交易 API 問題 (`pkg/exchange/binancefut/`)

### 6. 🟡 期貨止損訂單缺少參數

**期貨特有參數** (缺少):
- ❌ `workingType` - 觸發價格類型 (`MARK_PRICE` / `CONTRACT_PRICE`)
- ❌ `priceProtect` - 價格保護 (TRUE/FALSE)
- ❌ `priceMatch` - 價格匹配模式
- ❌ `selfTradePreventionMode` - 自成交防止

**官方參數說明**:
- `workingType`: 默認為 `CONTRACT_PRICE`，可選 `MARK_PRICE`
- `priceProtect`: 防止標記價格和合約價格差異過大時觸發

---

### 7. 🟢 缺少批量下單

**當前**: 單個下單
**缺少**: 批量下單 (`POST /fapi/v1/batchOrders`)

**優勢**:
- 減少網絡延遲
- 快速建立多個持倉
- 降低 API 權重消耗

**限制**: 最多 5 個訂單/請求

---

### 8. 🟢 缺少條件單 (Conditional Orders)

**期貨支持的高級訂單**:
- ❌ `TRAILING_STOP_MARKET` - 跟蹤止損
- ❌ 激活價格 (`activationPrice`)
- ❌ 回調率 (`callbackRate`)

**用途**: 
- 自動跟隨價格移動止損位
- 鎖定利潤

---

## 🔴 關鍵安全問題

### 9. 🔴 缺少訂單速率限制檢測

**問題**: 
- 沒有檢測 API 返回的 `X-MBX-USED-WEIGHT` header
- 沒有本地限流機制

**風險**:
- 超過速率限制導致 IP 被封禁
- 官方限制: 
  - 現貨: 1200 weight / 分鐘
  - 合約: 2400 weight / 分鐘

**建議**: 
```go
// 在 doSigned 中檢查響應頭
usedWeight := res.Header.Get("X-MBX-USED-WEIGHT-1M")
if weight, _ := strconv.Atoi(usedWeight); weight > 1000 {
    log.Warn("Approaching rate limit: ", weight)
}
```

---

### 10. 🟡 缺少時間同步檢查

**問題**: 
- 每次請求都使用 `time.Now().UnixMilli()`
- 如果本地時間與服務器偏移 > recvWindow，簽名失敗

**建議**: 
- 定期調用 `GetServerTime()` 計算偏移
- 使用 `serverTime + offset` 而非本地時間

**當前代碼風險** (line 73, 103, 212...):
```go
params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
// 如果本地時間快/慢，可能導致 -1021 錯誤 (Timestamp outside recvWindow)
```

---

## 📊 完整性對比表

| 功能 | 現貨 | U合約 | 幣合約 | 官方支持 |
|------|------|-------|--------|---------|
| **基礎訂單** |
| MARKET | ✅ | ✅ | ✅ | ✅ |
| LIMIT | ✅ | ✅ | ✅ | ✅ |
| **止損止盈** |
| STOP_LOSS | ❌ | ❌ | ❌ | ✅ |
| STOP_LOSS_LIMIT | ❌ | ❌ | ❌ | ✅ |
| TAKE_PROFIT | ❌ | ❌ | ❌ | ✅ |
| TAKE_PROFIT_LIMIT | ❌ | ❌ | ❌ | ✅ |
| **高級訂單** |
| Iceberg | ❌ | ❌ | ❌ | ✅ |
| TRAILING_STOP | ❌ | ❌ | ❌ | ✅ (期貨) |
| **操作** |
| 撤單 | ✅ | ✅ | ✅ | ✅ |
| 批量撤單 | ❌ | ❌ | ❌ | ✅ |
| 批量下單 | ❌ | ❌ | ❌ | ✅ (期貨) |
| **查詢** |
| 查詢訂單 | ✅ | ✅ | ✅ | ✅ |
| 查詢持倉 | N/A | ✅ | ✅ | ✅ |
| 查詢賬戶 | ✅ | ✅ | ✅ | ✅ |
| **實時更新** |
| User Data Stream | ❌ | ❌ | ❌ | ✅ |
| **風控** |
| 速率限制檢測 | ❌ | ❌ | ❌ | 建議 |
| 時間同步 | ⚠️ | ⚠️ | ⚠️ | 建議 |

---

## 🎯 優先級修復建議

### 🔴 Priority 1 - 安全與穩定性

1. **添加速率限制檢測**
   - 解析 `X-MBX-USED-WEIGHT` header
   - 接近限制時延遲請求

2. **時間同步機制**
   - 定期同步服務器時間
   - 使用偏移量修正本地時間

### 🟡 Priority 2 - 核心功能

3. **添加止損/止盈訂單**
   - 擴展 OrderType 枚舉
   - 添加 StopPrice 字段
   - 實現 STOP_LOSS_LIMIT 下單邏輯

4. **User Data Stream**
   - 創建/續期 Listen Key
   - WebSocket 訂閱
   - 解析訂單更新事件

5. **批量撤單**
   - 實現 `CancelAllOpenOrders()`

### 🟢 Priority 3 - 高級功能

6. **冰山訂單**
   - 添加 IcebergQty 參數

7. **批量下單** (期貨)
   - 實現 `SubmitBatchOrders()`

8. **跟蹤止損** (期貨)
   - TRAILING_STOP_MARKET 類型
   - 激活價格和回調率

---

## 📝 程式碼修復示例

### 擴展 OrderType

```go
// pkg/exchange/types.go
const (
    OrderTypeMarket          OrderType = "MARKET"
    OrderTypeLimit           OrderType = "LIMIT"
    OrderTypeStopLoss        OrderType = "STOP_LOSS"
    OrderTypeStopLossLimit   OrderType = "STOP_LOSS_LIMIT"
    OrderTypeTakeProfit      OrderType = "TAKE_PROFIT"
    OrderTypeTakeProfitLimit OrderType = "TAKE_PROFIT_LIMIT"
    OrderTypeLimitMaker      OrderType = "LIMIT_MAKER"
)
```

### 擴展 OrderRequest

```go
type OrderRequest struct {
    Symbol       string
    Side         Side
    Type         OrderType
    Qty          float64
    Price        float64
    StopPrice    float64    // NEW: 止損/止盈觸發價
    TimeInForce  TimeInForce
    IcebergQty   float64    // NEW: 冰山訂單可見數量
    ClientID     string
    ReduceOnly   bool
    PositionSide string
    WorkingType  string     // NEW: MARK_PRICE / CONTRACT_PRICE
    PriceProtect bool       // NEW: 價格保護
}
```

### 下單邏輯修改

```go
// pkg/exchange/binance/binance.go SubmitOrder 方法
if req.Type == exchange.OrderTypeLimit || 
   req.Type == exchange.OrderTypeStopLossLimit ||
   req.Type == exchange.OrderTypeTakeProfitLimit {
    params.Set("price", formatFloat(req.Price))
    params.Set("timeInForce", string(toBinanceTIF(req.TimeInForce)))
}

if req.Type == exchange.OrderTypeStopLoss ||
   req.Type == exchange.OrderTypeStopLossLimit ||
   req.Type == exchange.OrderTypeTakeProfit ||
   req.Type == exchange.OrderTypeTakeProfitLimit {
    params.Set("stopPrice", formatFloat(req.StopPrice))
}

if req.IcebergQty > 0 {
    params.Set("icebergQty", formatFloat(req.IcebergQty))
}
```

---

## 📊 總結

### 整體評價: **B+ (85分)**

**優點**:
- ✅ 簽名算法完全正確
- ✅ 基礎交易功能可用
- ✅ 代碼結構清晰

**不足**:
- ⚠️ 缺少高級訂單類型 (止損/止盈)
- ⚠️ 缺少實時訂單更新 (User Data Stream)
- ⚠️ 缺少安全機制 (速率限制、時間同步)
- ⚠️ 功能覆蓋率約 60% (基礎訂單+查詢)

**建議**:
優先實現止損訂單和 User Data Stream，這是實戰必需的功能。冰山訂單和批量操作可延後。
