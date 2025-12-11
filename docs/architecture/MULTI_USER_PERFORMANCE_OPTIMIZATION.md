# 多用戶系統性能優化與改造計畫

> **版本**: 1.0  
> **日期**: 2025-12-10  
> **範圍**: `backend/cmd/trading-core` 多用戶架構 (auth / connections / orders / positions / risk / balance / gateway)

---

## 1. 背景與目標

多用戶架構（`MULTI_USER_REFACTORING.md`）與使用流程（`MULTI_USER_USAGE_GUIDE.md`）已經完成，功能與資料隔離均可透過整合測試驗證，例如：

- `TestMultiUserEndToEnd`：註冊 / 登入 / 建立加密連線 / 多用戶下單 / `/orders`、`/connections`、`/balance` 隔離。
- `TestMultiUserPositionsIsolation`：`positions` 的 `user_id` 寫入與 `/positions` 多用戶隔離。
- `TestMultiUserStrategyIsolation`：策略綁定 `user_id` / `connection_id`，禁用其他用戶操作。

在功能正確之後，下一階段目標：

1. 在多用戶情況下保持查詢與寫入的穩定延遲，避免某些熱點表或熱路徑拖垮整體延遲。
2. 控制記憶體佔用（特別是 per-user manager 與 gateway 緩存），避免隨用戶數線性成長。
3. 保持改造過程中行為與既有文件一致，並透過整合測試保障迭代安全。

---

## 2. 現況總結（與性能相關的設計）

**資料庫層**

- SQLite + WAL，單檔 DB (`DB_PATH`)，適合單實例部署。
- 多用戶欄位：
  - `orders.user_id`
  - `trades.user_id`
  - `positions.user_id`
- 查詢介面：`pkg/db/queries.go::UserQueries`，所有 `Get*ByUser` 都強制需要 `user_id`，否則回傳 `ErrUserIDRequired`。
- 目前查詢條件普遍採用：  
  `WHERE user_id = ? OR user_id IS NULL OR user_id = ''`  
  以支援舊資料與「全域」資料列。
- 索引（`pkg/db/schema.go`）：
  - `idx_orders_user_time ON orders(user_id, created_at)`
  - `idx_trades_user_time ON trades(user_id, created_at)`
  - `idx_positions_user ON positions(user_id)`

**持倉與 state manager**

- `positions` schema：`symbol TEXT PRIMARY KEY, qty, avg_price, user_id, updated_at`。
- DB 寫入：`Database.UpsertPosition(ctx, p)` 以 `symbol` 為唯一鍵。
- 多用戶查詢：`UserQueries.GetPositionsByUser` 用 `user_id` 過濾，但底層表實際上「每個 symbol 只存一列」。  
  → function 正確，但在多用戶大量交易時會形成 DB 寫入熱點。

**Gateway 緩存**

- `internal/gateway/manager.go::Manager`：
  - LRU + Idle timeout + Health check 的 gateway pool（`MaxSize`、`IdleTimeout`、`HealthInterval`）。
  - 透過 `UserQueries.GetConnectionByID` + `KeyManager` 解密，集中管理每個 `connection_id` 的 Gateway 實例。
- `internal/order/executor.go::Executor`：
  - 自行維護 `connGateways map[string]exchange.Gateway`，無大小上限與淘汰策略。
  - 目前尚未接入 `gateway.Manager`，造成雙重快取、管理分散。

**Per-user Risk / Balance**

- `internal/risk/multi_user.go::MultiUserManager`
  - `managers map[string]*risk.Manager`，目前沒有 TTL 或淘汰。
- `internal/balance/multi_user.go::MultiUserManager`
  - `managers map[string]*balance.Manager`，透過 factory 建立 per-user balance manager，同樣無淘汰機制。
- 適合「少量長期活躍用戶」場景，但在大量短生命週期 userID（例如 SaaS Tenant）情況下，記憶體會線性增加且無法回收。

**其他**

- 熱路徑中 log 較多（executor / gateway 失敗、DB 錯誤），高 QPS 下同步 IO 會放大 tail latency。
- 多數 API 都有合理的限制（例如 `GetOrdersByUser` 帶 `LIMIT`），但 positions / trades 若未加額外限制，在高頻交易與長期留存下會膨脹。  

---

## 3. 問題與優化方向總覽

