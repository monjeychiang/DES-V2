# DES Trading System v2.0 開發 Roadmap（建議順序）

> 本文件描述 DES v2.0 從「工程框架」走向「產品化策略交易工具」的建議開發順序。  
> 核心假設：目前後端交易／風控／事件流能力已相對完整，主要差距在「用戶化體驗」與「產品化包裝」。

參考文件：
- `docs/architecture/USER_FLOW_AND_IMPLEMENTATION_COMPARISON.md`
- `docs/architecture/SYSTEM_ARCHITECTURE.md`
- `docs/process/DEVELOPER_ONBOARDING.md`
- `docs/setup/QUICK_REFERENCE.md`

---

## Phase 1：產品化既有能力（核心 Dashboard MVP）

目標：  
把「已存在的後端能力」用最小成本包裝成一個「可以真的拿給使用者操作」的單一頁面 Dashboard，支援：
- 查看策略列表與狀態。
- 啟動／暫停／停止／Panic Sell。
- 檢視持倉、訂單與餘額。
- 清楚標示目前是 Dry Run 還是 Live。

### 1.1 功能範圍

- 策略列表與狀態顯示：
  - 讀取所有可用策略（strategy instances）。
  - 顯示名稱、類型、交易對、時間週期、是否 Active、目前模式（Dry Run / Live）。
- 策略控制：
  - Start / Pause / Stop。
  - Panic Sell（強制平倉）。
- 帳戶／持倉資訊：
  - 全局餘額（Balance）。
  - 各策略持倉（symbol、數量、均價、未實現損益）。
  - 訂單列表（最近 N 筆）。
- 模式標示：
  - 清楚告知目前系統是否在 Dry Run 模式（`DRY_RUN=true`）。
  - 若之後支援 per-strategy 模式，預留 UI 空間（目前可先視為全局設定）。

### 1.2 後端工作項目

- API 確認與補齊（對照 `docs/setup/QUICK_REFERENCE.md`）：
  - 核心已存在：
    - `GET /api/strategies`：列出策略。
    - `POST /api/strategies/:id/start`。
    - `POST /api/strategies/:id/pause`。
    - `POST /api/strategies/:id/stop`。
    - `POST /api/strategies/:id/panic`。
    - `GET /api/orders`。
    - `GET /api/positions`。
    - `GET /api/balance`。
  - 若尚未實作或需要補欄位，統一在 `internal/api` 調整：
    - `GET /api/strategies` 回傳：
      - 策略 id、名稱、類型（MA/RSI/Bollinger...）、symbol、interval。
      - status（ACTIVE / PAUSED / STOPPED）。
      - is_active。
      - 目前已實作參數（例如 size、period）。
    - `GET /api/positions` 回傳：
      - 以 `strategy_instance_id` 分組。
      - 每個策略的持倉列表（symbol、qty、avg_price、unrealized_pnl）。
    - `GET /api/balance` 回傳：
      - 可用餘額、總資產估值。
      - 可閱讀的貨幣單位（USDT / USD）。
- 模式資訊 API：
  - 新增：`GET /api/system/status` 或在現有 API 中加 header：
    - 是否 Dry Run。
    - 目前連線的交易所（Binance Spot / Futures）。
    - 版本號（便於前端顯示）。

### 1.3 前端工作項目

- 新增「Dashboard 首頁」：
  - 版面設計：
    - 左側：策略列表（表格或卡片）。
    - 右上：系統狀態區（DRY RUN / LIVE 標示、連線交易所、當前時間）。
    - 右下：持倉與訂單簡表。
  - 使用 `frontend/src/api.js` 封裝所有 API 呼叫。
- 策略控制 UI：
  - 為每一個策略提供操作按鈕：
    - Start / Pause / Stop / Panic。
  - 加入基本防呆：
    - Stop、Panic 類操作需二次確認（modal）。
  - 操作後提示：
    - 成功／失敗 toast。
- Data Refresh：
  - 先使用 polling（例如每 5–10 秒打一次 `GET /api/strategies`、`/api/positions`）。
  - 之後才考慮 WebSocket push。

