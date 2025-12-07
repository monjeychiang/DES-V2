# 環境變數配置說明

## 📋 完整變數列表與功能

---

### 🌐 服務器配置

#### `PORT=8080`
**功能**: HTTP 服務器監聽端口  
**默認值**: 8080  
**說明**: 
- API 服務器的端口號
- 用於提供 REST API 和 WebSocket 連接
- 修改時確保端口未被占用

**範例**:
```env
PORT=8080      # 默認端口
PORT=3000      # 自定義端口
```

---

### 🔐 Binance 現貨交易配置

#### `BINANCE_TESTNET=false`
**功能**: 是否使用 Binance 測試網  
**默認值**: false  
**說明**:
- `true`: 連接測試網 (testnet.binance.vision)
- `false`: 連接正式環境 (api.binance.com)
- **建議**: 開發時設為 true，生產時設為 false

**範例**:
```env
BINANCE_TESTNET=true   # 測試環境
BINANCE_TESTNET=false  # 生產環境
```

---

#### `BINANCE_API_KEY=`
**功能**: Binance 現貨 API 密鑰  
**默認值**: 空  
**說明**:
- 從 Binance 帳戶設置中生成
- 用於身份驗證
- **安全**: 不要提交到版本控制

**獲取方式**:
```
1. 登錄 Binance
2. 帳戶設置 → API 管理
3. 創建 API Key
4. 記錄 API Key 和 Secret
```

---

#### `BINANCE_API_SECRET=`
**功能**: Binance 現貨 API 密鑰  
**默認值**: 空  
**說明**:
- 配合 API Key 使用
- 用於簽名請求
- **極重要**: 絕對保密

---

#### `BINANCE_SYMBOLS=BTCUSDT,ETHUSDT`
**功能**: 要監控/交易的交易對列表  
**默認值**: BTCUSDT,ETHUSDT  
**格式**: 逗號分隔的交易對列表  

**說明**:
- 系統將監控這些交易對的價格
- 策略會在這些交易對上執行
- 可以添加任意數量的交易對

**範例**:
```env
BINANCE_SYMBOLS=BTCUSDT,ETHUSDT,BNBUSDT,ADAUSDT
BINANCE_SYMBOLS=BTCUSDT  # 只監控一個
```

---

### 📊 數據源配置

#### `USE_MOCK_FEED=true`
**功能**: 是否使用模擬數據源  
**默認值**: true  
**說明**:
- `true`: 使用隨機生成的價格數據（測試用）
- `false`: 使用真實的 Binance 市場數據

**用途**:
```
開發階段: USE_MOCK_FEED=true   # 不消耗API配額
測試階段: USE_MOCK_FEED=false  # 真實數據測試
生產環境: USE_MOCK_FEED=false  # 必須為 false
```

---

### 💰 交易啟用配置

#### `ENABLE_BINANCE_TRADING=false`
**功能**: 是否啟用 Binance 現貨交易  
**默認值**: false  
**說明**:
- `true`: 訂單會真實提交到交易所
- `false`: Dry-run 模式，不會實際下單

**⚠️ 重要**:
```env
# 測試/開發
ENABLE_BINANCE_TRADING=false

# 生產環境（確認無誤後才啟用）
ENABLE_BINANCE_TRADING=true
```

---

### 🚀 Binance USDT 合約配置

#### `ENABLE_BINANCE_USDT_FUTURES=false`
**功能**: 是否啟用 USDT 本位合約交易  
**默認值**: false  
**說明**:
- 啟用後可交易 USDT 永續合約
- 支持槓桿交易

---

#### `BINANCE_USDT_KEY=`
**功能**: USDT 合約 API 密鑰  
**默認值**: 空  
**說明**:
- 獨立於現貨 API
- 需要在 Binance Futures 中單獨生成

---

#### `BINANCE_USDT_SECRET=`
**功能**: USDT 合約 API 密鑰  
**默認值**: 空

---

### 📈 Binance Coin 合約配置

#### `ENABLE_BINANCE_COIN_FUTURES=false`
**功能**: 是否啟用幣本位合約交易  
**默認值**: false  
**說明**:
- 幣本位永續合約（例如 BTC 作為保證金）

---

#### `BINANCE_COIN_KEY=`
**功能**: 幣本位合約 API 密鑰  
**默認值**: 空

---

#### `BINANCE_COIN_SECRET=`
**功能**: 幣本位合約 API 密鑰  
**默認值**: 空

---

### 🐍 Python Worker 配置

#### `ENABLE_PYTHON_WORKER=false`
**功能**: 是否啟用 Python 策略引擎  
**默認值**: false  
**說明**:
- 啟用後可使用 Python 編寫的策略
- Go 作為主引擎，Python 作為策略計算

---

#### `PYTHON_WORKER_ADDR=localhost:50051`
**功能**: Python Worker gRPC 地址  
**默認值**: localhost:50051  
**說明**:
- Python 服務監聽的地址和端口
- 使用 gRPC 通信

**範例**:
```env
PYTHON_WORKER_ADDR=localhost:50051  # 本地
PYTHON_WORKER_ADDR=python-worker:50051  # Docker
```

---

### 💾 數據庫配置

#### `DB_PATH=./trading.db`
**功能**: SQLite 數據庫文件路徑  
**默認值**: ./trading.db  
**說明**:
- 存儲所有交易數據
- 訂單、持倉、風控記錄等

