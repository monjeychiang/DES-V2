# DES v2.0 快速參考

## 🚀 一分鐘啟動

```bash
# 後端
cd backend/cmd/trading-core
cp .env.example .env    # 編輯填入 API 金鑰
go run main.go          # http://localhost:8080

# 前端
cd frontend
npm install
npm run dev             # http://localhost:5173
```

## 📂 關鍵檔案

| 檔案 | 用途 |
|------|------|
| `.env` | 環境變數 (API 金鑰、系統參數) |
| `strategies.yaml` | 策略配置 |
| `main.go` | 程式入口 |
| `test_btc.db` | SQLite 數據庫 |

## 🎯 常用指令

### 開發
```bash
go run main.go              # 啟動後端
go test ./...               # 執行測試
go build                    # 編譯
```

### 測試
```bash
.\scripts\test\test-api.ps1          # API 測試
.\scripts\test\test-middleware.ps1   # 中間件測試
```

### 清理
```bash
rm test_btc.db             # 重置數據庫
go mod tidy                # 清理依賴
```

## 🔧 關鍵環境變數

```env
# 最重要的 3 個
DRY_RUN=true               # 模擬模式（必須先設 true）
BINANCE_API_KEY=xxx        # Binance 金鑰
BINANCE_API_SECRET=xxx     # Binance 密鑰

# 風險控制
MAX_POSITION_SIZE=0.1      # 最大倉位 10%
DAILY_LOSS_LIMIT=-500      # 每日虧損限制
STOP_LOSS_PERCENT=0.02     # 止損 2%
```

## 📡 API 端點

| 端點 | 方法 | 說明 |
|------|------|------|
| `/api/strategies` | GET | 策略列表 |
| `/api/strategies/:id/start` | POST | 啟動策略 |
| `/api/strategies/:id/pause` | POST | 暫停策略 |
| `/api/strategies/:id/stop` | POST | 停止策略 |
| `/api/strategies/:id/panic` | POST | 恐慌平倉 |
| `/api/strategies/:id/params` | PUT | 更新參數 |
| `/api/orders` | GET | 訂單列表 |
| `/api/positions` | GET | 持倉列表 |
| `/api/balance` | GET | 餘額查詢 |

## 🏗️ 模組速查

```
internal/
├── api/          → HTTP API + Middleware
├── balance/      → 餘額管理
├── events/       → 事件總線
├── market/       → 市場數據訂閱
├── order/        → 訂單執行
├── risk/         → 風險管理
├── state/        → 持倉狀態
└── strategy/     → 策略引擎
    ├── ma_cross.go
    ├── rsi.go
    └── bollinger.go
```

## 🔍 除錯技巧

### 查看實時日誌
系統日誌已啟用微秒精度：
```
2025/12/01 16:00:00.123456 [API] GET /api/strategies | 200 | 2.5ms
```

### 常見問題排查
```bash
# 策略不執行？
1. 檢查 strategies.yaml 中 is_active: true
2. 查看日誌是否有 WebSocket 連接錯誤
3. 確認有收到 price tick 事件

# 前端無法連接？
1. 後端應該在 :8080
2. 檢查 CORS 是否啟用
3. 查看瀏覽器 Console

# 訂單沒下？
1. DRY_RUN=true 時不會真正下單
2. 檢查餘額是否足夠
3. 查看風險管理是否拒絕
```

## 🚨 緊急停止

```bash
# 停止所有策略
curl -X POST http://localhost:8080/api/strategies/{id}/stop

# 恐慌平倉
curl -X POST http://localhost:8080/api/strategies/{id}/panic
```

## 📚 延伸閱讀

- 完整文檔: `docs/process/DEVELOPER_ONBOARDING.md`
- 架構設計: `docs/design/ADVANCED_FEATURES_DESIGN.md`
- 策略提案: `docs/design/STRATEGY_FEATURES_PROPOSAL.md`