1. **`user_id` 條件的 OR 導致索引利用不佳**  
   - 目前：`WHERE user_id = ? OR user_id IS NULL OR user_id = ''`。  
   - 影響：SQLite 難以只用 `(user_id, created_at)` 索引範圍掃描，orders / trades / positions 查詢在資料量大時會退化。

2. **`positions` 以 `symbol` 為 PK，不適合多用戶**  
   - 目前：單一 symbol 全系統只有一列，實務上多用戶下單會集中寫同一 row → 寫入鎖競爭與 WAL 衝突。

3. **Gateway 緩存分裂：Executor 內部 map 無淘汰機制**  
   - 目前同時存在 `gateway.Manager` 與 `Executor.connGateways` 兩份緩存，後者無大小限制，長期運行會累積大量 Gateway / 連線。

4. **Per-user Risk / Balance Manager 無界成長**  
   - `MultiUserManager.managers` map 沒有 TTL / Idle 清理，多租戶環境 user 數成長時會線性佔用記憶體。

5. **SQLite 單檔 DB 在高寫入負載下的瓶頸**  
   - 單 writer、多 reader 的特性，配上多 index / `ON CONFLICT`，在高頻 orders / trades / positions 寫入下會成為整體上限。

6. **同步 logging 對高頻熱路徑的影響**  
   - 大量 `log.Printf`（特別是失敗重試時）在高 QPS 下會佔用 CPU 與 IO，造成 tail latency 增長。

---

## 4. 優化計畫細節

### 4.1 DB 查詢與 `user_id` 條件優化

**問題**  

- OR 條件 `user_id = ? OR user_id IS NULL OR user_id = ''` 讓 Planner 無法單純使用 `user_id` index。
- 同時將「全域資料列」混在每個 user 的查詢結果中，使得 row 數膨脹。

**改造目標**

1. 查詢路徑可以穩定使用 `(user_id, created_at)` 或 `(user_id)` 索引。
2. 清楚區分「全域資料」與「多用戶資料」，避免在 hot-path 查詢中混合兩者。

**建議做法（分階段）**

1. **引入明確的全域 sentinel**  
   - 定義常數，例如 `GLOBAL_USER_ID = 'global'`。  
   -新寫入需要全域資料時，明確寫入 `user_id = 'global'`，而不是 `NULL / ''`。

2. **更新查詢條件**  
   - 將 `Get*ByUser` 的 WHERE 改成以下策略之一：
     - **方案 A（只看自己的資料）**  
       `WHERE user_id = ?`
     - **方案 B（包含全域 row）**  
       `WHERE user_id IN (?, 'global')`
   - 這兩種形式都能良好利用 `idx_*_user_time` 之類的索引。

3. **資料清理 / 過渡期處理**  
   - 增加一次性 migration：將舊資料中 `user_id IS NULL OR user_id = ''` 的 row 統一更新為 `'global'`。  
   - 可選：如不再需要全域 row，亦可選擇直接刪除或歸檔至歷史表。

4. **測試保障**  
   - 更新 `pkg/db/queries_test.go`，新增：
     - 「全域 row 不會被誤當成其他 user 的資料」或「全域 row 只出現在設計允許的查詢中」。
   - 更新 multi-user 整合測試，驗證 `/orders`、`/positions` 的行為符合預期。

---

### 4.2 `positions` 表的主鍵與多用戶持倉

**問題**

- 目前 schema：  
  `positions(symbol PRIMARY KEY, qty, avg_price, user_id, updated_at)`。  
  → 每個 symbol 系統只有一個 position row。
- 多用戶場景下，實際上每個 user 都可能在同一 symbol 有獨立持倉，現行 schema 會：  
  - 將所有 user 的持倉「疊在同一 row 上」，產生寫入熱點。  
  - 難以在 DB 層直接透過 `(symbol, user_id)` 做查詢與統計。

**改造目標**

1. 支援真正的「每 user 每 symbol 一列」持倉資料。
2. 降低單 row 寫入鎖競爭。

**建議做法（偏向安全的兩階段）**

1. **新增 user-aware 持倉表（推薦）**
   - 新增 `user_positions`：  
     ```sql
     CREATE TABLE user_positions (
       symbol   TEXT NOT NULL,
       user_id  TEXT NOT NULL,
       qty      REAL NOT NULL,
       avg_price REAL NOT NULL,
       updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
       PRIMARY KEY (symbol, user_id)
     );
     CREATE INDEX idx_user_positions_user ON user_positions(user_id, symbol);
     ```
   - 在 `UserQueries.UpsertPositionWithUser` / `GetPositionsByUser` 中改寫為對 `user_positions` 操作。
   - `positions` 原表暫時保留給單用戶舊邏輯與現有報表，避免一次性大改。

