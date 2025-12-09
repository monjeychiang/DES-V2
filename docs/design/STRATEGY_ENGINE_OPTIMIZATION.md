# Strategy Engine 優化計畫

> 文檔版本: 1.1  
> 建立日期: 2025-12-08  
> 更新日期: 2025-12-09  
> 狀態: **暫緩** (當前架構已足夠)

## 0. 必要性分析 (重要)

### 結論: 優化暫緩

> ⚠️ **當前架構對於一般量化交易需求已經足夠，建議先專注策略開發和風控完善。**

### 使用場景評估

| 場景 | Tick 頻率 | 策略數量 | 當前架構 |
|------|-----------|----------|----------|
| 個人交易 (1-5 策略) | ~1-10/s | 1-5 | ✅ 完全足夠 |
| 小型量化 (10-20 策略) | ~10-50/s | 10-20 | ✅ 足夠 |
| 中型量化 (50+ 策略) | ~100/s | 50+ | ⚠️ 可能需要 Phase 1 |
| 高頻交易 | ~1000+/s | 任意 | ❌ 需要 Phase 1-3 |

### 當前架構餘量

```
實際負載: 5 策略 × 10 tick/s = 50 次 OnTick()/秒
Go Worker Pool 能力: ~10,000+ 次/秒
餘量: 200x
```

### 觸發條件 (何時需要優化)

- [ ] CPU 持續 > 80%
- [ ] Tick 處理延遲 p99 > 100ms
- [ ] 策略數量 > 50 個
- [ ] 使用 AggTrade 高頻數據

---

### 1.1 當前架構

```
WebSocket ──► priceStream (chan) ──► Engine.handleTick() ──► Worker Pool ──► OnTick()
                                            │
                                            ▼
                                     Strategy 1..N (並行)
                                            │
                                            ▼
                                     Signal Bus ──► Risk ──► Executor
```

### 1.2 關鍵程式碼路徑

| 組件 | 檔案 | 函數 |
|------|------|------|
| Tick 接收 | `engine.go:189` | `case msg := <-priceStream` |
| Tick 處理 | `engine.go:222` | `handleTick(msg)` |
| 指標計算 | `engine.go:244` | `Indicators.Update(symbol, price)` |
| 策略執行 | `engine.go:274` | `strat.OnTick(symbol, price, indVals)` |
| Signal 發布 | `engine.go:295` | `bus.Publish(EventStrategySignal, sig)` |

### 1.3 已識別瓶頸

| # | 問題 | 根因 | 影響 |
|---|------|------|------|
| B1 | Tick 逐條處理 | 無批次邏輯 | CPU 利用率低, 函數呼叫開銷大 |
| B2 | Channel 反壓不足 | 無溢出處理 | 高頻場景可能丟數據 |
| B3 | 策略耦合 | 共用 handleTick | 一策略慢影響全部 |
| B4 | 指標重複計算 | 每策略獨立算 | 相同 symbol 重複計算 |
| B5 | Python 策略延遲 | gRPC 往返 | 每 tick ~1-5ms 開銷 |

---

## 2. 優化方案

### 2.1 Phase 1: Tick 批次處理 (P0)

**目標**: 減少 tick 處理頻率, 提高 CPU 效率

**改動範圍**: `engine.go`

```go
// 新增配置
type EngineConfig struct {
    BatchInterval time.Duration // 批次間隔 (預設 10ms)
    BatchSize     int           // 最大批次大小 (預設 100)
}

// 改動: Start() 使用批次模式
func (e *Engine) Start(ctx context.Context, stream <-chan any) {
    ticker := time.NewTicker(e.config.BatchInterval)
    latestPrices := make(map[string]float64)
    
    for {
        select {
        case msg := <-stream:
            sym, price := extractTick(msg)
            latestPrices[sym] = price // 只保留最新
            
        case <-ticker.C:
            // 批次處理所有 symbol
            for sym, price := range latestPrices {
                go e.handleTickAsync(sym, price)
            }
            latestPrices = make(map[string]float64)
        }
    }
}
```

**預估收益**: 
- CPU 開銷減少 50-90% (高頻場景)
- 延遲增加 ~10ms (可配置)

**風險**: 低 - 可透過配置開關

---

### 2.2 Phase 2: 策略獨立隔離 (P1)

**目標**: 每策略獨立 goroutine, 完全隔離

**改動範圍**: `engine.go`, 新增 `strategy_runner.go`

