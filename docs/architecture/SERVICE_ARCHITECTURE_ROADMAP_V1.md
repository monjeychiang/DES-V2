# DES Trading System V2.0 - 服務架構演進路線 V1

> **版本**: 1.0  
> **日期**: 2025-12-08  
> **相關文件**:  
> - 系統總體架構: `docs/architecture/SYSTEM_ARCHITECTURE.md`  
> - 性能分析: `docs/architecture/PERFORMANCE_ANALYSIS.md`  
> - 性能優化計畫: `docs/roadmap/PERFORMANCE_IMPROVEMENT_PLAN_V1.md`

---

## 1. 背景與目標

DES V2.0 目前採用 **單一 Go 服務 (trading-core)** 搭配 React 前端與 Python 策略層：

- `backend/cmd/trading-core/main.go` 啟動：
  - HTTP API（REST + WebSocket）
  - 策略引擎、事件匯流排、order queue、風險控制、State/Reconciliation 等 goroutine
- `internal/*` 包含所有執行路徑：market feed、strategy、risk、order、state、DB、monitoring
- `python/` 透過 gRPC 與 Go backend 整合
- `license-server/` 為獨立授權服務

目前架構在單機/小規模下是合理的，但長期來看會面臨：

- 功能持續增加，單一程式碼庫與 binary 複雜度上升
- 不同功能模組的部署與擴縮需求不一致（例如：API QPS ↑，但交易核心仍需維持低延遲）
- 團隊人數增加後，模組之間變更的耦合成本偏高

因此需要一條 **「先在單一服務內做好邊界 ⇒ 再適度拆分服務」** 的演進路線，而不是一開始就走到全面微服務。

---

## 2. 設計原則

1. **關鍵路徑優先**  
   - Tick → Strategy → Risk → Order → Exchange 這條路徑必須保持在最少跳數與最少依賴中，避免為了「好看」的微服務拆分而犧牲延遲與穩定性。

2. **先有清楚邏輯邊界，再談物理拆分**  
   - 先在單一 binary 裡用清楚的 package/interface 劃清 Engine / Control 邊界，等契約穩定後再考慮拆成獨立部署。

3. **協定優先 (contract-first)**  
   - 任何未來可能拆出的服務，都應先有 **明確且穩定的介面**（例如 gRPC proto），避免日後出現「拆開後才發現介面不夠用」的重工。

4. **觀測與度量先行**  
   - 在大幅改動服務邊界前，要能量測 P50/P99 latency、QPS、goroutine 數、DB latency 等，避免盲目優化。

5. **簡單勝於複雜**  
   - 在團隊規模與流量尚未真的需要前，避免引入過多服務數量與新基礎設施（e.g. Kafka、K8s），降低運維成本。

---

## 3. 目前實作：單一 trading-core 服務

### 3.1 邏輯構成

- **API 層** (`internal/api`)
  - HTTP handler: `/api/strategies`, `/api/orders`, `/api/balance`, `/api/auth/*`, `/api/system/status` 等
  - Middleware: Auth、CORS、安全標頭、Rate limiting

- **市場資料與事件**  
  - `internal/market` + `pkg/market/binance/...`：Binance REST/WebSocket、price feed
  - `internal/events`：In-memory event bus，策略/監控/下單等元件透過事件解耦

- **策略與指標**  
  - `internal/strategy`：策略引擎、策略生命週期、策略配置載入 (`strategies.yaml`)
  - `internal/indicators`：MA、RSI、Bollinger 等技術指標

- **風險與資金管理**  
  - `internal/risk`：風險檢查、停損/停利、daily loss limit 等
  - `internal/balance`：餘額管理、鎖定/釋放資金、DryRun 初始資金

- **訂單執行與狀態**  
  - `internal/order`：order queue、executor、exchange gateway routing、DryRun executor
  - `internal/state`：持倉狀態、Position 管理
  - `internal/reconciliation`：與交易所對帳

- **資料持久化**  
  - `internal/persistence` + `pkg/db`：SQLite schema、migrations、查詢

- **輔助元件**  
  - `internal/monitor`：告警與監控
  - `pkg/exchanges/*`：Binance spot/futures gateway 實作
  - `proto/strategy.proto`：Go ↔ Python 策略層 gRPC 介面

### 3.2 目前問題與限制（高層級）

- API 層與交易核心高度耦合（`internal/api` 很多地方直接操作內部型別）
- 所有功能都在同一 binary 中，部署時無法針對負載型態做差異化擴縮
- 欠缺清楚的「engine vs control」介面，未來要拆服務時會卡在大量 refactor

---

## 4. Phase 1：單服務內先完成邏輯邊界重構

**目標時間**：短期（1–2 週，可以與性能優化 Phase 1 並行）  
**結果**：仍然是一個 `trading-core` 服務，但內部有明確邏輯邊界與介面。

