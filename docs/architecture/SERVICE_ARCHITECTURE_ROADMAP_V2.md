# DES Trading System V2.0 - 服務架構演進路線 V2

> **版本**: 2.0  
> **日期**: 2025-12-08  
> **Phase 1 狀態**: ✅ 已完成  
> **相關文件**:  
> - 系統總體架構: `docs/architecture/SYSTEM_ARCHITECTURE.md`  
> - 性能分析: `docs/architecture/PERFORMANCE_ANALYSIS.md`  
> - 性能優化計畫: `docs/roadmap/PERFORMANCE_IMPROVEMENT_PLAN_V2.md`

---

## 📊 執行摘要

| Phase | 狀態 | 完成日期 |
|-------|------|----------|
| Phase 1: 邏輯邊界重構 | ✅ **完成** | 2025-12-08 |
| Phase 2: 服務拆分 | 📋 規劃中 | - |
| Phase 3: 進階演進 | 📋 規劃中 | - |

---

## 1. Phase 1 完成報告

### 1.1 目標達成

- ✅ 定義 `engine.Service` 介面
- ✅ 實作 `engine.Impl` 組合現有模組
- ✅ 重構 API 層使用介面
- ✅ 移除 Legacy 依賴欄位
- ✅ 編譯驗證通過

### 1.2 架構變化

```
Before (直接依賴):
┌─────────────┐
│  api.Server │
├─────────────┤
│ *strategy.  │──▶ strategy.Engine
│   Engine    │
│ *risk.      │──▶ risk.Manager
│   Manager   │
│ *balance.   │──▶ balance.Manager
│   Manager   │
└─────────────┘

After (介面隔離):
┌─────────────┐
│  api.Server │
├─────────────┤
│ engine.     │──▶ engine.Service (介面)
│   Service   │           │
└─────────────┘           ▼
                  ┌─────────────┐
                  │ engine.Impl │
                  ├─────────────┤
                  │ 組合所有    │
                  │ 內部模組    │
                  └─────────────┘
```

### 1.3 新增檔案

| 檔案路徑 | 說明 |
|----------|------|
| `internal/engine/service.go` | Service 介面定義 |
| `internal/engine/types.go` | DTO 類型 |
| `internal/engine/impl.go` | 介面實作 |

### 1.4 Engine Service 介面

```go
type Service interface {
    // 策略指令
    StartStrategy(ctx, id) error
    PauseStrategy(ctx, id) error
    StopStrategy(ctx, id) error
    PanicSellStrategy(ctx, id, userID) error
    UpdateStrategyParams(ctx, id, params) error
    BindStrategyConnection(ctx, strategyID, userID, connectionID) error

    // 策略查詢
    ListStrategies(ctx, userID) ([]StrategyInfo, error)
    GetStrategyStatus(ctx, id) (*StrategyStatus, error)
    GetStrategyPosition(ctx, id) (float64, error)

    // 持倉與訂單
    GetPositions(ctx) ([]Position, error)
    GetOpenOrders(ctx) ([]Order, error)

    // 風險與績效
    GetRiskMetrics(ctx) (*RiskMetrics, error)
    GetStrategyPerformance(ctx, id, from, to) (*Performance, error)

    // 餘額
    GetBalance(ctx) (*BalanceInfo, error)

    // 系統
    GetSystemStatus(ctx) *SystemStatus
}
```

---

## 2. Phase 2: 服務拆分 (規劃中)

### 2.1 目標

將單一 `trading-core` 拆分為：
- **trading-engine**: 核心交易邏輯
- **control-api**: REST API 層

### 2.2 先決條件

| 條件 | 狀態 |
|------|------|
| Phase 1 完成 | ✅ |
| gRPC proto 設計 | 📋 待開始 |
| 明確擴縮需求 | ⏸️ 評估中 |
| 團隊規模 >= 3 | ⏸️ 評估中 |

### 2.3 預計工作項目

```
Phase 2 TODO:
├── [ ] 設計 proto/engine.proto
├── [ ] 建立 trading-engine binary
├── [ ] 建立 control-api binary
├── [ ] gRPC client 封裝
├── [ ] 部署配置更新
└── [ ] 前端 URL 切換
```

### 2.4 gRPC Proto 設計 (草案)

```protobuf
service TradingEngine {
    // Strategy Commands
    rpc StartStrategy(StrategyRequest) returns (StatusResponse);
    rpc PauseStrategy(StrategyRequest) returns (StatusResponse);
    rpc StopStrategy(StrategyRequest) returns (StatusResponse);
    rpc PanicSellStrategy(PanicRequest) returns (StatusResponse);
    
    // Queries
    rpc GetPositions(Empty) returns (PositionsResponse);
    rpc GetRiskMetrics(Empty) returns (RiskMetricsResponse);
    rpc GetBalance(Empty) returns (BalanceResponse);
}
```

---

## 3. Phase 3: 進階演進 (長期規劃)

### 3.1 可能方向

| 服務 | 說明 | 觸發條件 |
|------|------|----------|
| Analytics Service | 回測與分析 | 需要獨立計算資源 |
| Auth Service | 認證與計費 | SaaS 化需求 |
| Event Bus (Kafka/NATS) | 跨服務事件 | 分散式部署需求 |

### 3.2 資料庫演進

| 階段 | 資料庫 | 狀態 |
|------|--------|------|
| 短期 | SQLite | ✅ 使用中 |
| 中期 | PostgreSQL | 📋 規劃中 |
| 長期 | TimescaleDB/ClickHouse | 📋 評估中 |

---

## 4. 設計原則 (維持不變)

1. **關鍵路徑優先** - Tick → Strategy → Risk → Order → Exchange 保持最少跳數
2. **先有邏輯邊界，再談物理拆分** - ✅ Phase 1 已達成
3. **協定優先 (contract-first)** - 介面穩定再拆分
4. **觀測與度量先行** - 有數據支持的決策
5. **簡單勝於複雜** - 避免過早引入基礎設施

---

## 5. 後續行動建議

### 短期 (1-2 週)
- [ ] 完善 Engine 介面單元測試
- [ ] 遷移 Metrics/OrderQueue 到 Engine (可選)
- [ ] 評估 Phase 2 必要性

### 中期 (1-2 月)
- [ ] 設計 gRPC proto (如需拆分)
- [ ] PostgreSQL 遷移準備

### 長期
- [ ] Phase 2/3 依業務需求啟動

---

## 變更日誌

| 版本 | 日期 | 變更 |
|------|------|------|
| V1 | 2025-12-08 | 初版規劃 |
| **V2** | **2025-12-08** | **Phase 1 完成，更新狀態** |

---

*本文件將隨著架構演進持續更新。*