2. **同步更新 `state.Manager` 與 Engine**
   - `internal/state/manager.go::RecordFill` / `Positions()` 需要切換到 user-aware 模型：  
     - key 由 `symbol` 改為 `symbol + user_id`。  
     - DB 持久化改為呼叫 `UpsertPositionWithUser`。
   - Engine / API：`GetPositionsByUser` / `/api/v1/positions` 統一使用 `user_positions`。

3. **清理與合併（第二階段）**
   - 確認所有呼叫點都從 `positions` 遷移至 `user_positions` 後：  
     - 可以將 `positions` 標記為 deprecated，僅作兼容用。  
     - 若不再需要單用戶模式，可設計 migration 將 `user_positions` 併回 `positions`（此時主鍵採 `(symbol, user_id)`）。

---

### 4.3 Gateway 緩存統一管理

**問題**

- `internal/gateway/manager.go::Manager` 已實作完善的多連線 Gateway pool，但 `Executor` 仍有自己的 `connGateways` map：
  - 無 `MaxSize` / Idle 清理，長期運行會累積大量 Gateway 實例。
  - 健康檢查 / 失敗重試邏輯分散，難以統一調優。

**改造目標**

1. 所有 per-connection Gateway 皆由 `gateway.Manager` 管理。  
2. Executor 不再保存自己的無界 gateway map。

**建議做法**

1. 在 `main.go` 中：
   - 新增並啟動單一 `gateway.Manager`：  
     ```go
     userQueries := db.NewUserQueries(database.DB)
     gwMgr := gateway.NewManager(userQueries, keyMgr, gatewayFactory, gateway.DefaultConfig())
     gwMgr.Start(ctx)
     ```

2. 調整 `order.Executor`：
   - 新增依賴：`GatewayPool interface { GetOrCreate(ctx context.Context, userID, connectionID string) (exchange.Gateway, error) }`。  
   - 將 `gatewayForConnection` 內部的 DB 查詢與解密邏輯改為呼叫 `GatewayPool.GetOrCreate`。  
   - 移除 `connGateways map[string]exchange.Gateway` 與 `createGateway` 中的實體建構，避免雙重邏輯。

3. 錯誤處理與統計：
   - 若 `GatewayPool.GetOrCreate` 回傳 `ErrGatewayUnhealthy` / `ErrPoolFull` 等錯誤，在 Executor 中統一轉換為 user-facing 錯誤訊息或重試策略。

4. 測試
   - 對 `gateway.Manager` 保留單元測試，並在 multi-user 整合測試中覆蓋 per-connection 下單路徑，確保改造不改變行為（尤其是加密連線錯誤情境）。

---

### 4.4 Per-user Risk / Balance Manager 記憶體控制

**問題**

- `risk.MultiUserManager.managers` 與 `balance.MultiUserManager.managers` 目前無任何淘汰策略：  
  - 一旦某個 `userID` 被使用就會常駐。  
  - 在 tenant/user 數量級很大或 user 流動頻繁時，map 的大小會線性成長。

**改造目標**

1. 對長期無活動的 user manager 進行釋放。  
2. 保持 active users 的風險與餘額計算在記憶體中，以降低 DB 查詢頻率。

**建議做法**

1. 為 per-user manager 增加 `LastAccess` 欄位：  
   - 在 `GetOrCreate` / `Get` / `EvaluateForUser` / `UpdateMetricsForUser` 等 API 中更新 `lastAccess`。

2. 新增背景清理 goroutine：  
   - 例如每 10 分鐘掃描一次 user managers，將 `LastAccess` 超過 N 分鐘（可配置，如 60 分鐘）的移除。  
   - 移除前可選擇持久化必要資訊（如統計 metrics）到 DB。

3. 加入上限與監控：  
   - 增加 `MaxUsers` 配置，超過上限時拒絕再建立新的 in-memory manager（可回退到較慢但安全的 DB 方案）。  
   - 暴露 `/metrics` 中關於 `user_risk_managers` / `user_balance_managers` 的數量與命中率。

---

### 4.5 SQLite 寫入與未來擴展

**現況**