### 1.4 驗收標準

- 命令列啟動 backend + frontend，打開瀏覽器：
  - 能看到策略列表與狀態。
  - 能透過按鈕啟動／暫停策略，並在 log 中看到對應事件。
  - 能看到目前持倉與餘額變化（Dry Run 模式亦可）。
- 不需要任何額外設定，即可完成「一人操作多個策略」的最小閉環。

---

## Phase 2：帳號系統與 API 金鑰管理

目標：  
從「單一操作者、全局環境變數」進化成「多使用者、多組交易 API 金鑰」的架構，為之後真正 SaaS 化做準備。

### 2.1 功能範圍

- 基礎帳號系統：
  - 註冊／登入（Email + 密碼即可）。
  - Token-based 認證（JWT 或 session）。
- API 金鑰管理：
  - 每個使用者可以新增多組「交易所連線」：
    - 名稱（自訂別名，如「Binance 主帳戶」、「期貨子帳戶」）。
    - 交易所類型（目前先支援 Binance Spot / Futures）。
    - API Key / Secret（安全存放）。
  - 可以啟用／停用某組金鑰。
- 策略綁定：
  - 每個策略實例（strategy instance）綁定到某一組「交易所連線」。
  - 啟動策略時，必須指定使用哪一組金鑰。

### 2.2 後端工作項目

- 資料模型擴充：
  - 新增 `users` 表：
    - `id`, `email`, `password_hash`, `created_at`, `updated_at`。
  - 新增 `connections`（交易所連線）表：
    - `id`, `user_id`, `exchange_type`, `name`, `api_key_encrypted`, `api_secret_encrypted`, `is_active`。
  - 在 `strategy_instances` 中加入：
    - `user_id`（擁有者）。
    - `connection_id`（綁定哪一組 API 金鑰）。
- 認證與授權：
  - 在 `internal/api` 增加 Auth middleware：
    - 登入／註冊 endpoints。
    - 保護 `/api/strategies`, `/api/orders`, `/api/positions` 等路由。
  - 所有與策略、訂單、持倉相關的查詢，都須以 user_id 為 filter。
- 金鑰安全：
  - 在配置層（`internal/config` 或 `pkg`）中抽象出「金鑰讀取介面」：
    - 從 `connections` 表讀取對應 user 的金鑰。
    - 使用環境變數中的 master key 對 API Secret 做加解密。

### 2.3 前端工作項目

- 新增登入／註冊頁：
  - 簡單的 email + password 表單。
  - 成功後儲存 token，後續 API 帶上 Authorization header。
- 新增「交易所連線管理」頁：
  - 顯示使用者所有連線：
    - 名稱、交易所類型、啟用狀態。
  - 支援新增／編輯／停用。
- 修改策略啟用流程：
  - 在 Dashboard 中，啟動策略前先選擇「連線」：
    - 若策略尚未綁定 connection_id，彈出選擇彈窗。
    - 選定後，後端更新 `strategy_instances.connection_id` 並使用該金鑰下單。

### 2.4 驗收標準

- 可以用兩個不同帳號登入：
  - 各自管理自己的 API 金鑰與策略。
  - 看不到彼此的策略、持倉與訂單。
- 前端 Dashboard 對不同帳號顯示不同資料。
- 策略啟動時使用正確的 API 金鑰下單（以 log 或 Dry Run 模式驗證）。

---

## Phase 3：報表與視覺化績效

目標：  
在不大幅改動引擎的前提下，利用目前 DB 中的交易與持倉資料，提供基本的策略績效視覺化與報表／匯出功能。

### 3.1 功能範圍

- 策略級別的績效圖表：
  - Equity Curve（策略累積淨值曲線）。
  - 每日盈虧柱狀圖。
- 整體帳戶視角：
  - 所有策略合併的總 Equity Curve。
- 匯出功能：
  - 交易紀錄匯出為 CSV（per strategy 或 all strategies）。
  - 可選擇時間區間。

### 3.2 後端工作項目

