# 多用戶多帳戶架構文檔

> **版本**: 3.0  
> **日期**: 2025-12-10  
> **狀態**: ✅ 已完成實作  
> **分支**: `feature/multi-user`

---

## 最終目標

將交易系統從單用戶模式升級為多用戶多帳戶架構，支援：

- **多個獨立用戶帳號**：每位用戶有自己的登入憑證和 JWT
- **每用戶多個交易所連線**：一位用戶可綁定多個現貨/合約帳戶
- **完全資料隔離**：用戶之間無法看到或影響彼此的資料
- **獨立風險控制**：每用戶有獨立的餘額管理和風控評估

---

## 使用情境

### 情境 1：個人進階用戶

> 一位交易者同時管理自己的幣安現貨和合約帳戶

```
User A (登入)
  ├── Connection 1: binance-spot (API Key A1)
  │     └── Strategy: BTCUSDT 網格
  └── Connection 2: binance-usdtfut (API Key A2)
        └── Strategy: ETHUSDT 趨勢跟蹤
```

### 情境 2：量化團隊

> 多位團隊成員各自管理自己的策略，互不干擾

```
User A (策略開發者)
  └── 策略 X, Y → 只看到自己的訂單/持倉

User B (策略研究員)
  └── 策略 Z → 完全隔離，看不到 User A 的資料
```

### 情境 3：SaaS 服務

> 作為多租戶平台，每位客戶獨立運作

```
Tenant A (客戶)           Tenant B (客戶)
  ├── 自己的 API Keys       ├── 自己的 API Keys
  ├── 自己的策略             ├── 自己的策略
  ├── 自己的餘額             ├── 自己的餘額
  └── 加密存儲               └── 加密存儲
```

### 情境 4：多交易所套利

> 一位用戶同時使用多個交易所進行套利

```
User A
  ├── Connection 1: 幣安現貨
  ├── Connection 2: 幣安 U 本位合約
  └── Connection 3: 幣安幣本位合約
        ↓
  策略可指定任一 Connection 下單
```

---

## 實作完成狀態

| 模組 | 狀態 | 關鍵檔案 |
|------|------|----------|
| API Key 加密 | ✅ | `pkg/crypto/encryption.go`, `key_manager.go` |
| 資料隔離 | ✅ | `pkg/db/queries.go`, `models.go` |
| Gateway 管理 | ✅ | `internal/gateway/manager.go` |
| 訂單路由 | ✅ | `internal/order/executor.go` |
| Per-User 餘額 | ✅ | `internal/balance/multi_user.go` |
| Per-User 風控 | ✅ | `internal/risk/multi_user.go` |
| API 整合 | ✅ | `internal/api/controllers.go`, `handler.go` |
| 核心流程整合 | ✅ | `main.go` |

---

## 1. 架構總覽

```
┌─────────────────────────────────────────────────────────────┐
│                        API Layer                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │ Auth Handler│  │Order Handler│  │Strategy API │        │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘        │
│         └────────────────┼─────────────────┘                │
│                   ┌──────▼──────┐                          │
│                   │ JWT Auth    │                          │
│                   │ (userID)    │                          │
│                   └──────┬──────┘                          │
└──────────────────────────┼──────────────────────────────────┘
                           │
┌──────────────────────────┼──────────────────────────────────┐
│                    Service Layer                            │
│  ┌─────────────┐  ┌──────▼──────┐  ┌─────────────┐        │
│  │ UserQueries │  │  Executor   │  │MultiUserRisk│        │
│  │ (user_id)   │  │(ConnectionID)│ │  Manager    │        │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘        │
│         │         ┌──────▼──────┐  ┌──────▼──────┐        │
│         │         │ KeyManager  │  │MultiUserBal │        │
│         │         │ (Decrypt)   │  │  Manager    │        │
│         │         └──────┬──────┘  └──────┬──────┘        │
│  ┌──────▼──────┐  ┌──────▼──────┐  ┌──────▼──────┐        │
│  │  SQLite DB  │  │Gateway Pool │  │ Per-User    │        │
│  │ (isolated)  │  │(perConnection)│ │ Instances   │        │
│  └─────────────┘  └─────────────┘  └─────────────┘        │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. 資料隔離

### 2.1 資料庫欄位

| 資料表 | user_id 欄位 | 索引 |
|--------|-------------|------|
| `orders` | ✅ | `idx_orders_user_time` |
| `trades` | ✅ | `idx_trades_user_time` |
| `positions` | ✅ | `idx_positions_user` |
| `connections` | ✅ (既有) | `idx_connections_user` |
| `strategy_instances` | ✅ (既有) | - |

### 2.2 寫入隔離

```go
// CreateOrder - 寫入時包含 user_id
_, err := d.DB.ExecContext(ctx, `
    INSERT INTO orders (..., user_id, ...) VALUES (..., ?, ...)
`, ..., o.UserID, ...)

// CreateTrade - 寫入時包含 user_id
_, err := d.DB.ExecContext(ctx, `
    INSERT INTO trades (..., user_id, ...) VALUES (..., ?, ...)
`, ..., t.UserID, ...)

