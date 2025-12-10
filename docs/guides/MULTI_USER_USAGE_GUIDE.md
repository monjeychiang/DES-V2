# 多用戶系統完整使用流程

> **版本**: 1.0  
> **日期**: 2025-12-10

---

## 目錄

1. [系統設定](#1-系統設定)
2. [用戶註冊與登入](#2-用戶註冊與登入)
3. [交易所連線管理](#3-交易所連線管理)
4. [策略綁定與交易](#4-策略綁定與交易)
5. [資料隔離驗證](#5-資料隔離驗證)

---

## 1. 系統設定

### 1.1 環境變數配置

編輯 `backend/cmd/trading-core/.env`：

```bash
# 必要：API Key 加密主金鑰 (32 bytes, base64 encoded)
# 生成方式: openssl rand -base64 32
MASTER_ENCRYPTION_KEY=your-32-byte-base64-key-here

# 可選：金鑰輪替（支援多版本金鑰）
# MASTER_ENCRYPTION_KEY_V2=new-key-for-rotation

# JWT 認證
JWT_SECRET=your-jwt-secret

# 其他標準設定
DB_PATH=./data/trading.db
```

### 1.2 資料庫遷移

啟動系統時會自動執行遷移，新增以下欄位：

| 資料表 | 新增欄位 |
|--------|----------|
| `connections` | `api_key_encrypted`, `api_secret_encrypted`, `key_version` |
| `orders` | `user_id` |
| `trades` | `user_id` |
| `positions` | `user_id` |

---

## 2. 用戶註冊與登入

### 2.1 註冊新用戶

```http
POST /api/auth/register
Content-Type: application/json

{
  "username": "trader1",
  "email": "trader1@example.com",
  "password": "SecurePassword123!"
}
```

**回應：**
```json
{
  "user_id": "usr_abc123",
  "username": "trader1"
}
```

### 2.2 用戶登入

```http
POST /api/auth/login
Content-Type: application/json

{
  "email": "trader1@example.com",
  "password": "SecurePassword123!"
}
```

**回應：**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_at": "2025-12-11T10:00:00Z"
}
```

### 2.3 後續請求授權

所有後續 API 請求都需要在 Header 中帶入 JWT：

```http
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

---

## 3. 交易所連線管理

### 3.1 新增交易所連線（API Key 自動加密）

```http
POST /api/connections
Authorization: Bearer {token}
Content-Type: application/json

{
  "name": "我的幣安現貨",
  "exchange_type": "binance-spot",
  "api_key": "your-binance-api-key",
  "api_secret": "your-binance-api-secret"
}
```

**回應：**
```json
{
  "id": "conn_xyz789",
  "name": "我的幣安現貨",
  "exchange_type": "binance-spot",
  "is_active": true,
  "encrypted": true,  // 表示 API Key 已加密存儲
  "created_at": "2025-12-10T10:00:00Z"
}
```

> **安全說明**：API Key 使用 AES-256-GCM 加密後存儲，原始金鑰永不持久化

### 3.2 查看我的連線列表

```http
GET /api/connections
Authorization: Bearer {token}
```

**回應：**
```json
[
  {
    "id": "conn_xyz789",
    "name": "我的幣安現貨",
    "exchange_type": "binance-spot",
    "is_active": true
  },
  {
    "id": "conn_abc456",
    "name": "合約帳戶",
    "exchange_type": "binance-usdtfut",
    "is_active": true
  }
]
```

### 3.3 停用連線

```http
DELETE /api/connections/conn_xyz789
Authorization: Bearer {token}
```

---

## 4. 策略綁定與交易

### 4.1 創建策略並綁定連線

```http
POST /api/strategies
Authorization: Bearer {token}
Content-Type: application/json

{
  "name": "BTCUSDT 網格策略",
  "strategy_type": "grid",
  "symbol": "BTCUSDT",
  "interval": "1h",
  "connection_id": "conn_xyz789",  // 綁定到特定交易所連線
  "parameters": {
    "grid_levels": 10,
    "price_low": 40000,
    "price_high": 50000
  }
}
```

### 4.2 手動下單（指定連線）

```http
POST /api/orders
Authorization: Bearer {token}
Content-Type: application/json

{
  "symbol": "BTCUSDT",
  "side": "BUY",
  "type": "LIMIT",
  "price": 42000,
  "qty": 0.01,
  "connection_id": "conn_xyz789"  // 指定使用哪個連線下單
}
```

**執行流程：**

```
┌─────────────────┐
│  API Request    │
│  (with token)   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Auth Middleware│
│  Extract userID │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Executor       │
│  gatewayForConnection()
└────────┬────────┘
         │
    ┌────┴────┐
    │ 解密 API Key │
    │ (KeyManager) │
    └────┬────┘
         │
         ▼
┌─────────────────┐
│  Exchange Gateway│
│  Submit Order    │
└─────────────────┘
```

### 4.3 查詢我的訂單

```http
GET /api/orders
Authorization: Bearer {token}
```

**回應（只返回當前用戶的訂單）：**
```json
[
  {
    "id": "ord_123",
    "symbol": "BTCUSDT",
    "side": "BUY",
    "price": 42000,
    "qty": 0.01,
    "status": "FILLED",
    "user_id": "usr_abc123"
  }
]
```

### 4.4 查詢我的持倉

```http
GET /api/positions
Authorization: Bearer {token}
```

---

## 5. 資料隔離驗證

### 5.1 用戶隔離原則

| 資源 | 隔離機制 |
|------|----------|
| 連線 (Connections) | `user_id` 過濾 + 權限驗證 |
| 訂單 (Orders) | `user_id` 強制欄位 |
| 交易 (Trades) | `user_id` 強制欄位 |
| 持倉 (Positions) | `user_id` 強制欄位 |
| 策略 (Strategies) | `user_id` 綁定 |
| 風控 (Risk) | 每用戶獨立 Manager 實例 |

### 5.2 安全保證

1. **資料庫層**：所有查詢強制包含 `WHERE user_id = ?`
2. **API 層**：Handler 從 JWT 提取 userID，無法偽造
3. **Gateway 層**：連線緩存按 `connection_id` 隔離
4. **風控層**：每用戶獨立的 `risk.Manager` 實例

### 5.3 錯誤場景處理

| 場景 | 回應 |
|------|------|
| 訪問其他用戶的連線 | `403 Forbidden` |
| 使用未加密的連線（無 KeyManager） | `500 Internal Error` |
| 空的 user_id 查詢 | `ErrUserIDRequired` |

---

## 6. 系統架構圖

```
┌─────────────────────────────────────────────────────────────┐
│                        API Layer                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │ Auth Handler│  │Order Handler│  │Conn Handler │        │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘        │
│         │                │                 │                │
│         └────────────────┼─────────────────┘                │
│                          │                                  │
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
│  │ (user_id)   │  │ (ConnID)    │  │  Manager    │        │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘        │
│         │                │                 │                │
│         │         ┌──────▼──────┐         │                │
│         │         │ KeyManager  │         │                │
│         │         │ (Decrypt)   │         │                │
│         │         └──────┬──────┘         │                │
│         │                │                 │                │
│  ┌──────▼──────┐  ┌──────▼──────┐  ┌──────▼──────┐        │
│  │  SQLite DB  │  │Gateway Pool │  │ Per-User    │        │
│  │ (isolated)  │  │(perConnection)│ │ Risk Mgr   │        │
│  └─────────────┘  └─────────────┘  └─────────────┘        │
└─────────────────────────────────────────────────────────────┘
```

---

## 7. 快速開始

```bash
# 1. 設定加密金鑰
echo "MASTER_ENCRYPTION_KEY=$(openssl rand -base64 32)" >> .env

# 2. 啟動系統
go run main.go

# 3. 註冊用戶
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"demo","email":"demo@example.com","password":"Demo123!"}'

# 4. 登入取得 token
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"demo@example.com","password":"Demo123!"}' | jq -r '.token')

# 5. 新增交易所連線
curl -X POST http://localhost:8080/api/connections \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"My Binance","exchange_type":"binance-spot","api_key":"xxx","api_secret":"yyy"}'

# 6. 開始交易！
```