- SQLite + WAL 非常適合單實例策略引擎與桌面應用，但在高寫入負載與多租戶 SaaS 場景會遇到：
  - 單 writer 限制。
  - 單檔 DB 成為 I/O 熱點。

**建議**

1. **短期**：在維持 SQLite 的前提下，透過前述 schema / 索引優化降低鎖競爭。  
2. **中期**：若確定要支援多節點部署或極高用戶數，預先設計「可遷移到 Postgres」的邏輯邊界：  
   - 將 DB 存取集中在 `pkg/db` 模組與少數 service 層方法中。  
   - 避免在其他層直接寫 raw SQL，降低未來遷移成本。

---

### 4.6 Logging 與監控

**問題**

- 熱路徑（Executor / Gateway / Queries）中使用同步 `log.Printf`，在高 QPS 下：
  - 影響 CPU 與 IO。  
  - 難以快速分析 per-user 或 per-connection 問題。

**建議**

1. 對正常路徑減少 verbose log，僅在錯誤或重大事件時記錄。  
2. 考慮引入 structured logging 介面（包裝 log），為每筆重要記錄帶上 `user_id` / `connection_id` / `strategy_id`。  
3. 在 `monitor.SystemMetrics` 中增加 multi-user 指標：
   - 活躍 users 數量。
   - per-user request latency 分佈（可以先從簡單累計開始）。

---

## 5. 實施順序建議

建議按風險與影響範圍分階段進行：

1. **Phase 1 – 查詢與索引優化（低風險）**
   - 引入全域 user sentinel。  
   - 調整 `Get*ByUser` 的 WHERE 條件與索引使用。  
   - 補充 / 更新對應單元測試與整合測試。

2. **Phase 2 – Gateway 緩存統一化（中風險，無 schema 變更）**
   - 接入 `gateway.Manager`，移除 Executor 內部 map。  
   - 確保 manual orders / strategy orders 路徑在測試中全覆蓋。

3. **Phase 3 – Per-user 持倉表與 state manager（中高風險，涉及 schema 與 state）**
   - 新增 `user_positions` 表與對應查詢。  
   - 逐步將 state manager 與 Engine / API 切到新表。  
   - 保留回滾計畫與資料遷移腳本。

4. **Phase 4 – Per-user Manager 記憶體控制（中風險）**
   - 為 risk/balance 加入 idle 清理與上限。  
   - 觀察長時間壓測下的記憶體曲線。

5. **Phase 5 – DB 層長期演進（高風險，僅在必要時）**
   - 視實際負載與運維需求，評估是否將核心交易資料從 SQLite 遷移到 Postgres 或其他多節點友善的 DB。

---

## 6. 下一步

- 針對本文件中每個 Phase：
  - 在 `ARCHITECTURE_ROADMAP_V3.md` / `PERFORMANCE_ANALYSIS.md` 中掛上連結與預估工期。  
  - 對應建立 Git 分支（例如 `feature/multi-user-performance-phase1` 等），並在 PR 模板中要求：
    - 說明使用者可見行為是否改變。  
    - 列出新增或更新的整合測試名稱。

此文件作為多用戶系統性能優化的藍圖，後續實作與測試計畫應以此為基準進一步細化。  
## 6. 實作進度與後續

- 目前已完成的項目（對照本文件各節）：
  - **4.1 user_id 查詢條件優化**：`pkg/db/queries.go` 的 `GetOrdersByUser`、`GetOpenOrdersByUser`、`GetTradesByUser`、`GetPositionsByUser` 已全部改為嚴格 `WHERE user_id = ?`，不再混用 `NULL/''`，實際查詢都能吃到 `idx_*_user_*` 索引。
  - **4.2 user_positions 表與 per-user 持倉**：`schema.go` 已建立 `user_positions` 以及 `idx_user_positions_user`；`UserQueries.GetPositionsByUser` / `UpsertPositionWithUser` 改為操作 `user_positions`，`state.Manager.RecordFill` 會同步寫入 legacy `positions` 以及 `user_positions`。
  - **4.3 Gateway 緩存統一管理**：`internal/gateway/manager.go` 作為單一 GatewayPool 實作；`internal/order/executor.go` 的 `gatewayForConnection` 與 `gatewayForStrategy` 會優先走 `GatewayPool.GetOrCreate(userID, connectionID)`，僅在未注入 Pool 時才回退到舊的 `connGateways` 快取路徑。
  - **4.4 Per-user Risk / Balance Manager 記憶體控制**：`risk.MultiUserManager` 與 `balance.MultiUserManager` 已加入 `lastSeen` 與 `CleanupIdle(ttl)`；`main.go` 透過背景 goroutine 每 10 分鐘呼叫一次 cleanup，TTL 預設 60 分鐘，避免 managers map 無界成長。
  - **整合測試覆蓋**：`go test ./...`（含 `trading-core/test` 下的多用戶整合測試）在上述改動後全部通過，確認功能與隔離行為與 `MULTI_USER_USAGE_GUIDE.md` 一致。

