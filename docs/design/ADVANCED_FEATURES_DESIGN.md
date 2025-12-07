# Advanced Strategy Features Implementation Design

本文件詳細說明如何實現策略的 **生命週期控制 (Start/Pause/Stop)**、**參數修改**、**虛擬持倉 (Virtual Position)** 及 **一鍵平倉 (Panic Sell)**。

## 1. 核心概念：虛擬持倉 (Virtual Position)

由於交易所僅提供賬戶總持倉，而我們需要針對 "單個策略實例" 進行止盈止損或平倉，因此必須在數據庫層面維護每個策略的 **虛擬持倉**。

### 數據庫變更
新增 `strategy_positions` 表（或在 `strategy_states` 中擴充）：

```sql
CREATE TABLE strategy_positions (
    strategy_instance_id TEXT PRIMARY KEY,
    symbol TEXT NOT NULL,
    qty REAL DEFAULT 0,        -- 當前持倉數量 (+為多, -為空)
    avg_price REAL DEFAULT 0,  -- 平均開倉價格
    realized_pnl REAL DEFAULT 0, -- 已實現盈虧
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**更新邏輯**:
每次 `OrderExecutor` 確認訂單成交 (Filled) 時，除了更新總持倉外，還需根據 `Order.StrategyInstanceID` 更新該表。

## 2. 生命週期控制 (Lifecycle)

### 狀態定義
在 `strategy_instances` 表中增加 `status` 字段：
- `ACTIVE`: 正常運行
- `PAUSED`: 暫停（不產生新信號，但保留持倉）
- `STOPPED`: 停止（不產生信號，通常已平倉）
- `ERROR`: 異常停止

### 實現邏輯
在 `StrategyEngine` 中：
- **Pause**: 設置內存標誌 `paused = true`。`OnTick` 檢查此標誌，若為 true 則直接返回 nil。
- **Resume**: 設置 `paused = false`。
- **Stop**: 設置 `active = false`，並從 Engine 中移除該實例。

## 3. 一鍵平倉 (Panic Sell)

**流程**:
1. 用戶觸發 `POST /api/strategies/:id/panic_sell`。
2. 查詢 `strategy_positions` 獲取該策略的 `qty`。
3. 如果 `qty != 0`：
    - 生成一個反向的 `MARKET` 訂單 (Close Order)。
    - 訂單備註設為 "Panic Sell"。
    - 發送訂單到 `OrderQueue`。
4. 訂單成交後，`strategy_positions` 自動歸零。
5. 將策略狀態設為 `STOPPED`。

## 4. 參數修改 (Edit Params)

**流程**:
1. 用戶觸發 `PUT /api/strategies/:id/params` (Body: 新 JSON 參數)。
2. 更新 `strategy_instances` 表中的 `parameters` 字段。
3. **熱更新**:
    - `StrategyEngine` 檢測到變更。
    - 調用策略的 `UpdateParams(json)` 方法（需要在接口中新增）。
    - 或者：重啟該策略實例（銷毀舊對象，用新參數創建新對象，並恢復 State）。

## 5. 止盈止損 (TP/SL)

**實現方式**:
- **方式 A (策略內)**: 策略自己在 `OnTick` 中判斷價格，發出 Close 信號。
- **方式 B (風控層)**: 在 `RiskManager` 中維護 TP/SL 價格。
    - 當 `MarketData` 更新時，檢查是否觸發。
    - 若觸發，由 `RiskManager` 直接發送平倉訂單。
    - **推薦**: 方式 B，因為更可靠且統一。

## 6. API 接口設計

```http
POST /api/strategies/:id/start
POST /api/strategies/:id/pause
POST /api/strategies/:id/stop
POST /api/strategies/:id/panic_sell
PUT  /api/strategies/:id/params
```

## 7. 前端實現

在 `StrategyList` 組件中增加操作按鈕：
- [▶] Start / [⏸] Pause
- [⏹] Stop
- [🚨] Panic Sell (紅色警告按鈕)
- [⚙️] Edit Params (彈出模態框)