```
                    ┌──────────────┐
                    │  priceStream │
                    └──────┬───────┘
                           │ Dispatcher (fan-out)
           ┌───────────────┼───────────────┐
           ▼               ▼               ▼
    ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
    │ Runner A    │ │ Runner B    │ │ Runner C    │
    │ - tickChan  │ │ - tickChan  │ │ - tickChan  │
    │ - strategy  │ │ - strategy  │ │ - strategy  │
    │ - indicators│ │ - indicators│ │ - indicators│
    └─────────────┘ └─────────────┘ └─────────────┘
```

**新增結構**:
```go
type StrategyRunner struct {
    id         string
    strategy   Strategy
    tickChan   chan Tick      // 專屬 channel (ring buffer)
    indicators *IndicatorSet  // 專屬指標實例
    ctx        context.Context
}

func (r *StrategyRunner) Run() {
    for tick := range r.tickChan {
        indVals := r.indicators.Update(tick.Symbol, tick.Price)
        sig, _ := r.strategy.OnTick(tick.Symbol, tick.Price, indVals)
        if sig != nil {
            bus.Publish(EventStrategySignal, sig)
        }
    }
}
```

**預估收益**:
- 策略完全隔離, 一個慢不影響其他
- 可針對單策略調優 (不同 buffer 大小)

**風險**: 中 - 內存增加 (每策略一套指標)

---

### 2.3 Phase 3: 進階優化 (P2)

| 優化項 | 說明 | 複雜度 | 收益 |
|--------|------|--------|------|
| Ring Buffer | 替代 channel, lock-free | 高 | 吞吐 2-5x |
| SIMD 指標 | 批量計算 MA/RSI | 中 | 指標計算 4x |
| 指標共享 | 相同 symbol 共用 | 中 | 內存節省 |
| Python WASM | 替代 gRPC | 高 | 延遲降 10x |

---

## 3. 實施計畫

### 3.1 Phase 1 時程 (1-2 天)

| 步驟 | 任務 | 預估 |
|------|------|------|
| 1.1 | 新增 `EngineConfig` 結構 | 0.5h |
| 1.2 | 實作批次收集邏輯 | 2h |
| 1.3 | 新增配置開關 (env/config) | 1h |
| 1.4 | 單元測試 | 2h |
| 1.5 | 性能測試對比 | 2h |

### 3.2 Phase 2 時程 (3-5 天)

| 步驟 | 任務 | 預估 |
|------|------|------|
| 2.1 | 設計 `StrategyRunner` 介面 | 2h |
| 2.2 | 實作 Dispatcher (fan-out) | 4h |
| 2.3 | 實作 Runner 生命週期 | 4h |
| 2.4 | 遷移現有策略 | 4h |
| 2.5 | 整合測試 | 8h |

---

## 4. 驗證指標

### 4.1 性能基準

| 指標 | 現況 (估計) | Phase 1 目標 | Phase 2 目標 |
|------|-------------|--------------|--------------|
| Tick 處理延遲 (p99) | 5ms | 15ms (含批次) | 10ms |
| 策略執行延遲 (p99) | 10ms | 10ms | 5ms |
| CPU 使用率 (10k tick/s) | 80% | 40% | 30% |
| 內存使用 | 100MB | 100MB | 150MB |

### 4.2 測試方法

```bash
# 延遲測試
go test -bench=BenchmarkTickProcessing -benchmem

# 壓力測試 (模擬高頻)
go run cmd/loadtest/main.go --ticks-per-sec=10000 --duration=60s
```

---

## 5. 風險與回滾

| 風險 | 緩解措施 |
|------|----------|
| 批次延遲不可接受 | 配置開關, 可關閉批次模式 |
| 內存增加過多 | 監控 + 共享指標 (Phase 3) |
| 策略行為變化 | A/B 測試, 驗證 signal 一致性 |

**回滾方案**: 所有優化透過 feature flag 控制, 可即時回滾

---

## 6. 附錄

### 6.1 相關檔案

- `backend/cmd/trading-core/internal/strategy/engine.go`
- `backend/cmd/trading-core/internal/strategy/types.go`
- `backend/cmd/trading-core/internal/indicators/engine.go`
- `backend/cmd/trading-core/main.go` (signal subscriber)

### 6.2 參考資料

- [Go Concurrency Patterns](https://go.dev/blog/pipelines)
- [Lock-free Ring Buffer](https://github.com/Workiva/go-datastructures)
- [SIMD in Go](https://github.com/klauspost/cpuid)
