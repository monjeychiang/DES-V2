# License 系統設計文檔

> **狀態**: 📋 規劃中  
> **驗證方式**: 用戶登入  
> **商業模式**: 訂閱制 (月/年)  
> **功能分級**: 無 (統一方案)

---

## 📋 概述

### 核心概念

```
用戶購買訂閱 → 創建帳號 → 客戶部署 → 登入驗證 → 使用系統
```

### 驗證流程

```
┌──────────────────────────────────────────────────────────────┐
│                    License Server (你維護)                    │
│                                                              │
│  用戶資料庫 + 訂閱狀態 + Token 簽發                            │
└──────────────────────────────────────────────────────────────┘
                            ▲
                            │ HTTPS
                            ▼
┌──────────────────────────────────────────────────────────────┐
│                  Trading System (客戶部署)                    │
│                                                              │
│  啟動時驗證 → Token 緩存 → 定期刷新                            │
└──────────────────────────────────────────────────────────────┘
```

---

## 🏗️ 系統架構

### 組件

| 組件 | 說明 | 位置 |
|------|------|------|
| **License Server** | 中央驗證服務 | 你的雲端主機 |
| **License Client** | 客戶端驗證模組 | trading-core 內 |
| **Admin Panel** | 用戶/訂閱管理 | License Server |

### License Server API

| 端點 | 方法 | 說明 |
|------|------|------|
| `/api/auth/login` | POST | 用戶登入，返回 Token |
| `/api/auth/refresh` | POST | 刷新 Token |
| `/api/auth/validate` | POST | 驗證 Token 有效性 |
| `/api/subscription/status` | GET | 查詢訂閱狀態 |

### 資料表設計

#### License Server 資料庫

```sql
-- 用戶表
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 訂閱表
CREATE TABLE subscriptions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    plan TEXT DEFAULT 'standard',
    status TEXT DEFAULT 'active',  -- active, expired, cancelled
    starts_at DATETIME NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES users(id)
);

-- 登入記錄 (可選)
CREATE TABLE login_logs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    ip_address TEXT,
    machine_id TEXT,
    logged_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES users(id)
);
```

---

## 🔐 Token 設計

### JWT Payload

```json
{
  "sub": "user_001",
  "email": "customer@example.com",
  "subscription_status": "active",
  "expires_at": "2025-12-31T23:59:59Z",
  "iat": 1733644800,
  "exp": 1733731200
}
```

### Token 生命週期

| 類型 | 有效期 | 用途 |
|------|--------|------|
| Access Token | 24 小時 | API 認證 |
| Refresh Token | 30 天 | 刷新 Access Token |
| 離線寬限期 | 7 天 | 無網路時可繼續使用 |

---

## 📱 客戶端驗證邏輯

### 完整啟動流程 (前端登入)

```
┌─────────────────────────────────────────────────────────────────┐
│ Step 1: 客戶部署並啟動系統                                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  docker-compose up                                              │
│        │                                                        │
│        ├── Backend 啟動 (Go Core)                               │
│        │       └── 功能暫時鎖定，等待驗證                         │
│        │                                                        │
│        └── Frontend 啟動 (React)                                │
│                └── 顯示登入頁面                                   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ Step 2: 用戶在前端登入                                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  用戶輸入 Email + Password                                       │
│        │                                                        │
│        └── Frontend → Backend → License Server                  │
│                                        │                        │
│                    ┌───────────────────┴───────────────────┐    │
│                    │                                       │    │
│                    ▼                                       ▼    │
│            驗證成功                                   驗證失敗   │
│                │                                       │        │
│                ▼                                       ▼        │
│         Token 存入 Backend                      顯示錯誤訊息    │
│         功能解鎖                                (帳密錯誤/      │
│         進入主界面                               訂閱過期)       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ Step 3: 正常使用                                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  用戶正常操作系統                                                 │
│        │                                                        │
│        ├── Backend 使用緩存的 Token                              │
│        │                                                        │
│        └── 背景任務每 24 小時刷新 Token                           │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 後端狀態管理

```go
// 後端維護授權狀態
type LicenseState struct {
    IsAuthorized bool
    UserEmail    string
    ExpiresAt    time.Time
    Token        string
    LastRefresh  time.Time
}

// 未授權時，API 返回 401
if !licenseState.IsAuthorized {
    return 401, "請先登入"
}
```

### 離線處理

```
Token 最後刷新時間 + 7 天 = 離線截止時間

情況 1: 有網路
    → 正常刷新 Token

情況 2: 無網路但在寬限期內
    → 使用緩存 Token 繼續運行

情況 3: 無網路且超過寬限期
    → 功能鎖定，顯示 "請連線驗證"
```

---

## 🖥️ 管理後台功能

### 用戶管理

- 創建用戶
- 重設密碼
- 停用帳號

### 訂閱管理

- 創建訂閱
- 延長/縮短期限
- 取消訂閱

### 監控

- 登入記錄
- 活躍用戶數
- 訂閱到期提醒

---

## 📦 部署架構

```
┌─────────────────────────────────────────────────────────────┐
│                      你的基礎設施                            │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐   │
│  │   License   │     │   Database  │     │    Admin    │   │
│  │   Server    │────▶│  (SQLite/   │◀────│   Panel     │   │
│  │  (FastAPI)  │     │  PostgreSQL │     │   (React)   │   │
│  └─────────────┘     └─────────────┘     └─────────────┘   │
│         │                                                   │
└─────────┼───────────────────────────────────────────────────┘
          │ HTTPS
          ▼
┌─────────────────────────────────────────────────────────────┐
│                     客戶環境                                 │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                   Trading System                     │   │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐    │   │
│  │  │  Backend   │  │  Frontend  │  │  License   │    │   │
│  │  │ (Go Core)  │  │  (React)   │  │  Client    │    │   │
│  │  └────────────┘  └────────────┘  └────────────┘    │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## 🔧 實作清單

### Phase 1: License Server

- [ ] FastAPI 專案結構
- [ ] 用戶認證 API
- [ ] 訂閱管理 API
- [ ] JWT Token 簽發

### Phase 2: License Client

- [ ] `pkg/license/client.go` 驗證客戶端
- [ ] Token 緩存機制
- [ ] 啟動時驗證
- [ ] 背景刷新任務

### Phase 3: Admin Panel

- [ ] 用戶 CRUD
- [ ] 訂閱 CRUD
- [ ] 登入記錄查看

### Phase 4: 整合

- [ ] Trading System 啟動驗證
- [ ] 前端登入頁面
- [ ] 訂閱到期提醒

---

## ⚠️ 安全考量

| 風險 | 緩解措施 |
|------|----------|
| Token 被盜 | 短有效期 + IP 檢查 |
| 密碼洩漏 | bcrypt 加密 |
| 中間人攻擊 | HTTPS 強制 |
| 暴力破解 | 登入限速 |

---

## 📝 待確定項目

| 項目 | 選項 | 建議 |
|------|------|------|
| 訂閱週期 | 月/年 | 都支援 |
| 價格設定 | 自訂 | 待定 |
| 試用期 | 7/14/30 天 | 14 天 |
| 付款整合 | Stripe/PayPal | 手動 (初期) |

---

*實作將在系統更完整後進行*