- 擴充 DB schema（若需要）：
  - 確認是否已有足夠的 trade-level 記錄（成交價、數量、fee、時間）。
  - 如有必要，新增 `trades` 表或補欄位。
- 報表 API：
  - `GET /api/strategies/:id/performance`：
    - 回傳指定時間區間內：
      - 每日損益（date, pnl）。
      - 累積 equity。
  - `GET /api/performance/aggregate`：
    - 聚合所有策略的整體績效。
  - `GET /api/strategies/:id/trades/export`：
    - 以 CSV 串流回傳交易紀錄。
- 計算邏輯：
  - 可先在 DB 層用 SQL group by date 計算每日 pnl。
  - 或在 Go 層將 trade 資料聚合成時間序列（後續可抽成 `pkg/performance`）。

### 3.3 前端工作項目

- Dashboard 新增「績效」分頁：
  - 每個策略卡片提供「查看績效」按鈕：
    - 顯示 Equity Curve 與每日盈虧圖。
  - 一個總覽頁，展示所有策略的合併曲線。
- 匯出 UI：
  - 提供日期選擇與「匯出 CSV」按鈕。
  - 下載檔名中包含策略名稱與日期區間。

### 3.4 驗收標準

- 對於持續跑一段時間的策略：
  - 可以在 UI 上看到合理的盈虧曲線（與 log/DB 數據一致）。
  - 匯出 CSV 後可以用 Excel/Sheets 簡單驗算。

---

## Phase 4：策略「市場化」與多版本管理（A/B 測試基礎）

目標：  
將目前的「策略實例列表」升級成更接近「策略市場／策略庫」的體驗，並為未來的 A/B 測試與版本管理打底。

### 4.1 功能範圍

- 策略「型號」與「實例」分離：
  - Strategy Template（策略模板）：
    - 例如：`MA Crossover`, `RSI Reversion`, `Bollinger Band Breakout`。
  - Strategy Instance（策略實例）：
    - 模板 + 具體參數 + 綁定的交易對／時間週期／API 金鑰。
- 策略市場頁：
  - 顯示可用的策略模板：
    - 名稱、摘要、適用標的／週期、風格（趨勢／均值回歸）、風險等級（簡易標示）。
  - 使用者可以從模板建立新的實例：
    - 填寫 symbol、interval、初始 size、風控參數。
- 多版本管理：
  - 支援從既有實例複製成新實例（copy as…），以調整參數做 A/B。

### 4.2 後端工作項目

- 資料模型：
  - 新增 `strategy_templates` 表：
    - `id`, `name`, `description`, `strategy_type`, `default_parameters`, `tags`。
  - `strategy_instances` 增加：
    - `template_id`（引用模板）。
    - `label`（例如 user 自訂名稱：`ETH MA v1`）。
- API：
  - `GET /api/strategy-templates`：
    - 回傳所有可用模板，支援 tags 篩選。
  - `POST /api/strategy-instances`：
    - 根據指定 template_id 與參數建立新實例。
  - `POST /api/strategy-instances/:id/clone`：
    - 從既有實例複製一份（可覆寫部分參數）。

### 4.3 前端工作項目

- 新增「策略市場」頁：
  - 卡片形式展示模板。
  - 點擊卡片進入詳細介紹頁（顯示策略邏輯摘要、適用場景、預設參數）。
  - 「從此模板建立實例」按鈕，導向建立表單。
- 實例管理：
  - 在 Dashboard 中，策略列表區：
    - 顯示 template name + instance label。
    - 提供「複製成新實例」操作。

### 4.4 驗收標準

- 一個使用者可以：
  - 從模板建立多個不同設定的實例（例如不同 symbol、不同 interval）。
  - 在 Dashboard 中清楚分辨各實例。
  - 用複製功能快速做 A/B 測試（例如不同停損％）。

---

## Phase 5：歷史回測引擎與通知／告警

目標：  
讓使用者在啟用策略之前，可以先跑回測；並在策略運行過程中，對關鍵事件提供主動通知。

