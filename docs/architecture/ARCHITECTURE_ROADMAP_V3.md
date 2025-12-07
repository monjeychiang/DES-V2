# DES-V3 架構路線圖（更新版）

版本：1.1  
更新日期：2025-12-01  

本文描述在目前 DES-V2 穩定可用的前提下，如何分階段升級到更模組化、可回測與可優化的 V3 架構。重點是「小步演進」，避免一次動太多核心路徑。

---

## 0. 目前基礎（V2 現況）

- 事件驅動：
  - 價格：`EventPriceTick`
  - 策略訊號：`EventStrategySignal`
  - 成交：`EventOrderFilled`
- 交易核心：
  - `order.Executor` + `DryRunExecutor`，下單前有風控 / 餘額鎖定。
  - `state.Manager` 管理 `positions`，從 DB 開機恢復。
  - `risk.Manager` 有基礎風控（單筆大小、曝險、日虧、日交易次數），並將實際 PnL（含手續費）寫入 `risk_metrics`。
- 交易所對接：
  - Binance REST / WebSocket 已封裝，`market.Feed` + user data stream 已實測。
  - DRY_RUN 模式與實盤 DB 已分離（`DB_PATH` vs `DRY_RUN_DB_PATH`）。

在這個基礎上，V3 的目標是：**把策略實例、PnL 歸因、行情服務、回測/優化** 分離成清楚的層次。

---

## 1. Phase 1：策略實例化（Strategy as Instance）

### 1.1 目標

用資料庫管理「策略實例」，讓系統啟動時從 DB 載入要跑的策略，而不是在 `main.go` 裡硬編碼。

### 1.2 設計

- 新增 `strategy_instances` 表：

  ```text
  strategy_instances(
    id               TEXT PRIMARY KEY,   -- 例如 uuid
    name             TEXT,               -- 顯示名稱，如 "BTC_1h_MA"
    strategy_type    TEXT,               -- "ma_cross", "grid", "rsi"...
    symbol           TEXT,               -- "BTCUSDT"
    kline_interval   TEXT,               -- "1m", "1h" ...
    parameters       TEXT,               -- JSON: {"fast":10,"slow":30,"size":0.01}
    is_active        INTEGER,            -- 1 啟用 / 0 停用
    created_at       DATETIME,
    updated_at       DATETIME
  )
  ```

- `strategy.Engine`：
  - 新增一個載入方法，例如 `LoadFromDB(db *sql.DB)`：
    - `SELECT * FROM strategy_instances WHERE is_active=1`
    - 依 `strategy_type` + `parameters` 建構對應的 `Strategy` 實作。
    - `engine.Add(instanceID, strategy)`（保留 instanceID，用於之後 PnL 歸因）。
  - `main.go` 只負責呼叫 `engine.LoadFromDB(...)`，若 DB 無任何實例，可寫入一筆預設實例當作 bootstrap。

### 1.3 準備工作

- 將目前 `main.go` 裡硬編碼的 `Add(NewGridStrategy(...))` 改成：
  - 若 DB 無實例 → 寫入一筆預設實例。
  - 否則完全改用 DB 驅動。

---

## 2. Phase 2：策略層 PnL / 持倉歸因

### 2.1 目標

讓每一筆 order / trade / position 都知道「來自哪一個策略實例」，方便算出：

- 每個策略的實現損益（Realized PnL）
- 每個策略的當前持倉 / 曝險
- 未來可以根據策略表現做風控或自動調整 size

### 2.2 設計

- DB schema 調整：
  - `orders` 表新增 `strategy_instance_id TEXT`。
  - `trades` 表新增 `strategy_instance_id TEXT`（跟單對應）。
  - `positions` 表可選擇也帶 `strategy_instance_id`，或只維持 per-symbol，改由查詢 orders/trades 聚合。
- `order.Order` 結構新增：

  ```go
  StrategyID string
  ```

- 下單流程：
  - 在策略引擎轉成 `order.Order` 時，將該實例的 `id` 填入 `StrategyID`。
  - `Executor` 寫 DB 時，帶入 `strategy_instance_id`。

