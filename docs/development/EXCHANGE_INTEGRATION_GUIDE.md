# 交易所串接開發者指南

> **版本**: 1.0  
> **創建日期**: 2025-12-08  
> **適用對象**: 後端開發人員  
> **前置知識**: Go 語言基礎、REST API、WebSocket

---

## 📋 目錄

1. [概述](#概述)
2. [架構設計](#架構設計)
3. [必須實作介面](#必須實作介面)
4. [開發步驟](#開發步驟)
5. [共用元件](#共用元件)
6. [程式碼範例](#程式碼範例)
7. [測試指南](#測試指南)
8. [檢查清單](#檢查清單)
9. [最佳實踐](#最佳實踐)
10. [附錄](#附錄)

---

## 概述

### 1.1 目的

本文檔定義了 DES Trading System 新增交易所支援的標準流程和規範，確保：
- 統一的介面設計
- 一致的錯誤處理
- 可複用的共用元件
- 易於維護和擴展

### 1.2 現有支援交易所

| 交易所 | 市場類型 | 實作路徑 |
|--------|----------|----------|
| Binance | 現貨 | `pkg/exchanges/binance/spot/` |
| Binance | USDT-M 合約 | `pkg/exchanges/binance/futures_usdt/` |
| Binance | COIN-M 合約 | `pkg/exchanges/binance/futures_coin/` |

### 1.3 目錄結構

```
pkg/exchanges/
├── common/                    # 共用介面和工具
│   ├── gateway.go            # Gateway 介面定義
│   ├── types.go              # 共用類型定義
│   ├── ratelimit.go          # 速率限制器
│   └── timesync.go           # 時間同步器
├── binance/                   # Binance 實作
│   ├── common/               # Binance 共用
│   ├── spot/                 # 現貨
│   ├── futures_usdt/         # USDT-M
│   └── futures_coin/         # COIN-M
└── <new_exchange>/           # 新交易所 (你要創建的)
    ├── common/               # 交易所內共用
    ├── spot/                 # 現貨 (如適用)
    └── futures/              # 期貨 (如適用)
```

---

## 架構設計

### 2.1 核心架構圖

```
┌─────────────────────────────────────────────────────────────────┐
│                        DES Trading Core                         │
│                                                                 │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐        │
│  │   Strategy  │───▶│    Risk     │───▶│   Order     │        │
│  │   Engine    │    │   Manager   │    │   Executor  │        │
│  └─────────────┘    └─────────────┘    └──────┬──────┘        │
│                                               │                │
└───────────────────────────────────────────────┼────────────────┘
                                                │
                    ┌───────────────────────────▼───────────────────────────┐
                    │                  common.Gateway                       │
                    │          (統一交易介面 - 你需要實作這個)               │
                    └───────────────────────────┬───────────────────────────┘
                                                │
        ┌───────────────┬───────────────┬───────┴───────┬───────────────┐
        ▼               ▼               ▼               ▼               ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│Binance Spot  │ │Binance USDT  │ │Binance COIN  │ │  OKX Client  │ │  Bybit Client │
│   Client     │ │   Client     │ │   Client     │ │  (新增)      │ │  (新增)       │
└──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘
```

### 2.2 設計原則

1. **介面隔離**: 只實作系統需要的方法
2. **錯誤包裝**: 返回有意義的錯誤訊息
3. **狀態標準化**: 將交易所狀態對應到 `common.OrderStatus`
4. **配置外部化**: API 金鑰等敏感資訊從環境變數讀取
5. **可測試性**: 支援 Mock 和 Testnet

---

## 必須實作介面

### 3.1 核心介面: `common.Gateway`

```go
// pkg/exchanges/common/gateway.go
package common

import "context"

// Gateway abstracts a trading venue.
// 這是你必須實作的最小介面
type Gateway interface {
    // SubmitOrder 提交訂單到交易所
    SubmitOrder(ctx context.Context, req OrderRequest) (OrderResult, error)
    
    // CancelOrder 取消訂單
    CancelOrder(ctx context.Context, symbol, exchangeOrderID string) error
}
```

### 3.2 建議實作的擴展介面

```go
// 擴展介面 (建議實作)
type ExtendedGateway interface {
    Gateway
    
    // 帳戶相關
    GetAccountInfo(ctx context.Context) (*AccountInfo, error)
    GetBalance(ctx context.Context) ([]Balance, error)
    
    // 訂單查詢
    GetOrder(ctx context.Context, symbol, orderID string) (*Order, error)
    GetOpenOrders(ctx context.Context, symbol string) ([]Order, error)
    GetAllOrders(ctx context.Context, symbol string, limit int) ([]Order, error)
    
    // 批量操作
    CancelAllOpenOrders(ctx context.Context, symbol string) error
    
    // 持倉 (期貨)
    GetPositions(ctx context.Context, symbol string) ([]Position, error)
    
    // 槓桿 (期貨)
    SetLeverage(ctx context.Context, symbol string, leverage int) error
}

// User Data Stream 介面 (強烈建議)
type UserDataStreamClient interface {
    CreateListenKey(ctx context.Context) (string, error)
    KeepAliveListenKey(ctx context.Context, listenKey string) error
    CloseListenKey(ctx context.Context, listenKey string) error
}

// 查詢介面 (用於對帳)
type ReconciliationClient interface {
    GetPositions(ctx context.Context, symbol string) ([]Position, error)
    GetUserTrades(ctx context.Context, symbol string, limit int, fromID string) ([]Trade, error)
}

// 餘額查詢介面 (用於 balance.Manager)
type BalanceClient interface {
    GetBalance(ctx context.Context) (float64, error)
}
```

### 3.3 共用類型

```go
// pkg/exchanges/common/types.go

// OrderRequest - 下單請求 (系統會填充這個結構)
type OrderRequest struct {
    Symbol       string      // 交易對 (例: BTCUSDT)
    Side         Side        // BUY / SELL
    Type         OrderType   // MARKET / LIMIT / STOP_LOSS 等
    Qty          float64     // 數量
    Price        float64     // 價格 (限價單必填)
    StopPrice    float64     // 觸發價格 (止損/止盈單)
    TimeInForce  TimeInForce // GTC / IOC / FOK
    IcebergQty   float64     // 冰山訂單可見數量
    ClientID     string      // 客戶端訂單 ID
    ReduceOnly   bool        // 僅減倉 (期貨)
    PositionSide string      // LONG/SHORT (對沖模式)
    Market       MarketType  // SPOT / USDT_FUTURES / COIN_FUTURES
    
    // 期貨專用
    WorkingType     string  // MARK_PRICE / CONTRACT_PRICE
    PriceProtect    bool    // 價格保護
    ActivationPrice float64 // 跟蹤止損激活價
    CallbackRate    float64 // 跟蹤止損回調率
}

// OrderResult - 下單結果 (你需要填充這個結構)
type OrderResult struct {
    ExchangeOrderID string      // 交易所返回的訂單 ID
    Status          OrderStatus // 訂單狀態
    ClientID        string      // 客戶端訂單 ID (回顯)
}

// OrderStatus - 標準化訂單狀態
const (
    StatusNew      OrderStatus = "NEW"      // 新建
    StatusPartial  OrderStatus = "PARTIAL"  // 部分成交
    StatusFilled   OrderStatus = "FILLED"   // 完全成交
    StatusCanceled OrderStatus = "CANCELED" // 已取消
    StatusRejected OrderStatus = "REJECTED" // 被拒絕
    StatusExpired  OrderStatus = "EXPIRED"  // 已過期
    StatusUnknown  OrderStatus = "UNKNOWN"  // 未知
)
```

---

## 開發步驟

### 4.1 步驟總覽

```
1. 創建目錄結構
2. 實作 Config 結構
3. 實作 Client 結構
4. 實作 Gateway 介面 (SubmitOrder, CancelOrder)
5. 實作狀態映射函數
6. 整合 RateLimiter 和 TimeSync
7. 實作 User Data Stream (可選但推薦)
8. 編寫單元測試
9. 整合到主程式
```

### 4.2 詳細步驟

#### Step 1: 創建目錄結構

```bash
mkdir -p pkg/exchanges/<exchange_name>/spot
mkdir -p pkg/exchanges/<exchange_name>/futures  # 如果支援期貨
```

#### Step 2: 定義 Config

```go
// pkg/exchanges/<exchange_name>/spot/config.go
package spot

type Config struct {
    APIKey     string
    APISecret  string
    Passphrase string // 某些交易所需要 (例: OKX)
    Testnet    bool
    RecvWindow int64  // 請求有效時間窗口 (毫秒)
}
```

#### Step 3: 實作 Client

```go
// pkg/exchanges/<exchange_name>/spot/client.go
package spot

import (
    "net/http"
    "time"
    
    "trading-core/pkg/exchanges/common"
)

type Client struct {
    cfg         Config
    baseURL     string
    httpClient  *http.Client
    timeSync    *common.TimeSync
    rateLimiter *common.RateLimiter
}

func NewClient(cfg Config) *Client {
    baseURL := "https://api.exchange.com"
    if cfg.Testnet {
        baseURL = "https://testnet-api.exchange.com"
    }
    
    c := &Client{
        cfg:     cfg,
        baseURL: baseURL,
        httpClient: &http.Client{
            Timeout: 10 * time.Second,
        },
    }
    
    // 初始化時間同步
    c.timeSync = common.NewTimeSync(c.GetServerTime)
    
    // 初始化速率限制器 (根據交易所限制調整)
    c.rateLimiter = common.NewRateLimiter(600, time.Minute)
    
    return c
}
```

#### Step 4: 實作 Gateway 介面

```go
// pkg/exchanges/<exchange_name>/spot/orders.go
package spot

import (
    "context"
    "trading-core/pkg/exchanges/common"
)

// SubmitOrder 實作下單邏輯
func (c *Client) SubmitOrder(ctx context.Context, req common.OrderRequest) (common.OrderResult, error) {
    // 1. 構建請求參數
    params := c.buildOrderParams(req)
    
    // 2. 簽名請求
    body, err := c.doSigned(ctx, "POST", "/api/v1/order", params)
    if err != nil {
        return common.OrderResult{}, err
    }
    
    // 3. 解析響應
    var resp orderResponse
    if err := json.Unmarshal(body, &resp); err != nil {
        return common.OrderResult{}, err
    }
    
    // 4. 映射到標準結果
    return common.OrderResult{
        ExchangeOrderID: resp.OrderID,
        Status:          mapStatus(resp.Status),
        ClientID:        resp.ClientOrderID,
    }, nil
}

// CancelOrder 實作撤單邏輯
func (c *Client) CancelOrder(ctx context.Context, symbol, exchangeOrderID string) error {
    params := url.Values{}
    params.Set("symbol", symbol)
    params.Set("orderId", exchangeOrderID)
    
    _, err := c.doSigned(ctx, "DELETE", "/api/v1/order", params)
    return err
}
```

#### Step 5: 實作狀態映射

```go
// pkg/exchanges/<exchange_name>/spot/status.go
package spot

import "trading-core/pkg/exchanges/common"

// mapStatus 將交易所狀態映射到標準狀態
func mapStatus(exchangeStatus string) common.OrderStatus {
    switch exchangeStatus {
    case "NEW", "OPEN", "PENDING":
        return common.StatusNew
    case "PARTIALLY_FILLED", "PARTIAL":
        return common.StatusPartial
    case "FILLED", "CLOSED":
        return common.StatusFilled
    case "CANCELED", "CANCELLED":
        return common.StatusCanceled
    case "REJECTED", "FAILED":
        return common.StatusRejected
    case "EXPIRED":
        return common.StatusExpired
    default:
        return common.StatusUnknown
    }
}
```

#### Step 6: 實作簽名請求

```go
// pkg/exchanges/<exchange_name>/spot/signing.go
package spot

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "net/url"
    "time"
)

func (c *Client) doSigned(ctx context.Context, method, endpoint string, params url.Values) ([]byte, error) {
    // 1. 添加時間戳
    timestamp := time.Now().UnixMilli()
    if c.timeSync != nil && c.timeSync.Offset() != 0 {
        timestamp = c.timeSync.Now()
    }
    params.Set("timestamp", strconv.FormatInt(timestamp, 10))
    
    // 2. 簽名
    queryString := params.Encode()
    signature := c.sign(queryString)
    params.Set("signature", signature)
    
    // 3. 發送請求
    url := c.baseURL + endpoint + "?" + params.Encode()
    req, err := http.NewRequestWithContext(ctx, method, url, nil)
    if err != nil {
        return nil, err
    }
    
    req.Header.Set("X-API-KEY", c.cfg.APIKey)
    
    // 4. 執行請求
    res, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer res.Body.Close()
    
    // 5. 更新速率限制
    if c.rateLimiter != nil {
        weightHeader := res.Header.Get("X-RateLimit-Used")
        c.rateLimiter.UpdateFromHeader(weightHeader)
    }
    
    // 6. 讀取響應
    body, err := io.ReadAll(res.Body)
    if err != nil {
        return nil, err
    }
    
    // 7. 檢查錯誤
    if res.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("API error: %d - %s", res.StatusCode, string(body))
    }
    
    return body, nil
}

func (c *Client) sign(data string) string {
    h := hmac.New(sha256.New, []byte(c.cfg.APISecret))
    h.Write([]byte(data))
    return hex.EncodeToString(h.Sum(nil))
}
```

---

## 共用元件

### 5.1 RateLimiter

```go
import "trading-core/pkg/exchanges/common"

// 初始化 (根據交易所限制調整參數)
limiter := common.NewRateLimiter(600, time.Minute) // 600 weight/分鐘

// 從響應頭更新
limiter.UpdateFromHeader(res.Header.Get("X-RateLimit-Used"))

// 檢查是否需要延遲
if limiter.ShouldDelay() {
    time.Sleep(500 * time.Millisecond)
}
```

### 5.2 TimeSync

```go
import "trading-core/pkg/exchanges/common"

// 初始化
timeSync := common.NewTimeSync(func() (int64, error) {
    return c.GetServerTime()
})

// 啟動同步 (可選，定期同步)
timeSync.Start(ctx)

// 獲取同步後的時間戳
timestamp := timeSync.Now()
```

---

## 程式碼範例

### 6.1 完整的 OKX Spot Client 範例

```go
// pkg/exchanges/okx/spot/client.go
package spot

import (
    "context"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strconv"
    "time"
    
    "trading-core/pkg/exchanges/common"
)

type Config struct {
    APIKey     string
    APISecret  string
    Passphrase string
    Testnet    bool
}

type Client struct {
    cfg         Config
    baseURL     string
    httpClient  *http.Client
    timeSync    *common.TimeSync
    rateLimiter *common.RateLimiter
}

func NewClient(cfg Config) *Client {
    baseURL := "https://www.okx.com"
    if cfg.Testnet {
        baseURL = "https://www.okx.com" // OKX uses simulated trading flag
    }
    
    c := &Client{
        cfg:     cfg,
        baseURL: baseURL,
        httpClient: &http.Client{Timeout: 10 * time.Second},
    }
    
    c.timeSync = common.NewTimeSync(c.GetServerTime)
    c.rateLimiter = common.NewRateLimiter(60, time.Second) // OKX: 60/sec
    
    return c
}

func (c *Client) SubmitOrder(ctx context.Context, req common.OrderRequest) (common.OrderResult, error) {
    body := map[string]interface{}{
        "instId":  req.Symbol,
        "tdMode":  "cash",          // spot mode
        "side":    mapSide(req.Side),
        "ordType": mapOrderType(req.Type),
        "sz":      strconv.FormatFloat(req.Qty, 'f', -1, 64),
    }
    
    if req.Type == common.OrderTypeLimit {
        body["px"] = strconv.FormatFloat(req.Price, 'f', -1, 64)
    }
    
    if req.ClientID != "" {
        body["clOrdId"] = req.ClientID
    }
    
    respBody, err := c.doSigned(ctx, "POST", "/api/v5/trade/order", body)
    if err != nil {
        return common.OrderResult{}, err
    }
    
    var resp struct {
        Code string `json:"code"`
        Msg  string `json:"msg"`
        Data []struct {
            OrdId   string `json:"ordId"`
            ClOrdId string `json:"clOrdId"`
            SCode   string `json:"sCode"`
            SMsg    string `json:"sMsg"`
        } `json:"data"`
    }
    
    if err := json.Unmarshal(respBody, &resp); err != nil {
        return common.OrderResult{}, err
    }
    
    if resp.Code != "0" || len(resp.Data) == 0 {
        return common.OrderResult{}, fmt.Errorf("OKX error: %s - %s", resp.Code, resp.Msg)
    }
    
    return common.OrderResult{
        ExchangeOrderID: resp.Data[0].OrdId,
        Status:          common.StatusNew,
        ClientID:        resp.Data[0].ClOrdId,
    }, nil
}

func (c *Client) CancelOrder(ctx context.Context, symbol, exchangeOrderID string) error {
    body := map[string]interface{}{
        "instId": symbol,
        "ordId":  exchangeOrderID,
    }
    
    _, err := c.doSigned(ctx, "POST", "/api/v5/trade/cancel-order", body)
    return err
}

func (c *Client) GetServerTime() (int64, error) {
    resp, err := c.httpClient.Get(c.baseURL + "/api/v5/public/time")
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()
    
    var result struct {
        Data []struct {
            Ts string `json:"ts"`
        } `json:"data"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return 0, err
    }
    
    if len(result.Data) == 0 {
        return 0, fmt.Errorf("no time data")
    }
    
    return strconv.ParseInt(result.Data[0].Ts, 10, 64)
}

func (c *Client) doSigned(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
    // OKX signature format differs from Binance
    timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
    
    var bodyStr string
    if body != nil {
        b, _ := json.Marshal(body)
        bodyStr = string(b)
    }
    
    // Prehash: timestamp + method + path + body
    prehash := timestamp + method + path + bodyStr
    signature := c.sign(prehash)
    
    req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, strings.NewReader(bodyStr))
    if err != nil {
        return nil, err
    }
    
    req.Header.Set("OK-ACCESS-KEY", c.cfg.APIKey)
    req.Header.Set("OK-ACCESS-SIGN", signature)
    req.Header.Set("OK-ACCESS-TIMESTAMP", timestamp)
    req.Header.Set("OK-ACCESS-PASSPHRASE", c.cfg.Passphrase)
    req.Header.Set("Content-Type", "application/json")
    
    if c.cfg.Testnet {
        req.Header.Set("x-simulated-trading", "1")
    }
    
    res, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer res.Body.Close()
    
    return io.ReadAll(res.Body)
}

func (c *Client) sign(prehash string) string {
    h := hmac.New(sha256.New, []byte(c.cfg.APISecret))
    h.Write([]byte(prehash))
    return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func mapSide(s common.Side) string {
    if s == common.SideBuy {
        return "buy"
    }
    return "sell"
}

func mapOrderType(t common.OrderType) string {
    switch t {
    case common.OrderTypeMarket:
        return "market"
    case common.OrderTypeLimit:
        return "limit"
    default:
        return "market"
    }
}
```

---

## 測試指南

### 7.1 單元測試

```go
// pkg/exchanges/<exchange>/spot/client_test.go
package spot

import (
    "context"
    "testing"
    
    "trading-core/pkg/exchanges/common"
)

func TestSubmitOrder(t *testing.T) {
    cfg := Config{
        APIKey:    "test-key",
        APISecret: "test-secret",
        Testnet:   true,
    }
    
    client := NewClient(cfg)
    
    result, err := client.SubmitOrder(context.Background(), common.OrderRequest{
        Symbol: "BTCUSDT",
        Side:   common.SideBuy,
        Type:   common.OrderTypeMarket,
        Qty:    0.001,
    })
    
    if err != nil {
        t.Fatalf("SubmitOrder failed: %v", err)
    }
    
    if result.ExchangeOrderID == "" {
        t.Error("Expected non-empty order ID")
    }
}
```

### 7.2 整合測試腳本

```bash
# scripts/test_exchange.sh
#!/bin/bash
EXCHANGE=$1

go test -v ./pkg/exchanges/$EXCHANGE/... -count=1
```

---

## 檢查清單

### 8.1 必須完成

- [ ] 實作 `Gateway` 介面 (`SubmitOrder`, `CancelOrder`)
- [ ] 實作狀態映射函數
- [ ] 整合 `RateLimiter`
- [ ] 整合 `TimeSync`
- [ ] 處理 API 錯誤並返回有意義的錯誤訊息
- [ ] 支援 Testnet (如果交易所提供)
- [ ] 編寫基本單元測試

### 8.2 建議完成

- [ ] 實作 `GetAccountInfo`
- [ ] 實作 `GetOpenOrders`
- [ ] 實作 `CancelAllOpenOrders`
- [ ] 實作 User Data Stream
- [ ] 支援所有訂單類型 (LIMIT, STOP_LOSS 等)
- [ ] 編寫整合測試

### 8.3 期貨支援 (如適用)

- [ ] 實作 `GetPositions`
- [ ] 實作 `SetLeverage`
- [ ] 實作 `SetMarginType`
- [ ] 支援 `ReduceOnly` 和 `PositionSide`

---

## 最佳實踐

### 9.1 錯誤處理

```go
// ✅ 好的做法: 包裝錯誤並提供上下文
if res.StatusCode != http.StatusOK {
    return common.OrderResult{}, fmt.Errorf("OKX SubmitOrder failed: status=%d, body=%s", 
        res.StatusCode, string(body))
}

// ❌ 不好的做法: 直接返回原始錯誤
return common.OrderResult{}, err
```

### 9.2 日誌記錄

```go
// 使用標準 log 包
log.Printf("[%s] SubmitOrder: symbol=%s, side=%s, qty=%.6f", 
    "OKX", req.Symbol, req.Side, req.Qty)

// 敏感資訊不要記錄
// ❌ log.Printf("API Secret: %s", c.cfg.APISecret)
```

### 9.3 Context 傳遞

```go
// ✅ 確保 context 傳遞到所有 HTTP 請求
req, err := http.NewRequestWithContext(ctx, method, url, body)
```

### 9.4 配置管理

```go
// 從環境變數讀取敏感配置
cfg := Config{
    APIKey:    os.Getenv("OKX_API_KEY"),
    APISecret: os.Getenv("OKX_API_SECRET"),
    Testnet:   os.Getenv("OKX_TESTNET") == "true",
}
```

---

## 附錄

### A. 常見交易所 API 參考

| 交易所 | 文檔 URL |
|--------|----------|
| Binance | https://binance-docs.github.io/apidocs/spot/en/ |
| OKX | https://www.okx.com/docs-v5/ |
| Bybit | https://bybit-exchange.github.io/docs/ |
| Coinbase | https://docs.cloud.coinbase.com/exchange/docs |
| Kraken | https://docs.kraken.com/rest/ |

### B. 簽名算法參考

| 交易所 | 算法 | 編碼 |
|--------|------|------|
| Binance | HMAC-SHA256 | Hex |
| OKX | HMAC-SHA256 | Base64 |
| Bybit | HMAC-SHA256 | Hex |
| Coinbase | HMAC-SHA256 | Base64 |

### C. 速率限制參考

| 交易所 | 現貨限制 | 期貨限制 |
|--------|----------|----------|
| Binance | 1200 weight/min | 2400 weight/min |
| OKX | 60 req/sec | 60 req/sec |
| Bybit | 120 req/min | 120 req/min |

### D. 更新日誌

| 版本 | 日期 | 變更 |
|------|------|------|
| 1.0 | 2025-12-08 | 初版 |

---

*如有問題，請聯繫系統維護人員或查閱相關交易所的官方 API 文檔。*