### 4.1 邊界切分：Engine vs Control

#### Engine (Trading Engine Core)

- 職責：
  - 接收市場 tick（WebSocket / REST 補資料）
  - 更新指標、觸發策略邏輯
  - 執行風險檢查
  - 產生訂單、送往交易所或 DryRun executor
  - 更新持倉狀態、寫入 DB
  - 發佈事件給監控與 UI
- 主要 package：
  - `internal/market`
  - `internal/indicators`
  - `internal/strategy`
  - `internal/risk`
  - `internal/order`
  - `internal/state`
  - `internal/reconciliation`
  - `internal/events`
  - `internal/persistence` + `pkg/db`

#### Control (Control/API Layer)

- 職責：
  - HTTP/REST API 對外介面
  - 使用者與 Auth、連線管理 (connections)、策略 CRUD
  - 查詢型操作：讀取 orders/positions/trades/PnL、系統狀態
  - 將使用者操作轉成 Engine 的命令（Start/Stop/Panic/UpdateParams 等）
- 主要 package：
  - `internal/api`
  - 部分 `pkg/config` / auth/連線管理的輔助模組

### 4.2 定義 Engine 介面（程式內部的「服務介面」）

在 `internal/engine`（或等效位置）新增一個介面層，例如概念上：

- 指令介面（Command-like）：
  - `StartStrategy(id, params)`
  - `PauseStrategy(id)`
  - `StopStrategy(id)`
  - `PanicStrategy(id)`
  - `UpdateStrategyParams(id, params)`
  - `BindStrategyConnection(id, connectionID)`

- 查詢介面（Query-like）：
  - `ListStrategies(filter)`
  - `GetStrategyStatus(id)`
  - `GetPositions(filter)`
  - `GetOpenOrders(filter)`
  - `GetRiskMetrics(filter)`

在 Phase 1 中，這些介面仍然是 **同一個 process 裡的 Go interface**，`internal/api` 只透過這些介面存取核心，不直接操作底層 struct。  
此舉的目的，是為 Phase 2 的「換成 gRPC 介面」預先打底。

### 4.3 DB 操作政策

- **寫入**：一律由 Engine 模組負責寫入 DB（orders、trades、positions、metrics 等）。
- **讀取**：
  - Engine：用於內部風險計算、狀態恢復等。
  - Control：讀取報表與查詢資料，但不得直接修改交易相關核心資料表。

這樣可以確保未來拆服務時，只需要保證 Engine 對 DB 有寫入權限，Control 可變成 read-only 客戶端。

---

## 5. Phase 2：拆成 trading-engine + control-api 兩個服務

**時間點**：在 Phase 1 邊界穩定、且觀察到流量/團隊需求有必要拆分時  
**目標**：保持關鍵交易路徑在單一服務內，同時把人機介面與管理功能獨立出去。

### 5.1 服務角色與責任

#### Service A：trading-engine

- **職責**
  - 接收市場資料（WebSocket / REST）
  - 執行策略與指標更新
  - 做風險檢查與下單
  - 更新持倉與風險/績效資料
  - 對外提供 **gRPC 介面** 給 control-api 與其他內部服務使用
- **對外介面**
  - gRPC 服務（例如可建立 `proto/engine.proto`）：
    - `StartStrategy`, `PauseStrategy`, `StopStrategy`, `PanicStrategy`
    - `UpdateStrategyParams`, `UpdateStrategyBinding`
    - `GetStrategyStatus`, `ListStrategies`
    - `GetPositions`, `GetOrders`, `GetPerformance`
  - 可選：內部 event stream（NATS/Kafka）作為長期目標

#### Service B：control-api

- **職責**
  - 對前端提供 HTTP/REST/JSON API
  - Auth、使用者、連線（connections）管理
  - 策略 CRUD 與管理介面
  - 報表與查詢（PnL、orders、positions）；可直接讀 DB 或透過 trading-engine 提供的 Query API
  - 將 UI 操作轉為 gRPC 呼叫給 trading-engine

- **對外介面**
  - REST API：與現在的 `/api/*` 類似，但實作改為呼叫 trading-engine
  - 對 trading-engine 的 gRPC client：封裝在 `internal/engineclient` 或類似 package 中

### 5.2 典型請求流程

#### 5.2.1 使用者在 Dashboard 啟動策略

1. 前端呼叫 `POST /api/strategies/:id/start`（control-api）
2. control-api 檢查 Auth、權限與參數
3. control-api 透過 gRPC 呼叫 trading-engine 的 `StartStrategy(id, userContext)`
4. trading-engine 更新策略狀態、必要時寫 DB，並透過 event bus 發佈狀態變更
5. control-api 回應前端成功/失敗

#### 5.2.2 Tick → 策略 → 下單（關鍵路徑）