- 後續建議：
  - 在 `ARCHITECTURE_ROADMAP_V3.md` / `PERFORMANCE_ANALYSIS.md` 中引用本文件，並標註「已完成」與「規劃中」的子項目，方便追蹤。
  - 新增針對壓測場景的專門測試腳本（多 user、多 connection、高頻下單），觀察 GatewayPool、per-user manager 數量與 SQLite 寫入延遲，作為之後是否需要進一步 DB 拆分或遷移的依據。

此附錄描述的是「目前已實作的優化項目」，與上方各節的設計說明一起閱讀，可以快速了解系統在多用戶場景下的實際行為與性能保護機制。

---

## 7. 驗證優化成果與壓力測試計畫

### 7.1 壓力測試計畫（Load / Stress Testing）

目標：在接近實際 SaaS 場景與高頻交易場景下，驗證：

- SQLite 在多用戶、多連線、高寫入負載下的鎖競爭情況與延遲（有無頻繁 `database is locked`）。  
- GatewayPool 在大量 connection / user 下的快取命中率與 LRU/Idle 清理是否生效。  
- 多用戶模式（user_id + connection_id 路由）在高併發下仍能維持資料隔離與穩定延遲。

**建議新增測試檔案**

- 路徑：`backend/cmd/trading-core/test/stress/multi_user_stress_test.go`
- 性質：標記為壓力測試，可用 build tag（例如 `//go:build stress`）或獨立 test suite 控制，不必在每次 CI 皆執行。

**場景 A：多用戶 SaaS 場景**

- 模擬參數（可調整）：
  - 使用者數量：100–500 users。  
  - 每個 user 建立 1–3 個 `connections`（spot / futures 混合）。  
  - 每個 user 每秒發出少量下單請求（例如 1–5 req/s），總 QPS 接近實際預期上限。  
- 測試步驟（概念）：
  1. 啟動 trading-core，使用內部 HTTP client 或直接呼叫 API 層：註冊多個 user，為每個 user 新增加密 connection。  
  2. 併發呼叫 `/api/v1/orders`（帶 `Authorization` + `connection_id`），持續 N 分鐘。  
  3. 在測試過程中定期呼叫 `/api/v1/connections`、`/api/v1/orders`、`/api/v1/balance` 驗證資料隔離。  
  4. 結束後檢查：
     - 無或極少 `database is locked` / 5xx。  
     - GatewayPool 中的 `TotalGateways` 接近「實際活躍 connection 數量」而非 user × connections 上限。  
     - `/orders` 延遲分佈在可接受範圍內（在 monitor 指標中觀察）。

**場景 B：單一用戶高頻交易**

- 模擬參數：
  - 單一 user，1–2 個 connections。  
  - 單一 user 持續高頻下單（例如 50–200 req/s），持續 N 分鐘。  
- 測試重點：
  - 檢查 SQLite 寫入延遲、`orders/trades/user_positions` 的寫入成功率與錯誤率。  
  - 檢查 GatewayPool 是否維持少量 Gateway 實例（避免每次建新連線）。  
  - 檢查 per-user balance / risk manager 的 UserCount 是否穩定在預期範圍。

### 7.2 驗證記憶體回收（CleanupIdle 行為）

目標：確認 4.4 所述的 `CleanupIdle` 機制確實生效，避免 per-user manager map 無界成長。

**單元測試建議**

- 檔案：`backend/cmd/trading-core/internal/risk/multi_user_test.go`  
  - 建立 `MultiUserManager`，連續呼叫 `GetOrCreate("userA")`、`GetOrCreate("userB")`。  
  - 人為將 `lastSeen["userA"]` 調整為「現在時間 - 2*TTL」，`userB` 保持為「現在」。  
  - 呼叫 `CleanupIdle(TTL)` 後，驗證：
    - `UserCount()` 只剩 1。  
    - `Get("userA") == nil`，`Get("userB") != nil`。

