# 策略使用完整指南

> 從創建到管理策略的詳細操作手冊

---

## 📋 目錄

1. [策略基礎概念](#1-策略基礎概念)
2. [策略生命週期](#2-策略生命週期)
3. [策略配置詳解](#3-策略配置詳解)
4. [策略類型說明](#4-策略類型說明)
5. [進階功能](#5-進階功能)
6. [績效監控](#6-績效監控)
7. [故障排除](#7-故障排除)

---

## 1. 策略基礎概念

### 什麼是策略實例？

```
策略實例 = 交易對 + K線週期 + 策略邏輯 + 參數
```

**範例**:
- BTC 1小時 MA均線策略 (fast=10, slow=30)
- ETH 15分鐘 RSI策略 (period=14, overbought=70)

### 策略資料結構

| 欄位 | 說明 | 範例 |
|------|------|------|
| `id` | 唯一識別碼 | `strat_btc_ma_001` |
| `name` | 顯示名稱 | `BTC 1H MA Cross` |
| `strategy_type` | 策略邏輯類型 | `ma_cross` |
| `symbol` | 交易對 | `BTCUSDT` |
| `interval` | K線週期 | `1h` |
| `parameters` | 策略參數 (JSON) | `{"fast":10,"slow":30}` |
| `status` | 狀態 | `ACTIVE`/`PAUSED`/`STOPPED` |

---

## 2. 策略生命週期

### 狀態流轉圖

```
                    ┌─────────────┐
                    │   CREATED   │
                    └──────┬──────┘
                           │ start
                           ▼
    ┌──────────────────────────────────────────┐
    │                  ACTIVE                   │
    │         (策略運行中，產生交易)              │
    └────────┬──────────────┬──────────────────┘
             │              │
        pause│              │stop/panic
             ▼              ▼
    ┌────────────────┐ ┌────────────────┐
    │     PAUSED     │ │    STOPPED     │
    │  (暫停，保留倉位) │ │  (停止，可平倉)  │
    └────────┬───────┘ └────────────────┘
             │ start           ▲
             └─────────────────┘
```

### 操作說明

| 操作 | API | 效果 |
|------|-----|------|
| **啟動** | `POST /strategies/:id/start` | 開始監聽行情，產生交易信號 |
| **暫停** | `POST /strategies/:id/pause` | 停止信號，保留持倉 |
| **停止** | `POST /strategies/:id/stop` | 完全停止，保留持倉 |
| **緊急平倉** | `POST /strategies/:id/panic` | 市價平掉所有倉位並停止 |

### 範例操作

```bash
# 啟動策略
curl -X POST http://localhost:8080/api/strategies/strat_001/start \
  -H "Authorization: Bearer $TOKEN"

# 返回
{"status": "started", "strategy_id": "strat_001"}
```

---

## 3. 策略配置詳解

### 3.1 基礎配置

```sql
-- 查看策略配置
SELECT id, name, strategy_type, symbol, interval, parameters, status 
FROM strategy_instances;
```

### 3.2 修改參數

**API 方式**:
```bash
PUT /api/strategies/:id/params
Content-Type: application/json

{
  "fast_period": 10,
  "slow_period": 30,
  "size": 0.01,
  "stop_loss": 0.02,
  "take_profit": 0.05
}
```

**效果**: 
- 參數立即生效
- 不會影響現有倉位
- 下次信號將使用新參數

### 3.3 綁定交易所

每個策略需要綁定一個交易所連線才能下單：

```bash
# 1. 先創建連線
POST /api/connections
{
  "name": "主帳號",
  "exchange_type": "binance_futures_usdt",
  "api_key": "xxx",
  "api_secret": "xxx"
}
# 返回: {"id": "conn_001"}

# 2. 綁定到策略
PUT /api/strategies/strat_001/binding
{
  "connection_id": "conn_001"
}
```

### 3.4 K線週期選項

| 週期 | 說明 | 適用場景 |
|------|------|----------|
| `1m` | 1分鐘 | 高頻、剝頭皮 |
| `5m` | 5分鐘 | 短線 |
| `15m` | 15分鐘 | 日內交易 |
| `1h` | 1小時 | 波段 |
| `4h` | 4小時 | 中長線 |
| `1d` | 1天 | 長線 |

---

## 4. 策略類型說明

### 4.1 內建策略

| 類型 | 說明 | 關鍵參數 |
|------|------|----------|
| `ma_cross` | 均線交叉 | `fast_period`, `slow_period`, `size` |
| `rsi` | RSI 超買超賣 | `period`, `overbought`, `oversold`, `size` |
| `python_worker` | Python 自訂策略 | `script_path`, `size` |

### 4.2 MA Cross 均線交叉

**邏輯**:
- 快線上穿慢線 → 做多
- 快線下穿慢線 → 做空/平倉

**參數**:
```json
{
  "fast_period": 10,     // 快線週期
  "slow_period": 30,     // 慢線週期  
  "size": 0.01,          // 下單數量
  "stop_loss": 0.02,     // 止損百分比
  "take_profit": 0.05    // 止盈百分比
}
```

### 4.3 RSI 策略

**邏輯**:
- RSI < oversold → 做多
- RSI > overbought → 做空/平倉

**參數**:
```json
{
  "period": 14,
  "overbought": 70,
  "oversold": 30,
  "size": 0.01
}
```

### 4.4 Python Worker

使用 Python 編寫自訂策略：

```python
# python/strategies/my_strategy.py
def on_tick(kline, position, balance):
    if should_buy(kline):
        return {"action": "BUY", "size": 0.01}
    elif should_sell(kline):
        return {"action": "SELL", "size": 0.01}
    return None
```

---

## 5. 進階功能

### 5.1 利潤目標停止 ⭐

達到利潤目標時自動停止策略：

```sql
-- 設置: 累計盈利 500 USDT 時停止
UPDATE strategy_instances 
SET profit_target = 500, 
    profit_target_type = 'USDT' 
WHERE id = 'strat_001';

-- 設置: 累計盈利 10% 時停止
UPDATE strategy_instances 
SET profit_target = 10, 
    profit_target_type = 'PERCENT' 
WHERE id = 'strat_001';

-- 關閉利潤目標
UPDATE strategy_instances 
SET profit_target = 0 
WHERE id = 'strat_001';
```

**運作方式**:
1. 每次成交後檢查累計 PnL
2. 達標時自動將策略設為 STOPPED
3. 發送 `PROFIT_TARGET_REACHED` 事件

### 5.2 Maker Only 模式 ⭐

只使用限價掛單，降低手續費：

```sql
-- 設置 Maker Only
UPDATE strategy_instances 
SET time_in_force = 'GTX' 
WHERE id = 'strat_001';

-- 恢復默認
UPDATE strategy_instances 
SET time_in_force = 'GTC' 
WHERE id = 'strat_001';
```

**TimeInForce 選項**:
| 值 | 說明 |
|----|------|
| `GTC` | 默認，直到成交或取消 |
| `IOC` | 立即成交或取消 |
| `FOK` | 全部成交或取消 |
| `GTX` | Post Only (Maker Only) |

### 5.3 風控參數

策略級別的風控設置：

```json
{
  "stop_loss": 0.02,       // 止損 2%
  "take_profit": 0.05,     // 止盈 5%  
  "max_position": 1.0,     // 最大持倉
  "use_trailing_stop": true,
  "trailing_percent": 0.01
}
```

---

## 6. 績效監控

### 6.1 查看策略績效

```bash
GET /api/strategies/:id/performance
```

**返回**:
```json
{
  "strategy_id": "strat_001",
  "realized_pnl": 125.50,
  "unrealized_pnl": -15.20,
  "total_trades": 42,
  "win_rate": 0.62,
  "equity_curve": [
    {"date": "2025-12-01", "value": 10000},
    {"date": "2025-12-02", "value": 10125},
    ...
  ]
}
```

### 6.2 查看策略倉位

```sql
SELECT * FROM strategy_positions 
WHERE strategy_instance_id = 'strat_001';
```

| 欄位 | 說明 |
|------|------|
| `qty` | 持倉數量 (正=多，負=空) |
| `avg_price` | 平均價格 |
| `realized_pnl` | 已實現盈虧 |

### 6.3 即時狀態 WebSocket

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');
ws.onmessage = (e) => {
  const data = JSON.parse(e.data);
  if (data.type === 'strategy.signal') {
    console.log('策略信號:', data);
  }
};
```

---

## 7. 故障排除

### Q: 策略已啟動但沒有交易？

**檢查清單**:
1. ✅ 策略狀態是 ACTIVE？
2. ✅ 已綁定交易所連線？
3. ✅ 連線 API Key 有效？
4. ✅ 行情有觸發信號條件？

```sql
-- 檢查狀態
SELECT id, status, connection_id FROM strategy_instances 
WHERE id = 'strat_001';
```

### Q: 如何重置策略狀態？

```bash
# 1. 停止策略
POST /api/strategies/:id/stop

# 2. 清除狀態
DELETE FROM strategy_states WHERE strategy_instance_id = 'strat_001';
DELETE FROM strategy_positions WHERE strategy_instance_id = 'strat_001';

# 3. 重新啟動
POST /api/strategies/:id/start
```

### Q: 緊急平倉後倉位還在？

使用 `panic` 會發送市價平倉單，但需要確認：
1. 訂單是否成交 (查看 orders 表)
2. User Data Stream 是否收到回報

```sql
SELECT * FROM orders WHERE strategy_instance_id = 'strat_001' 
ORDER BY created_at DESC LIMIT 5;
```

### Q: 如何查看策略日誌？

系統日誌會顯示策略相關信息：
```
[INFO] strategy strat_001: signal BUY @ 45000
[INFO] executor: order BTCUSDT BUY 0.01 submitted
[INFO] executor: order filled, id=xxx
```

---

## 📊 快速參考卡

```
┌────────────────────────────────────────────────────┐
│                  策略操作速查                        │
├────────────────────────────────────────────────────┤
│ 啟動:  POST /strategies/:id/start                  │
│ 暫停:  POST /strategies/:id/pause                  │
│ 停止:  POST /strategies/:id/stop                   │
│ 平倉:  POST /strategies/:id/panic                  │
│ 參數:  PUT  /strategies/:id/params                 │
│ 綁定:  PUT  /strategies/:id/binding                │
│ 績效:  GET  /strategies/:id/performance            │
├────────────────────────────────────────────────────┤
│               SQL 快捷指令                          │
├────────────────────────────────────────────────────┤
│ 啟用利潤目標:                                       │
│   UPDATE strategy_instances                        │
│   SET profit_target=500, profit_target_type='USDT' │
│   WHERE id='xxx';                                  │
│                                                    │
│ 設置 Maker Only:                                   │
│   UPDATE strategy_instances                        │
│   SET time_in_force='GTX' WHERE id='xxx';          │
└────────────────────────────────────────────────────┘
```
