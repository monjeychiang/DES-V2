# DES-V2 系統架構改進提案

**文件版本**: 1.0
**日期**: 2025-11-28
**目標**: 將 DES-V2 從一個硬編碼的交易引擎，升級為一個**數據庫驅動、配置靈活、狀態持久、健壯可靠的動態策略平台**。

---

## 1. 核心思想：策略即實例 (Strategy as an Instance)

目前的架構中，「策略」是程式碼 (`.go` 檔案)。我們需要轉變思想，將「策略」定義為一個可運行的**「策略實例」**。

**策略實例 = 交易對 + K線週期 + 策略類型 + 一組具體的參數**

此改進將允許用戶在不動一行程式碼的情況下，創建、配置、啟動和停止任意數量的交易策略實例。

---

## 2. 實施路線圖

我們將分三階段進行改造：

*   **第一階段：核心重構 - 參數化與動態加載**
*   **第二階段：健壯性強化 - 狀態與生命週期管理**
*   **第三階段：用戶接口 - API 層**

---

### **第一階段：核心重構 - 參數化與動態加載**

這是本次升級的基石。目標是將所有策略的配置從程式碼中分離出來，存入資料庫。

#### **步驟 1.1：資料庫結構設計**

在現有的 SQLite 資料庫中，新增一張核心配置表 `strategy_instances`。

**`strategy_instances` 表結構:**

| 欄位名 | 類型 | 說明 | 範例 |
| :--- | :--- | :--- | :--- |
| `id` | INTEGER | 主鍵，自增長 | `1` |
| `name` | TEXT | 用戶自定義的實例名稱 | `"BTC_1h_ShortTerm_MA"` |
| `strategy_type` | TEXT | 對應的策略文件名（無後綴） | `"ma_cross"` |
| `symbol` | TEXT | 交易對 | `"BTCUSDT"` |
| `kline_interval`| TEXT | K線週期 | `"1h"` |
| `parameters` | TEXT | 策略參數 (JSON格式) | `{"fast": 10, "slow": 30, "size": 0.01}` |
| `is_active` | BOOLEAN | 是否啟用 | `true` |
| `created_at` | DATETIME | 創建時間 | `...` |

#### **步驟 1.2：改造策略引擎 (`internal/strategy/engine.go`)**

引擎的啟動邏輯需要完全重寫。

**舊邏輯 (Hard-coded):**
```go
// engine.go - 舊的啟動方式 (示意)
func (e *Engine) Start() {
    strategy1 := NewMACrossStrategy("BTCUSDT", 10, 30, 0.01)
    e.AddStrategy(strategy1)
    
    strategy2 := NewRSIStrategy("ETHUSDT", 14, 30, 70, 0.1)
    e.AddStrategy(strategy2)
    // ...
}
```

**新邏輯 (Database-driven):**
```go
// engine.go - 新的啟動方式 (示意)
import "encoding/json"

func (e *Engine) Start(db *sql.DB) {
    // 1. 從資料庫讀取所有啟用的策略實例
    rows, _ := db.Query("SELECT * FROM strategy_instances WHERE is_active = true")
    
    for rows.Next() {
        var instance dbStrategyInstance // 對應表結構的 struct
        // ...掃描數據到 instance...

        // 2. 策略工廠：根據 strategy_type 創建對應的策略物件
        var strategy Strategy
        switch instance.strategy_type {
        case "ma_cross":
            var params struct { Fast, Slow int; Size float64 }
            json.Unmarshal([]byte(instance.parameters), &params)
            strategy = NewMACrossStrategy(instance.symbol, params.Fast, params.Slow, params.Size)
        case "rsi":
            // ...類似的邏輯...
        // ...更多 case...
        }
        
        // 3. 將創建好的策略實例添加到引擎中
        if strategy != nil {
            e.AddStrategy(instance.id, strategy) // 注意：需要用實例ID作為唯一標識
        }
    }
}
```

---

### **第二階段：健壯性強化 - 狀態與生命週期管理**

在第一階段的基礎上，我們解決那些讓系統變得脆弱的細節問題。

#### **步驟 2.1：策略狀態持久化**

*   **目標**: 讓策略在重啟後能恢復之前的內部狀態。
*   **實施**:
    1.  新增 `strategy_states` 表 (`strategy_instance_id`, `state_data TEXT`)。
    2.  在 `Strategy` 接口中新增兩個方法：`GetState() map[string]interface{}` 和 `SetState(state map[string]interface{})`。
    3.  每個具體的策略（如 `MACrossStrategy`）都需要實現這兩個方法，用來序列化和反序列化其內部變數（如 `prevSignal`）。
    4.  策略引擎在**加載**策略前，先從 `strategy_states` 讀取狀態並調用 `SetState`。在系統**優雅退出**時，遍歷所有策略，調用 `GetState` 並將狀態寫回資料庫。

#### **步驟 2.2：歷史數據填充**

*   **目標**: 確保策略啟動時有足夠的數據來「預熱」指標。
*   **實施**:
    1.  創建一個 `HistoricalDataService` 模組。
    2.  該模組提供一個方法 `GetKlines(symbol, interval string, limit int) []Kline`，內部實現對幣安 REST API 的呼叫。
    3.  策略引擎在 `AddStrategy` 後，會檢查策略需要多少根 K 線（例如 MA200 需要 200 根），然後調用 `HistoricalDataService` 獲取數據。
    4.  將獲取到的歷史 K 線數據按順序、靜默地（不產生交易信號）餵給策略的 `OnTick` 方法，完成預熱。

#### **步驟 2.3：策略與倉位綁定**

*   **目標**: 精確追蹤每個策略實例的倉位和表現。
*   **實施**:
    1.  在 `positions` 表和 `orders` 表中新增 `strategy_instance_id` 欄位。
    2.  修改訂單執行器 (`order/executor.go`)，在下單時將 `strategy_instance_id` 寫入訂單記錄。
    3.  修改狀態管理器 (`state/manager.go`)，在更新倉位時，確保倉位與策略實例關聯。
    4.  策略在產生信號前，可以先向狀態管理器查詢「屬於自己」的倉位，以決定是開倉還是平倉。

---

### **第三階段：用戶接口 - API 層**

*   **目標**: 讓用戶可以透過外部接口管理策略實例。
*   **實施**:
    *   開發一組 RESTful API，對 `strategy_instances` 表進行 CRUD (創建、讀取、更新、刪除) 操作。
    *   **`GET /api/strategies`**: 列出所有策略實例。
    *   **`POST /api/strategies`**: 創建一個新的策略實例（請求體包含 symbol, type, params 等）。
    *   **`PUT /api/strategies/{id}`**: 更新一個策略實例（例如，修改參數或切換 `is_active` 狀態）。
    *   **`DELETE /api/strategies/{id}`**: 刪除一個策略實例。

---

## 3. 預期成果

完成以上改造後，DES-V2 將具備以下核心優勢：

*   **極高的靈活性**: 用戶可以任意組合、測試和運行無數個策略，無需開發人員介入。
*   **健壯性**: 狀態持久化和歷史數據填充將大大減少因重啟或啟動延遲帶來的異常交易。
*   **精確的 PnL 分析**: 按策略實例追蹤倉位和訂單，為後續的性能分析和回測打下基礎。
*   **可擴展性**: 新增一種策略類型，只需在策略工廠中增加一個 `case` 即可，對現有系統無影響。
