# 交易所代碼結構優化方案

**目標**: 讓代碼結構更清晰，方便未來添加其他交易所

---

## 📊 現狀分析

### 當前目錄結構
```
pkg/
├── binance/                    # 市場數據 (公開API)
│   ├── market_data.go
│   ├── rest.go
│   ├── websocket.go
│   └── types.go
│
└── exchange/
    ├── types.go                # 通用接口定義
    ├── gateway.go
    ├── ratelimit.go
    │
    ├── binance/                # Binance 現貨交易
    │   ├── binance.go
    │   ├── timesync.go
    │   ├── servertime.go
    │   └── user_data_stream.go
    │
    └── binancefut/             # Binance 期貨交易
        ├── binance_usdt.go     # USDT-M
        ├── binance_coin.go     # COIN-M
        ├── timesync.go
        ├── servertime.go
        ├── user_data_stream.go
        ├── config.go
        ├── helpers.go
        └── types_shared.go
```

### 問題點

1. **命名不一致**
   - `binance/` vs `binancefut/`
   - `binance.go` vs `binance_usdt.go`

2. **職責混淆**
   - `pkg/binance/` 是市場數據
   - `pkg/exchange/binance/` 是交易API
   - 不直觀

3. **期貨合併**
   - USDT-M 和 COIN-M 在同一個包
   - 文件名帶後綴 `_usdt`, `_coin`

4. **擴展困難**
   - 添加新交易所時沒有明確模式
   - 不清楚哪些代碼可復用

---

## ✨ 優化方案

### 方案 A: 按市場類型分層 (推薦)

```
pkg/
└── exchanges/                  # 所有交易所統一目錄
    │
    ├── common/                 # 共用組件
    │   ├── types.go            # 通用類型 (OrderRequest, OrderResult)
    │   ├── gateway.go          # Gateway 接口定義
    │   ├── ratelimit.go        # RateLimiter 通用實現
    │   ├── timesync.go         # TimeSync 通用實現
    │   └── errors.go           # 統一錯誤處理
    │
    ├── binance/                # Binance 交易所
    │   │
    │   ├── common/             # Binance 共用
    │   │   ├── auth.go         # 簽名邏輯
    │   │   ├── client.go       # HTTP 客戶端基礎
    │   │   └── types.go        # Binance 特有類型
    │   │
    │   ├── spot/               # 現貨市場
    │   │   ├── client.go       # 主客戶端
    │   │   ├── orders.go       # 訂單操作
    │   │   ├── account.go      # 賬戶查詢
    │   │   ├── market_data.go  # 市場數據 (REST)
    │   │   ├── websocket.go    # WebSocket (市場+用戶)
    │   │   └── streams.go      # User Data Stream
    │   │
    │   ├── futures_usdt/       # USDT-M 期貨
    │   │   ├── client.go
    │   │   ├── orders.go
    │   │   ├── account.go
    │   │   ├── positions.go    # 持倉管理
    │   │   ├── leverage.go     # 槓桿/保證金
    │   │   └── streams.go
    │   │
    │   └── futures_coin/       # COIN-M 期貨
    │       ├── client.go
    │       ├── orders.go
    │       ├── account.go
    │       ├── positions.go
    │       ├── leverage.go
    │       └── streams.go
    │
    ├── okx/                    # OKX 交易所 (範例)
    │   ├── common/
    │   ├── spot/
    │   ├── futures/
    │   └── swap/
    │
    └── bybit/                  # Bybit 交易所 (範例)
        ├── common/
        ├── spot/
        └── derivatives/
```

### 方案 B: 按功能分類 (備選)

```
pkg/
└── exchanges/
    ├── interfaces/             # 通用接口
    │   └── gateway.go
    │
    ├── binance/
    │   ├── config.go
    │   ├── spot.go             # 一個文件包含所有邏輯
    │   ├── futures_usdt.go
    │   ├── futures_coin.go
    │   ├── market_data.go      # 市場數據獨立
    │   └── websocket.go
    │
    └── okx/
        ├── spot.go
        └── futures.go
```

---

## 📝 推薦方案詳細設計 (方案 A)

### 1. 文件職責劃分

#### `exchanges/common/`
- **types.go**: 所有交易所通用的類型
  ```go
  type OrderRequest struct {...}
  type OrderResult struct {...}
  type Gateway interface {...}
  ```

- **ratelimit.go**: 通用速率限制器
- **timesync.go**: 通用時間同步
- **errors.go**: 統一錯誤定義

#### `exchanges/binance/common/`
- **auth.go**: HMAC 簽名實現
- **client.go**: HTTP 基礎客戶端
  ```go
  type BaseClient struct {
      apiKey    string
      apiSecret string
      httpClient *http.Client
      timeSync   *timesync.TimeSync
      rateLimiter *ratelimit.RateLimiter
  }
  
  func (c *BaseClient) DoSigned(...)
  ```

- **types.go**: Binance 特有類型
  ```go
  type BinanceOrderResponse struct {...}
  type BinanceError struct {...}
  ```

#### `exchanges/binance/spot/`
- **client.go**: 現貨客戶端
  ```go
  type SpotClient struct {
      *common.BaseClient
      baseURL string
  }
  
  func NewSpotClient(cfg Config) *SpotClient
  ```

- **orders.go**: 訂單相關
  ```go
  func (c *SpotClient) SubmitOrder(...)
  func (c *SpotClient) CancelOrder(...)
  func (c *SpotClient) CancelAllOrders(...)
  ```

- **account.go**: 賬戶相關
  ```go
  func (c *SpotClient) GetAccountInfo(...)
  func (c *SpotClient) GetBalances(...)
  ```