- PnL/持倉歸因：
  - 現階段不急著多一張 `strategy_positions` 表。
  - 可以透過查詢：
    - `SELECT SUM(pnl) FROM trades WHERE strategy_instance_id = ?`
    - `SELECT SUM(qty) FROM positions WHERE strategy_instance_id = ?`（若 positions 也分策略）
  - 若後續需求明確，再考慮增加 `strategy_positions` 快取表。

---

## 3. Phase 3：策略狀態持久化（只針對需要的策略）

### 3.1 目標

讓特定策略（例如 Grid、複雜多腿策略）在重啟後能恢復「內部狀態」，不必完全重新建倉。

### 3.2 設計建議（採用漸進式，不做過度通用）

- 新增 `strategy_states` 表：

  ```text
  strategy_states(
    strategy_instance_id TEXT PRIMARY KEY,
    state_data           TEXT,           -- JSON
    updated_at           DATETIME
  )
  ```

- 對於需要狀態的策略，手動實作：

  ```go
  type StatefulStrategy interface {
      Strategy
      GetState() (map[string]any, error)
      SetState(map[string]any) error
  }
  ```

- 啟動流程：
  - `LoadFromDB` 時，如果該策略支援 `StatefulStrategy`，就嘗試從 `strategy_states` 讀 `state_data`，呼叫 `SetState`。
- 關閉 / 定期保存：
  - 可在：
    - 定時 job
    - 或 `EventOrderFilled` 後  
    將 `GetState()` 的結果寫回 `strategy_states`。

> 注意：並非所有策略都需要 state 持久化，像簡單 MA/RSI 用歷史 K 線即可恢復，不必勉強實作通用 state 機制。

---

## 4. Phase 4：行情服務與回測/優化（長期目標）

### 4.1 MarketData Service 抽象

在現有 Binance REST/WS 及 `market.Feed` 已穩定的基礎上，再抽一層：

- `MarketDataService` 提供：
  - `SubscribeKlines(symbol, interval) -> events.EventPriceTick`
  - 未來可擴展 Depth / Trades 等事件。
- `main.go` 只關心「訂閱事件」，不直接接觸 Binance 具體 API。

此步驟影響大，建議在 Phase 1–3 穩定後再做。

### 4.2 Backtester / Optimizer

**Backtester**

- 新增 `cmd/backtester` 或 `scripts/backtester`：
  - 讀取歷史 K 線（DB / CSV）。
  - 使用現有 `strategy.Engine` / `risk.Manager` / `stateMgr`，在單機模擬下 replay。
  - 結果寫入一顆獨立的 backtest DB（避免污染實盤/DRY_RUN）。

**Optimizer**

- 站在 Backtester 之上，跑參數組合：
  - `Optimize(strategyType, paramRanges)` → 多次呼叫 backtester，聚合結果。
  - 僅做 CLI/腳本，不直接掛在線上引擎，以避免複雜度過高。

---

## 5. 實作優先順序總結

1. **Phase 1：DB 驅動的策略實例 (`strategy_instances`)**
   - 讓「跑哪些策略」完全由 DB 控制，`main.go` 不再硬寫實例。

2. **Phase 2：在 orders/trades 上掛 `strategy_instance_id`，完成 per‑strategy PnL 歸因**
   - 先不急著多一張 `strategy_positions`，用查詢聚合即可。

3. **Phase 3：針對「有需要」的策略逐一加 state 持久化**
   - Grid 等需要複雜狀態的策略優先，其它維持無 state 或從歷史 K 線恢復。

4. **Phase 4：行情服務抽象 + Backtester / Optimizer**
   - 等前三階段穩定，才考慮重構 marketdata 與加入回測/優化框架。

整體目標是：在不破壞現有穩定交易核心的前提下，逐步將策略、行情、風控、回測拆成清楚的模組，使 DES-V3 具備「可組合的策略實例」、「明確的 PnL 歸因」與「易於擴充的回測/優化能力」。
