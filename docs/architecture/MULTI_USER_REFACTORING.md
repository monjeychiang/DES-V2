# 多用戶多帳戶架構改造文檔

> **版本**: 2.1  
> **日期**: 2025-12-10  
> **狀態**: ✅ 已實作 (feature/multi-user 分支)  
> **編碼**: UTF-8

---

## 實作完成記錄

| Phase | 說明 | 狀態 | 提交 |
|-------|------|------|------|
| Phase 1 | API Key 加密 | ✅ 完成 | `pkg/crypto/encryption.go`, `key_manager.go` |
| Phase 2 | 資料隔離 | ✅ 完成 | `pkg/db/queries.go`, models.go 更新 |
| Phase 3 | Gateway 管理 | ✅ 完成 | `internal/gateway/manager.go`, `factory.go` |
| Phase 4 | 訂單路由 | ✅ 完成 | `internal/order/types.go` 更新 |
| Phase 5 | 餘額/風控隔離 | ✅ 完成 | `balance/multi_user.go`, `risk/multi_user.go` |
| 整合 | main.go | ✅ 完成 | KeyManager, GatewayManager 初始化 |
| 整合 | API Handler | ✅ 完成 | controllers.go 使用 UserQueries |
| 整合 | Executor 路由 | ✅ 完成 | `gatewayForConnection` 支援 ConnectionID |
| 整合 | Connection 加密 | ✅ 完成 | `createConnection` 使用 KeyManager |
| 整合 | Executor 解密 | ✅ 完成 | `gatewayForConnection` 使用 KeyManager 解密 |
| 整合 | Engine 風控 | ✅ 完成 | Engine 添加 `multiUserRiskMgr` 欄位 |
| 測試 | 單元測試 | ✅ 通過 | `queries_test.go`, `encryption_test.go` |

---

## 目錄