// UpsertPosition - 寫入時包含 user_id
_, err := d.DB.ExecContext(ctx, `
    INSERT INTO positions (..., user_id, ...) VALUES (..., ?, ...)
    ON CONFLICT(symbol) DO UPDATE SET user_id = excluded.user_id, ...
`, ..., p.UserID, ...)
```

### 2.3 查詢隔離

```go
// pkg/db/queries.go - UserQueries
func (q *UserQueries) GetOrdersByUser(ctx, userID string, limit int) ([]Order, error)
func (q *UserQueries) GetPositionsByUser(ctx, userID string) ([]Position, error)
func (q *UserQueries) GetConnectionsByUser(ctx, userID string) ([]Connection, error)
```

---

## 3. API Key 加密

### 3.1 加密流程

```
用戶提交 API Key → KeyManager.Encrypt() → 存入 api_key_encrypted
                                        → 存入 key_version
```

### 3.2 解密流程

```
Executor 需下單 → 讀取 api_key_encrypted
               → KeyManager.Decrypt()
               → 創建 Gateway
               → 提交訂單
```

### 3.3 關鍵程式碼

```go
// controllers.go - createConnection
if s.KeyManager != nil {
    encKey, _ := s.KeyManager.Encrypt(req.APIKey)
    encSecret, _ := s.KeyManager.Encrypt(req.APISecret)
    conn.APIKeyEncrypted = encKey
    conn.APISecretEncrypted = encSecret
    conn.KeyVersion = s.KeyManager.CurrentVersion()
    s.DB.Queries().CreateConnectionEncrypted(ctx, conn)
}

// executor.go - gatewayForConnection
if apiKeyEnc != "" && e.KeyManager != nil {
    decryptedKey, _ := e.KeyManager.Decrypt(apiKeyEnc)
    decryptedSecret, _ := e.KeyManager.Decrypt(apiSecretEnc)
    // 使用解密後的金鑰創建 Gateway
}
```

---

## 4. Per-User 餘額與風控

### 4.1 MultiUserManager 模式

```go
// balance/multi_user.go
type MultiUserManager struct {
    managers map[string]*Manager  // userID -> Manager
    factory  func(userID string) (*Manager, error)
}

func (m *MultiUserManager) GetOrCreate(userID string) (*Manager, error)
```

### 4.2 信號處理整合 (main.go)

```go
// 1. 解析策略所有者
database.DB.QueryRowContext(ctx, `
    SELECT si.user_id, si.connection_id, c.exchange_type
    FROM strategy_instances si LEFT JOIN connections c ON ...
    WHERE si.id = ?
`, sig.StrategyID).Scan(&stratUserID, &stratConnID, ...)

// 2. Per-user 餘額鎖定
if userID != "" && userBalanceMgr != nil {
    balSource, _ = userBalanceMgr.GetOrCreate(userID)
}
balSource.Lock(finalOrderValue)

// 3. Per-user 風控評估
if userID != "" && multiUserRisk != nil {
    decision, _ = multiUserRisk.EvaluateForUser(userID, signalInput, ...)
}

// 4. 訂單填入 UserID/ConnectionID
o := order.Order{
    UserID:       userID,
    ConnectionID: connectionID,
    ...
}
```

---

## 5. API 端點

### 5.1 認證相關

| 方法 | 路徑 | 說明 |
|------|------|------|
| POST | `/api/v1/auth/register` | 註冊新用戶 |
| POST | `/api/v1/auth/login` | 登入取得 JWT |

### 5.2 連線管理

| 方法 | 路徑 | 說明 |
|------|------|------|
| POST | `/api/v1/connections` | 新增連線 (自動加密) |
| GET | `/api/v1/connections` | 列出我的連線 |
| DELETE | `/api/v1/connections/:id` | 停用連線 |

### 5.3 策略管理

| 方法 | 路徑 | 說明 |
|------|------|------|
| POST | `/api/v1/strategies` | 創建策略 (綁定 connection) |
| GET | `/api/v1/strategies` | 列出我的策略 |
| POST | `/api/v1/strategies/:id/start` | 啟動策略 |

### 5.4 交易與查詢

| 方法 | 路徑 | 說明 |
|------|------|------|
| POST | `/api/v1/orders` | 手動下單 (指定 connection_id) |
| GET | `/api/v1/orders` | 查詢我的訂單 |
| GET | `/api/v1/positions` | 查詢我的持倉 |
| GET | `/api/v1/balance` | 查詢我的餘額 |

---

## 6. 環境設定

```bash
# .env
MASTER_ENCRYPTION_KEY=<openssl rand -base64 32>
JWT_SECRET=your-secret
DB_PATH=./trading.db
```

---

## 7. 安全保證

| 層級 | 機制 |
|------|------|
| 資料庫層 | 所有查詢強制 `WHERE user_id = ?` |
| API 層 | JWT 提取 userID，無法偽造 |
| Gateway 層 | 連線驗證所有權 + 緩存隔離 |
| 加密層 | AES-256-GCM + Key Version 輪替 |
| Balance 層 | Per-user 獨立實例 |
| Risk 層 | Per-user 獨立實例 |

---

## 8. 文件清單

| 路徑 | 說明 |
|------|------|
| `pkg/crypto/encryption.go` | AES-256-GCM 加密 |
| `pkg/crypto/key_manager.go` | 多版本金鑰管理 |
| `pkg/db/queries.go` | 用戶隔離查詢 |
| `pkg/db/models.go` | 資料模型 + CRUD |
| `internal/gateway/manager.go` | Gateway 池管理 |
| `internal/order/executor.go` | 訂單執行 + 解密 |
| `internal/balance/multi_user.go` | Per-user 餘額 |
| `internal/risk/multi_user.go` | Per-user 風控 |
| `internal/api/controllers.go` | API 處理器 |
| `internal/api/handler.go` | 路由定義 |
| `main.go` | 核心流程整合 |
