# Python Strategy Layer

Python 策略執行層,提供 gRPC 服務與 Go 後端通訊。

## 目錄結構

```
python/
├── strategies/          # 交易策略
│   ├── base.py         # 策略基礎類別
│   └── example_grid.py # 網格策略範例
├── worker/             # gRPC Worker
│   ├── main.py         # 主程式
│   └── proto/          # 自動生成的 protobuf 檔案
├── alert/              # 告警系統
│   ├── main.py         # 告警主程式
│   ├── notifier.py     # 通知介面
│   └── telegram.py     # Telegram 通知
└── requirements.txt    # Python 依賴
```

## 安裝

```bash
cd python
pip install -r requirements.txt
```

## 使用方式

### 啟動策略 Worker

```bash
cd worker
python main.py
```

Worker 會在 port 50051 啟動 gRPC 服務,等待 Go 後端的策略請求。

### 測試告警系統

```bash
cd alert
python main.py
```

## 開發自己的策略

1. 繼承 `strategies/base.py` 中的 `BaseStrategy`
2. 實作 `on_tick()` 方法
3. 在 `worker/main.py` 中註冊你的策略

範例請參考 `strategies/example_grid.py`

## 依賴說明

- **grpcio** - gRPC 核心庫
- **grpcio-tools** - protobuf 編譯工具
- **requests** - HTTP 請求 (用於告警)
- **python-dotenv** - 環境變數管理
