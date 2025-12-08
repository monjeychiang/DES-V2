# 策略框架設計文檔

> **版本**: 2.0  
> **審核日期**: 2025-12-08  
> **狀態**: Phase 1 完成，Phase 2 規劃中

---

## 📋 目錄

1. [核心架構](#1-核心架構)
2. [實現狀態](#2-實現狀態)
3. [功能詳情](#3-功能詳情)
4. [待實現功能](#4-待實現功能)
5. [優先實施建議](#5-優先實施建議)

---

## 1. 核心架構

### 1.1 設計理念：策略即實例

```
策略實例 = 交易對 + K線週期 + 策略類型 + 參數
```

系統從「硬編碼策略」升級為「資料庫驅動的動態策略平台」。

### 1.2 資料庫結構

**位置**: `pkg/db/schema.go`

```sql
-- 策略實例表
CREATE TABLE IF NOT EXISTS strategy_instances (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    strategy_type TEXT NOT NULL,
    symbol TEXT NOT NULL,
    interval TEXT DEFAULT '1h',
    parameters TEXT,           -- JSON 格式
    is_active BOOLEAN DEFAULT 1,
    status TEXT DEFAULT 'ACTIVE',
    user_id TEXT,
    connection_id TEXT,
    created_at DATETIME,
    updated_at DATETIME
);

-- 策略狀態持久化
CREATE TABLE IF NOT EXISTS strategy_states (
    strategy_instance_id TEXT PRIMARY KEY,
    state_data TEXT,
    updated_at DATETIME
);

-- 策略倉位追蹤
CREATE TABLE IF NOT EXISTS strategy_positions (
    strategy_instance_id TEXT PRIMARY KEY,
    symbol TEXT,
    qty REAL,
    avg_price REAL,
    realized_pnl REAL,
    updated_at DATETIME
);
```

### 1.3 架構圖

```
┌─────────────────────────────────────────────┐
│              API Layer                       │
│  /api/strategies  /api/strategies/:id/*     │
└─────────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────┐
│           Engine Service                     │
│  StartStrategy / PauseStrategy / StopStrategy│
└─────────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────┐
│           Strategy Engine                    │
│  LoadStrategies / handleTick / WorkerPool   │
└─────────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────┐
│         Concrete Strategies                  │
│   MACross / RSI / Python Worker / ...       │
└─────────────────────────────────────────────┘
```

---

## 2. 實現狀態

### 2.1 核心框架

| 功能 | 狀態 | 位置 |
|------|------|------|
| DB 驅動載入 | ✅ | `engine.go:LoadStrategies` |
| 狀態持久化 | ✅ | `strategy_states` 表 |
| 歷史數據預熱 | ✅ | `internal/data/historical.go` |
| 策略工廠模式 | ✅ | `engine.go` switch-case |
| 倉位追蹤 | ✅ | `strategy_positions` 表 |

### 2.2 生命週期 API

| 端點 | 狀態 | 說明 |
|------|------|------|
| `GET /api/strategies` | ✅ | 列出所有策略 |
| `POST /:id/start` | ✅ | 啟動策略 |
| `POST /:id/pause` | ✅ | 暫停策略 |
| `POST /:id/stop` | ✅ | 停止策略 |
| `POST /:id/panic` | ✅ | 緊急平倉 |
| `PUT /:id/params` | ✅ | 更新參數 |
| `PUT /:id/binding` | ✅ | 綁定連線 |
| `GET /:id/performance` | ✅ | 績效數據 |

### 2.3 功能完成度

| 類別 | 完成度 |
|------|--------|
| 生命週期控制 | 100% |
| 資金管理 | 50% |
| 執行邏輯 | 33% |
| 高級觸發器 | 25% |
| 分析與標籤 | 0% |

---

## 3. 功能詳情

### 3.1 生命週期控制 ✅ 100%

| 功能 | 狀態 | 實現 |
|------|------|------|
| 開始/暫停/停止 | ✅ | `engine.Service` |
| Panic Sell | ✅ | 市價平倉 + 停止 |
| 狀態重置 | ✅ | 清除 `strategy_states` |
| 歸檔 | ✅ | `status='STOPPED'` |

### 3.2 資金與倉位管理 ⚠️ 50%

| 功能 | 狀態 | 說明 |
|------|------|------|
| 槓桿設置 | ✅ | 交易所 API |
| 最大持倉限制 | ✅ | `risk.Manager` |
| DCA / 補倉 | 📋 | 待實現 |
| 冷卻時間 | 📋 | 待實現 |

### 3.3 執行邏輯 ⚠️ 33%

| 功能 | 狀態 | 說明 |
|------|------|------|
| 多訂單類型 | ✅ | LIMIT/MARKET/STOP |
| Maker Only | 📋 | 需加 TIME_IN_FORCE |
| 交易時段 | 📋 | 待實現 |
| 滑點控制 | 📋 | 待實現 |

### 3.4 高級觸發器 ⚠️ 25%

| 功能 | 狀態 | 說明 |
|------|------|------|
| 止盈/止損 | ✅ | `StopLossManager` |
| 自動啟動條件 | 📋 | 待實現 |
| 利潤目標停止 | 📋 | 待實現 |
| Webhook 控制 | 📋 | 待實現 |

### 3.5 分析與標籤 📋 0%

| 功能 | 狀態 | 說明 |
|------|------|------|
| 標籤 (Tags) | 📋 | 可加欄位 |
| 備註 (Notes) | 📋 | 可加欄位 |
| 專屬日誌 | 📋 | 按 ID 過濾 |

---

## 4. 待實現功能

### 4.1 高優先級

| 功能 | 價值 | 工作量 |
|------|------|--------|
| 冷卻時間 | 減少過度交易 | 低 |
| Maker Only | 降低手續費 | 低 |
| 利潤目標停止 | 自動止盈 | 中 |

### 4.2 中優先級

| 功能 | 價值 | 工作量 |
|------|------|--------|
| DCA 設置 | 增加策略 | 中 |
| 滑點控制 | 風險控制 | 低 |
| 標籤系統 | 管理便利 | 低 |

### 4.3 長期規劃

| 功能 | 價值 | 工作量 |
|------|------|--------|
| Webhook 控制 | TradingView 整合 | 高 |
| 交易時段 | 精細控制 | 中 |
| 專屬日誌 | 除錯便利 | 中 |

---

## 5. 優先實施建議

### Phase 2 建議順序

```
1. 冷卻時間 (1 天)
   ├─ 新增 cooldown_until 欄位
   └─ 在 handleTick 中檢查

2. Maker Only (0.5 天)
   └─ 新增 TIME_IN_FORCE 參數

3. 利潤目標停止 (2 天)
   ├─ 追蹤累計 PnL
   └─ 達標時自動停止

4. DCA 設置 (3 天)
   ├─ 新增 DCA 參數表
   └─ 實現補倉邏輯
```

---

## 參考資源

- [系統架構](../architecture/SYSTEM_ARCHITECTURE.md)
- [服務架構路線圖](../architecture/SERVICE_ARCHITECTURE_ROADMAP_V2.md)
- [訂單生命週期](order_lifecycle_analysis.md)

---

*此文檔整合自 STRATEGY_FRAMEWORK_UPGRADE.md 和 STRATEGY_FEATURES_PROPOSAL.md*