### 5.1 歷史回測引擎

**功能範圍**
- 支援針對單一交易所（先定為 Binance Spot 或 Futures）與主要品種執行：
  - 單策略、單實例的歷史回測。
  - 自訂時間區間。
  - 基本交易成本（手續費、滑價假設）。

**後端工作項目**
- 新增「回測執行器」模組（例如 `internal/backtest`）：
  - 可重用現有策略介面（與 live engine 共用 `Strategy` 介面）。
  - 以歷史 K 線／tick 資料模擬 `EventPriceTick` 流入策略。
  - 產生虛擬訂單與 PnL 計算。
- 歷史資料來源：
  - 短期可先從本地 csv／DB 匯入。
  - 之後再設計自動下載／更新機制。
- 回測 API：
  - `POST /api/backtest`：
    - 輸入：strategy template + parameters + symbol + interval + time range + 假設成本。
    - 輸出：回測結果（績效摘要 + Equity 曲線 + 主要統計）。

**前端工作項目**
- 在策略市場或策略詳情頁新增「回測」功能：
  - 用表單輸入參數與時間區間。
  - 顯示回測結果圖表與指標。

### 5.2 通知與告警

**功能範圍**
- 支援以下事件的主動通知：
  - 下單失敗。
  - 觸發每日最大虧損（Daily loss limit）。
  - 策略 Panic Sell 或自動停用。
  - 系統與交易所連線異常。

**後端工作項目**
- 通知模組：
  - 抽象出 `Notifier` 介面（Email / Webhook）。
  - 從 `internal/events` 中訂閱 `EventOrderFilled`, `EventRiskAlert`, `EventError` 等，轉換為通知事件。
- 設定：
  - 在使用者層級增加「通知設定」（Email, Webhook URL）。

**前端工作項目**
- 新增「通知設定」頁：
  - 讓使用者配置通知 Email 或 Webhook。
  - 簡單顯示近期通知 log（可後放）。

---

## Phase 6：進階策略編輯器與 Sandbox（長期）

目標：  
在現有「工程框架」之上，提供給非工程背景用戶也能使用的策略編寫與 Sandbox 執行環境。這一階段工程量大、風險高，建議在前面幾個 Phase 穩定後再投入。

### 6.1 功能範圍

- Web 端策略編輯器：
  - 腳本型（例如簡化版 DSL/Python）或圖形化（if/then blocks）。
  - 支援觸發條件、指標、倉位 sizing、風控規則設定。
- Sandbox 執行環境：
  - 將使用者自訂策略在隔離環境執行。
  - 嚴格資源與權限控制，避免安全問題。

### 6.2 建議做法

- 第一步先支援「配置型策略」：
  - 在既有策略（MA/RSI/Bollinger）上，允許使用者在 UI 中調參，再由後端轉成 `StrategyConfig`。
- 第二步再考慮：
  - 以 Lua / WASM / 限制版 Go plugin 等方式支援自訂邏輯。
  - 這部分需重新審視整體架構與安全模型。

---

## 總結：為什麼是這個順序？

- Phase 1：最小成本把已有能力變成「真的可用」的產品，建立信心與 Demo 能力。
- Phase 2：從一人工具進化為多人系統，是後面任何 SaaS 化的前提。
- Phase 3：報表與視覺化直接提升用戶對策略的信任度與可用性，工程成本相對可控。
- Phase 4：策略市場與版本管理，是讓系統「可玩性」與「探索性」提升的關鍵。
- Phase 5：回測與通知屬於高價值功能，但牽涉面較廣，放在核心穩定後實作。
- Phase 6：策略編輯器與 Sandbox 是長期投資，適合在前面幾階段驗證了市場需求之後再做。

未來可以將本 Roadmap 進一步拆解為 issue／milestone，例如：
- `Phase 1 - Dashboard MVP`
- `Phase 2 - Auth & API Key Management`
- `Phase 3 - Performance & Reporting`
… 並在各 milestone 下建立具體的 backend/frontend/task 清單，以利實際開發追蹤。