1. trading-engine 內部的 WebSocket feed 收到 tick
2. 更新指標 → 觸發策略邏輯 → 產生 signal
3. 風險檢查 → 下單（實際 hitting exchange 或 DryRun）
4. 更新持倉 / 寫 DB / 發佈事件

> 此流程完全在 trading-engine 內部完成，不需要經過 control-api 或其他服務，確保延遲最小。

### 5.3 部署與運維考量

- control-api 可以獨立水平擴充，以承受更多前端請求（例如報表查詢、管理操作）
- trading-engine 可採較少但較穩定的節點，並依照 exchange/連線數量來擴展
- 兩者間 gRPC 通訊可部署於同一 VPC / 內網，避免暴露 engine 端點到公網

---

## 6. Phase 3：進一步拆分與演進（視實際需求）

在 Phase 2 穩定運作、並且系統負載與團隊規模進一步成長時，可考慮以下方向：

### 6.1 Analytics / Backtest Service

- 從 DB 或事件流訂閱資料，專門處理：
  - 歷史 PnL 分析、風險報表
  - 回測（backtest）與參數優化
  - 多策略組合績效分析
- 優點：
  - 將重度計算與查詢從 trading-engine 中分離，避免影響實時交易
  - 可以選擇更適合分析的資料庫（例如 ClickHouse / TimescaleDB）

### 6.2 Auth / User / Billing Service

- 將認證、使用者與計費邏輯抽離：
  - 單一登入/權限服務
  - 不同使用者與 plan 對應的交易限制、風險參數預設值
- trading-engine 與 control-api 僅依賴該服務提供的 token/claims，不直接操作 user table

### 6.3 Event-driven 整合

- 階段性將 in-memory event bus 替換或補充為：
  - Kafka / NATS 作為跨服務事件匯流排
  - 讓 Analytics/Monitoring/Alerting 等服務以訂閱方式取得事件
- trading-engine 仍可保留本地 event bus 以滿足低延遲需求，外部則透過 async event stream 消費。

---

## 7. 資料庫與配置演進

### 7.1 DB 演進路線（建議）

1. **短期**：持續使用 SQLite，優先把 schema 與存取層 (`pkg/db`) 穩定下來
2. **中期**：引入 PostgreSQL 作為主要 OLTP DB，並保留 SQLite 作為開發/本地模式或 DryRun DB
3. **長期**：視分析需求引入時序/分析型 DB（TimescaleDB / ClickHouse 等）

### 7.2 資料存取策略

- trading-engine 為主要寫入者
- control-api / analytics 主要以 read-only 方式存取交易資料（透過 DB 或 engine 提供的 API）
- 重要資料表（orders、trades、positions）必須有清楚的 owner 模組與寫入政策，以避免多服務寫入衝突

---

## 8. 實作建議與 TODO 概覽

### Phase 1（短期即可啟動）

- [ ] 在程式碼層面明確切出 `engine` vs `control` 的 package 邊界
- [ ] 定義並實作 `EngineService` 介面，`internal/api` 只透過此介面操作交易核心
- [ ] 梳理 DB 存取路徑：限定寫入只能由 engine 完成
- [ ] 補齊與新邊界對應的單元測試與整合測試

### Phase 2（服務拆分）

- [ ] 設計並落實 gRPC 協定（`proto/engine.proto`）
- [ ] 建立 `trading-engine` 與 `control-api` 兩個獨立可部署的 binary
- [ ] 更新前端設定，使其只連線至 `control-api`
- [ ] 建立基本的 observability（latency、QPS、goroutine、DB latency）以評估拆分效果

### Phase 3（視需要啟動）

- [ ] 規劃並實作 Analytics/Backtest service 的資料來源與 API
- [ ] 抽離 Auth/User/Billing 至獨立服務（如有 SaaS 需求）
- [ ] 評估並逐步導入事件型匯流排（Kafka/NATS），讓更多周邊服務透過訂閱事件整合

---

## 9. 總結

這份路線圖的關鍵精神是：

- **先穩定交易核心，再逐步拆分**：  
  不犧牲關鍵路徑的延遲與穩定性，避免「為拆而拆」。

- **從邏輯邊界開始，而非直接動基礎設施**：  
  先用乾淨的 interface 與 package 邊界讓 monolith 容易維護，之後要拆服務時才有明確的契約可以依循。

- **配合性能分析與優化計畫同步推進**：  
  `PERFORMANCE_ANALYSIS.md` 與 `PERFORMANCE_IMPROVEMENT_PLAN_V1.md` 提供了性能面向的觀測與優化方向，本文件則補上「服務邊界與長期拆分」的架構視角，兩者應一起使用。

未來如有新的業務需求（多交易所、多租戶、SaaS 化等），可以在本文件基礎上，擴充更多服務類型與部署拓撲，但核心原則保持不變：**交易核心優先，觀測與契約先行，漸進式演進。**