- 檔案：`backend/cmd/trading-core/internal/balance/multi_user_test.go`  
  - 類似方式測試 `balance.MultiUserManager` 的 `CleanupIdle`，確保 idle user 的 manager 被回收。

**整合測試方向**

- 在 `test/multi_user_integration_test.go` 或新的 `test/stress` 測試中：  
  - 建立多個 user 並觸發 per-user balance / risk 行為。  
  - 等待超過 TTL 後，直接呼叫 `CleanupIdle`（或透過 main 中的 goroutine 觀察一段時間），確認 `UserCount` 回落到合理值。  
  - 注意避免依賴真實時間過長，可透過將 TTL 作為參數注入，或在測試時使用較小 TTL（例如數秒）縮短等待時間。

### 7.3 運維監控指標（Observability）

目標：在生產環境觀察優化效果，及早發現 GatewayPool、per-user managers 或 DB 鎖競爭問題。

**監控建議**

- 在 `internal/monitor` 或 API `/metrics`（若已有 Prometheus 風格輸出）中新增指標：
  - **Gateway Pool**
    - `gateway_pool_total_gateways`：目前 pool 中 Gateway 實例數。  
    - `gateway_pool_unhealthy_gateways`：處於 unhealthy 狀態的連線數（來自 `PoolStats.UnhealthyCount`）。  
    - `gateway_pool_evictions_total`：LRU 或 idle cleanup 造成的 Gateway 移除次數（需在 `Manager` 中累計）。  
  - **Per-user Managers**
    - `multiuser_risk_active_users`：`risk.MultiUserManager.UserCount()`。  
    - `multiuser_balance_active_users`：`balance.MultiUserManager.UserCount()`。  
    - 若有需要，可再加 `multiuser_*_cleanup_runs_total`、`multiuser_*_cleanup_removed_total` 計數。  
  - **DB 鎖與延遲**
    - 若後續加上 DB wrapper，可記錄 `db_write_latency`、`db_locked_errors_total`，針對 `orders/trades/user_positions` 特別觀察。

**落地方式**

- 建議在 `monitor.SystemMetrics` 中加入上述欄位，並在主程式定期（例如每秒或每數秒）從 GatewayManager、MultiUserManager 讀取當前狀態，更新 metrics。  
- 若已提供 `/metrics` HTTP 端點，將這些指標列入輸出，方便接入 Prometheus / Grafana 之類的監控系統。

透過上述壓力測試與監控指標，可以在實際運行環境中持續驗證本文件中性能優化的效果，並為下一階段（例如 DB 拆分或遷移）提供量化依據。

---

## 8. 測試索引（對應上述場景）

- **整合測試（HTTP 層）**  
  - `test/multi_user_integration_test.go`  
    - `TestMultiUserEndToEnd`：對應使用指南全流程（註冊/登入/下單/隔離）。  
    - `TestMultiUserPositionsIsolation`：檢查 `/positions` 只回本人的 `user_positions`。  
    - `TestMultiUserStrategyIsolation`：策略綁定 `user_id`/`connection_id`，跨用戶操作被拒。  
    - `TestMultiUserConnectionOwnershipEnforced`：惡意使用他人 `connection_id` 下單應失敗（場景 1）。  
    - `TestSingleUserMultiConnectionsOrders`：同一用戶多連線下單，`orders.user_id/connection_id` 正確（場景 2）。

- **資料庫層單元測試**  
  - `pkg/db/queries_test.go`  
    - `TestUserQueriesRequireUserID`：所有 `Get*ByUser` 必須帶 `user_id`。  
    - `TestUserQueriesDataIsolation`：不同 user 的 orders 隔離。  
    - `TestUserPositionsConcurrentUpserts`：多 user 同一 symbol 併發 upsert，`user_positions` 正確（場景 3）。

- **壓力測試（需 `-tags=stress`）**  
  - `test/stress/multi_user_stress_test.go`  
    - `TestMultiUserGatewayPoolStress`：多 user 多 connection 併發命中 GatewayPool，驗證快取/解密/LRU 在併發下穩定。

> 尚未自動化的場景：  
> - 高延遲 Gateway 的端到端行為（場景 5）：可在 Gateway 假實作加 `time.Sleep`，用 HTTP 發多筆下單觀察 `/metrics` 與最終狀態。  
> - 手動下單的 per-user 風控/餘額預檢（場景 4）：目前風控主用於策略路徑，可在 `/orders` 加預檢後補測。
