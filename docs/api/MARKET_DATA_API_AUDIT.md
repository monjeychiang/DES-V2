# 市場數據 API 錯誤檢查報告

**檢查日期**: 2025-11-27  
**檢查範圍**: `pkg/binance/` 所有市場數據相關 API (REST + WebSocket)

---

## 🔍 發現的問題總結

### 嚴重性分級
- 🔴 **Critical**: 導致功能無法使用或數據錯誤
- 🟡 **Warning**: 功能不完整但可用
- 🟢 **Info**: 優化建議

---

## 📊 REST API 問題 (`rest.go`)

### 1. 🟡 GetKlines - 缺少可選參數

**當前實現**:
```go
func (c *Client) GetKlines(symbol, interval string, limit int) ([]Kline, error)
```

**問題**:
- ❌ 缺少 `startTime` 參數 (指定起始時間)
- ❌ 缺少 `endTime` 參數 (指定結束時間)
- ❌ 缺少 `timeZone` 參數 (時區支持)

**影響**:
- 無法獲取指定時間範圍的歷史數據
- 只能獲取最近的 N 根 K線
- 回測系統無法正常工作

**官方 API 參數**:
```
symbol (required):  BTCUSDT
interval (required): 1m, 5m, 1h, 1d, etc.
startTime (optional): UNIX timestamp (ms)
endTime (optional):   UNIX timestamp (ms)
limit (optional):     max 1000, default 500
timeZone (optional):  e.g., "+8:00", default "0" (UTC)
```

**建議修復**:
```go
type KlineParams struct {
    Symbol    string
    Interval  string
    StartTime int64  // optional
    EndTime   int64  // optional
    Limit     int    // optional
    TimeZone  string // optional
}

func (c *Client) GetKlines(params KlineParams) ([]Kline, error)
```

---

### 2. 🟡 Kline 數據結構不完整

**當前 Kline 結構** (`types.go`):
```go
type Kline struct {
    OpenTime  int64
    Open      float64
    High      float64
    Low       float64
    Close     float64
    Volume    float64
    CloseTime int64
}
```

**缺少的字段**:
根據官方文檔，Binance K線返回 **12 個字段**，當前只解析了 **7 個**:

| 索引 | 官方字段名 | 當前實現 | 狀態 |
|------|-----------|---------|------|
| 0 | Open time | ✅ OpenTime | ✅ |
| 1 | Open | ✅ Open | ✅ |
| 2 | High | ✅ High | ✅ |
| 3 | Low | ✅ Low | ✅ |
| 4 | Close | ✅ Close | ✅ |
| 5 | Volume | ✅ Volume | ✅ |
| 6 | Close time | ✅ CloseTime | ✅ |
| 7 | **Quote asset volume** | ❌ | **缺少** |
| 8 | **Number of trades** | ❌ | **缺少** |
| 9 | **Taker buy base volume** | ❌ | **缺少** |
| 10 | **Taker buy quote volume** | ❌ | **缺少** |
| 11 | Unused | ❌ | 可忽略 |

**影響**:
- 無法計算成交量分析指標
- 無法區分主動買入和賣出量
- 策略無法使用 VWAP 等高級指標

**建議修復**:
```go
type Kline struct {
    OpenTime             int64
    Open                 float64
    High                 float64
    Low                  float64
    Close                float64
    Volume               float64   // Base asset volume
    CloseTime            int64
    QuoteVolume          float64   // Quote asset volume (NEW)
    NumberOfTrades       int       // Trade count (NEW)
    TakerBuyBaseVolume   float64   // Taker buy base volume (NEW)
    TakerBuyQuoteVolume  float64   // Taker buy quote volume (NEW)
}
```

---

### 3. 🟡 缺少 GetServerTime 實現

**現狀**: `rest.go` 沒有實現服務器時間獲取

**影響**:
- 簽名 API 需要精確時間戳
- 無法同步本地時間與服務器時間
- 可能導致簽名失效 (時間偏移 > recvWindow)

**注意**: `market_data.go` 有實現 `ServerTime()` 方法，但 `Client` 結構體在 `rest.go` 中定義，兩者不同步。

**建議**: 統一使用 `MarketDataClient` 或在 `Client` 中添加方法。

---

## 🌐 WebSocket API 問題 (`websocket.go`)