**範例**:
```env
DB_PATH=./trading.db           # 當前目錄
DB_PATH=/var/data/trading.db   # 絕對路徑
```

---

#### `DRY_RUN_DB_PATH=./trading_dry.db`
**功能**: Dry-run 模式專用數據庫路徑  
**默認值**: ./trading_dry.db  
**說明**:
- **重要**: 隔離測試數據和生產數據
- 只在 `DRY_RUN=true` 時使用
- 防止測試數據污染生產數據庫

**使用場景**:
```yaml
# config.yaml
dry_run: true
dry_run_db_path: "test.db"  # 測試數據單獨存儲

# 生產環境
dry_run: false
# 使用 DB_PATH
```

---

### 🔒 安全配置

#### `JWT_SECRET=your-secret-key`
**功能**: JWT 令牌加密密鑰  
**默認值**: your-secret-key  
**說明**:
- 用於生成和驗證 JWT 令牌
- **生產環境必須更改**為隨機強密碼

**生成強密鑰**:
```bash
# Linux/Mac
openssl rand -base64 32

# PowerShell
[Convert]::ToBase64String((1..32 | ForEach-Object { Get-Random -Maximum 256 }))
```

**範例**:
```env
# ❌ 不安全（默認值）
JWT_SECRET=your-secret-key

# ✅ 安全
JWT_SECRET=A7x9mKz2pQ8wE3nR6vT1yU4iO5pL0zA9sD8fG7hJ6kB3
```

---

#### `LICENSE_SERVER=`
**功能**: 授權服務器地址  
**默認值**: 空  
**說明**:
- 用於商業版授權驗證
- 開源版本可忽略
- 如果設置，系統會檢查授權

---

## 🎯 配置範例

### 開發環境配置
```env
# .env.development
PORT=8080
BINANCE_TESTNET=true
BINANCE_API_KEY=your-testnet-key
BINANCE_API_SECRET=your-testnet-secret
BINANCE_SYMBOLS=BTCUSDT,ETHUSDT
USE_MOCK_FEED=true
ENABLE_BINANCE_TRADING=false
DB_PATH=./dev.db
DRY_RUN_DB_PATH=./dev_dry.db
JWT_SECRET=dev-secret-key
```

### 測試環境配置
```env
# .env.test
PORT=8080
BINANCE_TESTNET=true
BINANCE_API_KEY=testnet-key
BINANCE_API_SECRET=testnet-secret
BINANCE_SYMBOLS=BTCUSDT
USE_MOCK_FEED=false
ENABLE_BINANCE_TRADING=true  # 測試網可以真實下單
DB_PATH=./test.db
DRY_RUN_DB_PATH=./test_dry.db
JWT_SECRET=test-secret-key
```

### 生產環境配置
```env
# .env.production
PORT=8080
BINANCE_TESTNET=false
BINANCE_API_KEY=prod-api-key
BINANCE_API_SECRET=prod-api-secret
BINANCE_SYMBOLS=BTCUSDT,ETHUSDT,BNBUSDT
USE_MOCK_FEED=false
ENABLE_BINANCE_TRADING=true
DB_PATH=/var/data/trading.db
DRY_RUN_DB_PATH=/var/data/trading_dry.db
JWT_SECRET=<strong-random-secret>
```

---

## ⚠️ 安全注意事項

### 必須保密的變數
```
BINANCE_API_KEY
BINANCE_API_SECRET
BINANCE_USDT_KEY
BINANCE_USDT_SECRET
BINANCE_COIN_KEY
BINANCE_COIN_SECRET
JWT_SECRET
```

### 安全建議
1. **不要提交 .env 到版本控制**
   ```bash
   # .gitignore
   .env
   .env.local
   .env.production
   ```

2. **使用環境變數管理工具**
   - Docker Secrets
   - Kubernetes Secrets
   - AWS Secrets Manager

3. **定期輪換 API 密鑰**

4. **限制 API 權限**
   - 只啟用必要的權限
   - 不需要提現權限時不要啟用

---

## 📝 配置檢查清單

啟動系統前檢查：

- [ ] PORT 未被占用
- [ ] API Key/Secret 已設置（如果不用模擬）
- [ ] BINANCE_TESTNET 設置正確
- [ ] ENABLE_BINANCE_TRADING 謹慎設置
- [ ] DB_PATH 路徑可寫
- [ ] JWT_SECRET 已更改（生產環境）
- [ ] .env 文件已添加到 .gitignore

---

## 🚀 快速配置

### Dry-Run 測試
```env
# 最小配置
USE_MOCK_FEED=true
ENABLE_BINANCE_TRADING=false
```

### 真實數據測試（不下單）
```env
BINANCE_TESTNET=true
BINANCE_API_KEY=your-testnet-key
BINANCE_API_SECRET=your-testnet-secret
USE_MOCK_FEED=false
ENABLE_BINANCE_TRADING=false
```

### 小額實盤
```env
BINANCE_TESTNET=false
BINANCE_API_KEY=your-prod-key
BINANCE_API_SECRET=your-prod-secret
USE_MOCK_FEED=false
ENABLE_BINANCE_TRADING=true
BINANCE_SYMBOLS=BTCUSDT  # 先測試單一幣種
```

---

**配置完成後記得測試**: `go run main.go`