1. [改造目標與非目標](#1-改造目標與非目標)
2. [名詞表 Glossary](#2-名詞表-glossary)
3. [現有基礎](#3-現有基礎)
4. [改造階段 (Phase 1-5)](#4-改造階段)
5. [安全設計](#5-安全設計)
6. [API 變更](#6-api-變更)
7. [資料遷移](#7-資料遷移)
8. [並發與競爭條件](#8-並發與競爭條件)
9. [WebSocket 用戶數據流](#9-websocket-用戶數據流)
10. [API Key 驗證與健康檢查](#10-api-key-驗證與健康檢查)
11. [審計日誌](#11-審計日誌)
12. [Rate Limiting](#12-rate-limiting)
13. [Session 管理](#13-session-管理)
14. [資料庫事務與刪除策略](#14-資料庫事務與刪除策略)
15. [通知系統](#15-通知系統)
16. [發布計畫 Rollout Plan](#16-發布計畫-rollout-plan)
17. [測試計畫](#17-測試計畫)
18. [風險與回滾](#18-風險與回滾)

---

## 1. 改造目標與非目標

### 1.1 目標 (In Scope)

| 項目 | 說明 |
|------|------|
| 多用戶支援 | 多個獨立用戶帳號，各自登入使用 |
| 多連線支援 | 每用戶可綁定多個交易所 API Key |
| 多租戶隔離 | user_id / connection_id 強制隔離所有資料 |
| API Key 加密 | AES-256-GCM 加密儲存，支援金鑰輪替 |
| 動態 Gateway | 每 Connection 獨立 Gateway 實例 |
| 獨立風控 | 每用戶獨立的餘額、倉位、風控計算 |

### 1.2 非目標 (Out of Scope)

| 項目 | 說明 |
|------|------|
| 前端登入 UI | 本文檔僅涵蓋後端，不處理前端實作 |
| 策略語言變更 | 現有策略定義方式維持不變 |
| 跨交易所聚合風控 | 暫不支援多交易所合併計算風險 |
| 跨 Connection 策略 | 單一策略暫只綁定單一 Connection |
| 計費系統 | 用量追蹤與訂閱計費為 Phase 6 擴展 |
| 多交易所整合 | OKX/Bybit 等為未來擴展 |

### 1.3 使用情境

#### 情境 1：個人進階用戶
> 小明在 Binance 有現貨+合約帳戶，想同時運行不同策略。

- 登入 DES 系統
- 新增兩個 Connection：`Binance 現貨` 和 `Binance 合約`
- 在現貨帳戶運行 `BTC RSI 策略`
- 在合約帳戶運行 `ETH 趨勢跟蹤策略`
- 兩帳戶餘額、倉位、風控完全獨立

#### 情境 2：量化團隊
> ABC 團隊有 5 位交易員，需統一管理。

- 管理員建立 5 個用戶帳號
- 每位交易員只能看到自己的數據
- 管理層透過審計日誌查看所有操作

#### 情境 3：SaaS 服務
> 作為 SaaS 平台提供服務。

- 客戶註冊獲得獨立帳戶
- 客戶自行綁定 API Key（加密儲存）
- 客戶間資料完全隔離

---

## 2. 名詞表 Glossary

| 術語 | 定義 | 範例 |
|------|------|------|
| **User** | 系統用戶，擁有獨立帳號 | `user_id = "u-123"` |
| **Connection** | 用戶綁定的交易所連線（含 API Key） | `Binance 現貨帳戶` |
| **Gateway** | 與交易所通訊的客戶端實例 | `exspot.New(...)` |
| **Strategy Instance** | 運行中的策略實例 | `BTC RSI on connection-1` |
| **Position** | 持倉狀態（數量、均價） | `BTCUSDT: 0.1 @ 50000` |
| **Order** | 委託單 | `BUY 0.1 BTC MARKET` |
| **Trade** | 成交紀錄 | `FILLED 0.1 @ 50100` |
| **RiskMetrics** | 風控指標（日盈虧、交易次數） | `daily_pnl = -150` |
| **Tenant** | 租戶（等同 User） | 多租戶架構中的隔離單位 |

---

## 3. 現有基礎

| 功能 | 狀態 | 位置 |
|------|------|------|
| `users` 表 | ✅ 已存在 | `schema.go` |
| `connections` 表 | ✅ 已存在 | `schema.go` |
| `strategy_instances.user_id` | ✅ 已存在 | `schema.go` |
| `strategy_instances.connection_id` | ✅ 已存在 | `schema.go` |

**現有問題：**
- API Key 明文儲存於 `connections.api_key`
- Gateway 為全局單例
- `positions` / `orders` / `trades` 無 `user_id`

---

## 4. 改造階段

### Phase 1: API Key 加密儲存

| 項目 | 內容 |
|------|------|
| **Input** | 現有 `connections.api_key` 明文儲存 |
| **Output** | `api_key_encrypted` / `api_secret_encrypted` 欄位填滿 |
| **Acceptance** | 1. 舊資料已遷移加密 2. 新增 Connection API 使用加密流程 3. 明文欄位可刪除 |

**新增檔案：**
```
pkg/crypto/
├── encryption.go    # AES-256-GCM 加解密
└── key_manager.go   # Master Key 管理
```

**資料庫變更：**
```sql
ALTER TABLE connections ADD COLUMN api_key_encrypted TEXT;
ALTER TABLE connections ADD COLUMN api_secret_encrypted TEXT;
ALTER TABLE connections ADD COLUMN key_version INTEGER DEFAULT 1;
```

**金鑰輪替策略：**
- `key_version` 欄位標記加密版本
- 輪替時：新資料用 v2，舊資料批次重加密
- 解密時依 version 選擇對應 key

**密文格式：**
```
ENC[v1]:base64(nonce + ciphertext + tag)
```

**日誌安全：**
- 永不記錄明文 API Key
- 只記錄 masked 版本：`BINANCE_***_KEY`

**預估時間：** 2 小時

---

### Phase 2: 資料隔離

| 項目 | 內容 |
|------|------|
| **Input** | `positions` / `orders` / `trades` 無 user_id |
| **Output** | 所有表都有 `user_id` 且已回填 |
| **Acceptance** | 1. Migration 完成 2. 查詢層強制帶 user_id 3. E2E 隔離測試通過 |

**資料庫變更：**
```sql
-- positions: 改為複合主鍵
ALTER TABLE positions ADD COLUMN user_id TEXT NOT NULL DEFAULT 'default';
-- 需重建表以改主鍵

-- orders
ALTER TABLE orders ADD COLUMN user_id TEXT;
CREATE INDEX idx_orders_user_time ON orders(user_id, created_at);

-- trades
ALTER TABLE trades ADD COLUMN user_id TEXT;
CREATE INDEX idx_trades_user_time ON trades(user_id, created_at);

-- risk_metrics: 改主鍵
ALTER TABLE risk_metrics ADD COLUMN user_id TEXT NOT NULL DEFAULT 'default';
CREATE INDEX idx_risk_user_date ON risk_metrics(user_id, date);
```

**資料回填策略：**
1. 建立 `default_user` 作為遷移過渡
2. 批次更新現有資料的 user_id
3. 遷移完成後可移除 default

**查詢安全護欄：**
```go
// 所有查詢必須帶 user_id
func (db *Database) GetPositions(userID string) ([]Position, error) {
    if userID == "" {
        return nil, errors.New("user_id required")
    }
    // ...
}
```

**預估時間：** 2.5 小時

---

### Phase 3: 動態 Gateway 管理

| 項目 | 內容 |
|------|------|
| **Input** | 全局單例 Gateway |
| **Output** | per-Connection Gateway 池 |
| **Acceptance** | 1. 多 Connection 同時運行 2. 閒置自動清理 3. 健康檢查正常 |

**新增模組：**
```
internal/gateway/
├── manager.go       # Gateway 池管理
├── cached.go        # CachedGateway 結構
└── lifecycle.go     # 健康檢查、清理
```

**核心結構：**
```go
type GatewayManager struct {
    mu       sync.RWMutex
    gateways map[string]*CachedGateway
    crypto   *crypto.KeyManager
    maxSize  int // LRU 上限
}

type CachedGateway struct {
    Gateway     exchange.Gateway
    UserStream  *order.UserStream
    CreatedAt   time.Time
    LastUsed    time.Time
    HealthyAt   time.Time
}
```

**生命週期：**
| 事件 | 處理 |
|------|------|
| 首次使用 | DB 讀取 → 解密 → 創建 Gateway → 快取 |
| 後續使用 | 更新 LastUsed |
| 閒置 30 分鐘 | 關閉並清除 |
| 超過 maxSize | LRU 淘汰 |
| 用戶刪除 Connection | 主動清除 |

**熔斷策略：**
```go
type CircuitBreaker struct {
    failures   int
    threshold  int  // 連續失敗 N 次觸發
    openUntil  time.Time
    halfOpenAt time.Time
}

// 失敗達閾值 → 標記 unhealthy 5 分鐘
// 期間不嘗試連線，避免打壓外部 API
```

**可觀測性 Metrics：**
- `gateway_count` - 當前 Gateway 數量
- `gateway_create_total` - 創建總數
- `gateway_error_total{conn_id}` - 每 connection 錯誤數
- `gateway_latency_seconds` - 操作延遲

**預估時間：** 3 小時

---

### Phase 4: 訂單執行路由

| 項目 | 內容 |
|------|------|
| **Input** | 訂單無 connection_id |
| **Output** | 訂單自動路由到正確 Gateway |
| **Acceptance** | 1. 訂單帶 connection_id 2. 執行時路由正確 |

**Order 結構擴展：**
```go
type Order struct {
    // ... 現有欄位
    UserID       string `json:"user_id"`
    ConnectionID string `json:"connection_id"`
}
```

**Executor 修改：**
```go
func (e *Executor) Execute(ctx context.Context, o Order) error {
    gw, err := e.gatewayMgr.GetOrCreate(o.ConnectionID)
    if err != nil {
        return fmt.Errorf("get gateway: %w", err)
    }
    return gw.PlaceOrder(ctx, o.ToExchangeOrder())
}
```

**預估時間：** 1.5 小時

---

### Phase 5: 餘額與風控隔離

| 項目 | 內容 |
|------|------|
| **Input** | 全局餘額/風控 |
| **Output** | per-User 餘額/風控 |
| **Acceptance** | 1. 用戶間互不影響 2. 重啟後狀態恢復 |

**BalanceManager：**
```go
type BalanceManager struct {
    mu       sync.RWMutex
    balances map[string]*UserBalance // userID -> balance
}
```

**RiskManager：**
```go
type RiskManager struct {
    configs map[string]*RiskConfig   // userID -> config
    metrics map[string]*RiskMetrics  // userID -> metrics
}
```

**設定優先順序：**
1. per-strategy config（最高優先）
2. per-user config
3. global config（fallback）

**持久化：**
- RiskMetrics 寫入 DB（重啟恢復）
- UserBalance 從交易所同步（啟動時拉取）

**併發控制：**
- per-user 鎖（非全局鎖）
- 避免跨用戶操作互相阻塞

**預估時間：** 2.5 小時

---

## 5. 安全設計

### 5.1 加密架構

```
┌─────────────────────────────────────┐
│        Environment Variable          │
│  MASTER_ENCRYPTION_KEY (32 bytes)   │
│  MASTER_ENCRYPTION_KEY_V2 (輪替用)  │
└─────────────────────────────────────┘
                  ↓
┌─────────────────────────────────────┐
│         KeyManager (Memory)          │
│  - 啟動時載入，永不落地             │
│  - 支援多版本 key                   │
└─────────────────────────────────────┘
                  ↓
┌─────────────────────────────────────┐
│        AES-256-GCM Encryption        │
│  - 每次加密使用隨機 nonce           │
│  - Authenticated encryption          │
└─────────────────────────────────────┘
                  ↓
┌─────────────────────────────────────┐
│      Database (Encrypted Data)       │
│  ENC[v1]:base64(nonce+ciphertext)   │
└─────────────────────────────────────┘
```

### 5.2 安全考量

| 風險 | 緩解措施 |
|------|----------|
| Master Key 洩漏 | 環境變數，不落地；可用 Vault |
| 資料庫被盜 | 密文無法反推明文 |
| SQL Injection | Prepared Statements |
| 用戶越權 | 所有查詢強制 user_id |
| 日誌洩漏 | 永不記錄明文 key |

---

## 6. API 變更

### 6.1 新增端點

| 方法 | 路徑 | 說明 |
|------|------|------|
| `POST` | `/api/connections` | 新增連線 |
| `GET` | `/api/connections` | 列出連線 |
| `DELETE` | `/api/connections/:id` | 刪除連線 |
| `POST` | `/api/connections/:id/test` | 測試連線 |

### 6.2 錯誤回應格式

```json
{
  "error": {
    "code": "RATE_LIMITED",
    "message": "Too many requests",
    "retry_after": 60
  }
}
```

---

## 7. 資料遷移

### 7.1 遷移步驟

1. **新增欄位**（不停機）
2. **雙寫期**：新資料寫加密欄位，舊資料回填
3. **驗證**：檢查所有資料已遷移
4. **清理**：移除明文欄位

### 7.2 回填策略

```go
func BackfillEncryption(db *Database, crypto *KeyManager, batchSize int) error {
    for {
        rows, _ := db.Query(`
            SELECT id, api_key, api_secret FROM connections 
            WHERE api_key_encrypted IS NULL LIMIT ?`, batchSize)
        if len(rows) == 0 {
            break
        }
        for _, row := range rows {
            enc, _ := crypto.Encrypt(row.APIKey)
            db.Exec("UPDATE connections SET api_key_encrypted = ? WHERE id = ?", enc, row.ID)
        }
    }
}
```

---

## 8. 並發與競爭條件

| 場景 | 問題 | 解決方案 |
|------|------|----------|
| 同時創建 Gateway | 重複創建 | Double-check locking |
| 同時更新餘額 | 計算錯誤 | per-user 鎖 |
| 同時下單 | 曝險不準確 | pending orders 計入 |

---

## 9. WebSocket 用戶數據流

```go
type StreamManager struct {
    streams map[string]*ConnectionStream // connectionID -> stream
    mu      sync.RWMutex
}

type ConnectionStream struct {
    UserStream *order.UserStream
    stopCh     chan struct{}
}
```

**生命週期：**
- Gateway 創建時啟動
- Gateway 銷毀時關閉
- 斷線自動重連（指數退避）

**連線數限制：**
- 單機最大 100 WebSocket
- 超過則拒絕新連線或淘汰最久未用

---

## 10. API Key 驗證與健康檢查

### 10.1 新增時驗證

```go
func (h *Handler) CreateConnection(c echo.Context) error {
    // 1. 測試 API Key 有效性
    gw := createTempGateway(req.APIKey, req.APISecret)
    if err := gw.TestConnection(); err != nil {
        return echo.NewHTTPError(400, "Invalid API Key")
    }
    // 2. 加密並儲存
}
```

### 10.2 定期健康檢查

```go
// 每 5 分鐘執行
func (m *GatewayManager) HealthCheck() {
    for connID, cached := range m.gateways {
        if err := cached.Gateway.Ping(); err != nil {
            cached.Failures++
            if cached.Failures >= 3 {
                notify(connID, "Connection unhealthy")
            }
        }
    }
}
```

---

## 11. 審計日誌

```sql
CREATE TABLE audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    resource_type TEXT,
    resource_id TEXT,
    ip_address TEXT,
    user_agent TEXT,
    details TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_audit_user_time ON audit_logs(user_id, created_at);
CREATE INDEX idx_audit_event ON audit_logs(event_type);
```

**記錄事件：**
- API Key 存取/修改
- 登入/登出
- 敏感操作
- Rate Limit 被擋

---

## 12. Rate Limiting

| 層級 | 限制 | 說明 |
|------|------|------|
| 全局 | 1000 req/min | DDoS 防護 |
| 每用戶 | 100 req/min | 公平使用 |
| 每 Connection | 10 orders/sec | 交易所限制 |

**優先順序：** Global → User → Connection

---

## 13. Session 管理

### 13.1 JWT 結構

```json
{
  "user_id": "uuid",
  "email": "user@example.com",
  "device_id": "fingerprint",
  "exp": 1702234567
}
```

### 13.2 Token 生命週期

- Access Token: 15 分鐘
- Refresh Token: 7 天
- 改密碼時作廢所有 token

---

## 14. 資料庫事務與刪除策略

### 14.1 刪除用戶

```go
func (db *Database) DeleteUser(userID string) error {
    tx, _ := db.DB.Begin()
    defer tx.Rollback()

    // 順序重要：先刪子表
    tx.Exec("DELETE FROM audit_logs WHERE user_id = ?", userID)
    tx.Exec("DELETE FROM trades WHERE user_id = ?", userID)
    tx.Exec("DELETE FROM orders WHERE user_id = ?", userID)
    tx.Exec("DELETE FROM positions WHERE user_id = ?", userID)
    tx.Exec("DELETE FROM strategy_instances WHERE user_id = ?", userID)
    tx.Exec("DELETE FROM connections WHERE user_id = ?", userID)
    tx.Exec("DELETE FROM users WHERE id = ?", userID)

    return tx.Commit()
}
```

### 14.2 法規保留

- `audit_logs` 保留 3 年
- `trades` 保留 7 年
- 用軟刪除 `is_deleted` 而非物理刪除

---

## 15. 通知系統

| 通知類型 | 觸發條件 |
|----------|----------|
| 訂單成交 | 狀態變為 FILLED |
| API Key 異常 | 連線失敗 3 次 |
| 風控告警 | 達每日虧損上限 |

**支援通道：** Email / Telegram / Webhook

---

## 16. 發布計畫 Rollout Plan

### 16.1 Feature Flag

```go
type FeatureFlags struct {
    EnableMultiUser    bool
    EnableEncryption   bool
    EnableGatewayPool  bool
}
```

### 16.2 分階段上線

| 階段 | 範圍 | 驗證 |
|------|------|------|
| Alpha | 內部測試帳號 | 功能正確性 |
| Beta | 10% 用戶 | 效能/穩定性 |
| GA | 全量 | 監控指標 |

### 16.3 回滾策略

- Schema 保留舊欄位 2 週
- Code 支援 feature flag 切換
- DB 有完整 backup

---

## 17. 測試計畫

| 測試類型 | 內容 |
|----------|------|
| 單元測試 | 加解密、Gateway 管理 |
| 整合測試 | API 流程、DB 操作 |
| E2E 測試 | 多用戶隔離驗證 |
| 負載測試 | 100 用戶同時交易 |
| 安全測試 | SQL Injection、越權存取 |

---

## 18. 風險與回滾

| 風險 | 影響 | 回滾方案 |
|------|------|----------|
| 加密邏輯錯誤 | Key 無法使用 | 保留明文欄位 |
| Gateway 洩漏 | OOM | 連線池限制 |
| 效能下降 | 延遲增加 | 索引優化 |
| 資料不一致 | 錯誤交易 | 停機遷移 |

---

## 附錄：時程預估

| 階段 | 時間 | 優先級 |
|------|------|--------|
| Phase 1 | 2 hr | 🔴 高 |
| Phase 2 | 2.5 hr | 🔴 高 |
| Phase 3 | 3 hr | 🔴 高 |
| Phase 4 | 1.5 hr | 🟡 中 |
| Phase 5 | 2.5 hr | 🟡 中 |

**總計：約 11.5 小時**