- **market_data.go**: 市場數據
  ```go
  func (c *SpotClient) GetKlines(...)
  func (c *SpotClient) GetTicker(...)
  ```

- **websocket.go**: WebSocket 市場數據
  ```go
  func (c *SpotClient) SubscribeKlines(...)
  func (c *SpotClient) SubscribeTrades(...)
  ```

- **streams.go**: User Data Stream
  ```go
  func (c *SpotClient) CreateListenKey(...)
  func (c *SpotClient) SubscribeUserData(...)
  ```

### 2. 命名規範

#### 目錄命名
- 交易所名稱：小寫 (`binance`, `okx`, `bybit`)
- 市場類型：
  - 現貨：`spot`
  - USDT本位期貨：`futures_usdt`
  - 幣本位期貨：`futures_coin`
  - 永續合約：`perpetual`

#### 文件命名
- `client.go` - 主客戶端
- `orders.go` - 訂單操作
- `account.go` - 賬戶查詢
- `positions.go` - 持倉管理 (期貨)
- `leverage.go` - 槓桿管理 (期貨)
- `market_data.go` - 市場數據
- `websocket.go` - WebSocket 訂閱
- `streams.go` - User Data Stream

#### 類型命名
```go
// Client 類型
type SpotClient struct {...}
type FuturesUSDTClient struct {...}
type FuturesCoinClient struct {...}

// 避免
type USDTClient struct {...}  // 不夠明確
type CoinClient struct {...}  // 不夠明確
```

### 3. 導入路徑

```go
// 優化後
import (
    "trading-core/pkg/exchanges/common"
    "trading-core/pkg/exchanges/binance/spot"
    "trading-core/pkg/exchanges/binance/futures_usdt"
    "trading-core/pkg/exchanges/okx/spot"
)

// 使用
spotClient := spot.NewClient(cfg)
futuresClient := futures_usdt.NewClient(cfg)
```

---

## 🔄 遷移計劃

### Phase 1: 創建新結構 (不影響現有代碼)

1. 創建 `pkg/exchanges/` 目錄
2. 實現 `exchanges/common/` 通用組件
3. 實現 `exchanges/binance/common/` 共用邏輯

### Phase 2: 遷移現貨

1. 創建 `exchanges/binance/spot/`
2. 拆分 `exchange/binance/binance.go` 到多個文件
3. 整合 `pkg/binance/` 市場數據到 `spot/`
4. 更新測試

### Phase 3: 遷移期貨

1. 創建 `exchanges/binance/futures_usdt/`
2. 創建 `exchanges/binance/futures_coin/`
3. 拆分 `binancefut/binance_usdt.go`
4. 拆分 `binancefut/binance_coin.go`
5. 提取共用邏輯到 `binance/common/`

### Phase 4: 更新依賴

1. 更新 `main.go` 導入路徑
2. 更新 `internal/order/executor.go`
3. 更新配置

### Phase 5: 清理

1. 刪除舊目錄
2. 更新文檔

---

## 📊 優化收益

### 可讀性提升

**之前**:
```
不清楚 binancefut/binance_usdt.go 是什麼
需要看代碼才知道 pkg/binance 和 pkg/exchange/binance 的區別
```

**之後**:
```
exchanges/binance/futures_usdt/client.go - 清晰！
exchanges/binance/spot/market_data.go - 一目了然！
```

### 擴展性提升

**添加新交易所 (OKX)**:

```bash
# 只需複製結構
mkdir -p pkg/exchanges/okx/{common,spot,futures}

# 實現相同接口
cp exchanges/binance/spot/client.go exchanges/okx/spot/
# 修改實現...
```

### 代碼復用

**共用組件**:
- ✅ TimeSync - 所有交易所通用
- ✅ RateLimiter - 所有交易所通用
- ✅ WebSocket 框架 - 可抽象
- ✅ 簽名邏輯 - 每個交易所獨立

### 職責清晰

```
Client 職責:
├── client.go      → 初始化、配置
├── orders.go      → 訂單 CRUD
├── account.go     → 賬戶查詢
├── market_data.go → 行情數據
└── streams.go     → 實時訂閱
```

---

## 🎯 實施建議

### 立即優化 (推薦)

```
exchanges/
├── common/           # 新建
├── binance/
│   ├── common/       # 提取共用
│   ├── spot/         # 重組現有代碼
│   ├── futures_usdt/ # 重組現有代碼
│   └── futures_coin/ # 重組現有代碼
```

優點：
- 結構清晰
- 方便擴展
- 職責明確

缺點：
- 需要修改導入路徑
- 一次性工作量較大

### 漸進優化 (穩妥)

階段 1: 僅重命名
```
exchange/binance/     → exchanges/binance/spot/
exchange/binancefut/  → exchanges/binance/futures/
```

階段 2: 文件拆分
```
spot/
├── client.go       # 從 binance.go 拆分
├── orders.go       # 從 binance.go 拆分
└── account.go      # 從 binance.go 拆分
```

階段 3: 提取共用
```
binance/common/  # 提取 auth, types
```

---

## ❓ 需要決定

1. **遷移策略**
   - [ ] 立即重構 (一次性)
   - [ ] 漸進優化 (分階段)
   - [ ] 保持現狀，僅添加新交易所時使用新結構

2. **目錄結構**
   - [ ] 方案 A: 按市場類型分層 (推薦)
   - [ ] 方案 B: 按功能分類
   - [ ] 其他方案

3. **優先級**
   - [ ] 立即執行
   - [ ] 下個階段
   - [ ] 有新交易所需求時

---

## 📝 next Steps (如果批准)

1. 獲得批准
2. 創建詳細遷移清單
3. 執行 Phase 1 (創建新結構)
4. 逐步遷移
5. 測試驗證
6. 更新文檔