### 4. ✅ WebSocket K線流 - 實現正確

**檢查項目**:
- ✅ WebSocket 連接 URL 正確 (`wss://stream.binance.com:9443/ws`)
- ✅ 流名稱格式正確 (`{symbol}@kline_{interval}`)
- ✅ 消息解析正確 (解析 `k` 對象內的字段)
- ✅ 字段映射正確 (`t`, `T`, `o`, `c`, `h`, `l`, `v`)
- ✅ 錯誤處理完善 (連接關閉檢測)
- ✅ 使用 `sync.Once` 防止重複關閉

**唯一建議**:
- 🟢 Symbol 應該小寫: 根據官方文檔 "All symbols for streams are **lowercase**"

**當前**:
```go
stream := fmt.Sprintf("%s@kline_%s", symbol, interval)
```

**建議**:
```go
stream := fmt.Sprintf("%s@kline_%s", strings.ToLower(symbol), interval)
```

---

### 5. 🟡 缺少其他市場數據流

**當前實現**:
- ✅ Kline Stream (`@kline_<interval>`)

**缺少的常用流**:
- ❌ Trade Stream (`@trade`) - 逐筆成交
- ❌ Ticker Stream (`@ticker`) - 24小時價格統計
- ❌ Book Ticker Stream (`@bookTicker`) - 最優買賣價
- ❌ Depth Stream (`@depth` / `@depth<level>@<update_speed>`) - 深度數據

**影響**:
- 無法獲取實時成交數據
- 無法獲取最優買賣價 (用於滑點計算)
- 無法實現高頻交易策略

---

## 📦 market_data.go 問題

### 6. ✅ MarketDataClient - 實現良好

**已實現方法**:
- ✅ `Ping()` - 連接測試
- ✅ `ServerTime()` - 服務器時間
- ✅ `ExchangeInfo()` - 交易規則
- ✅ `Depth()` - 訂單簿
- ✅ `Klines()` - K線數據

**問題**: 與 `rest.go` 的 `Client` 重複

**建議**: 
1. 保留 `MarketDataClient` (功能更完整)
2. 廢棄或重構 `rest.go` 的 `Client`
3. 統一命名和接口

---

## 🔧 優先級修復建議

### 🔴 Priority 1 - 必須修復

1. **添加 GetKlines 時間範圍參數**
   - 添加 `startTime` 和 `endTime`
   - 支持回測系統

2. **完善 Kline 數據結構**
   - 添加缺少的 5 個字段
   - 支持高級技術分析

### 🟡 Priority 2 - 建議修復

3. **統一 REST 客戶端**
   - 合併 `Client` 和 `MarketDataClient`
   - 避免混淆

4. **添加更多 WebSocket 流**
   - Trade Stream (高優先級)
   - BookTicker Stream (高優先級)
   - Depth Stream

### 🟢 Priority 3 - 優化

5. **Symbol 小寫轉換**
   - 在 WebSocket 訂閱時自動轉小寫

6. **錯誤處理增強**
   - 添加 API 限流檢測
   - 返回更詳細的錯誤信息

---

## ✅ 正確的部分

以下實現是正確的，無需修改：

1. ✅ WebSocket 連接管理
2. ✅ K線消息解析邏輯
3. ✅ Context 取消傳播
4. ✅ Channel 緩衝設計
5. ✅ 類型轉換函數 (`toFloat`, `toInt64`)
6. ✅ HTTP 超時設置
7. ✅ 錯誤檢查和日誌記錄

---

## 📝 總結

### 主要問題
1. **GetKlines 缺少時間範圍參數** - 阻塞回測功能
2. **Kline 數據不完整** - 影響技術分析
3. **WebSocket 流類型有限** - 無法支持高頻策略
4. **REST 客戶端重複** - 代碼混亂

### 建議行動
1. 優先修復 GetKlines 參數
2. 完善 Kline 數據結構
3. 統一 REST 客戶端接口
4. 添加 Trade 和 BookTicker 流

### 當前狀態
- ✅ 基礎功能可用 (獲取最近 K線)
- ⚠️ 不支持歷史時間範圍查詢
- ⚠️ 數據結構不完整
- ⚠️ WebSocket 流類型有限

**整體評價**: 基礎實現正確，但功能不完整，需要增強以支持完整的交易系統。
