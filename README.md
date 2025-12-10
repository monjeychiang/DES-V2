# DES Trading System V2.0

> **Dynamic Execution System** - 高性能量化交易系統

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8.svg)](https://golang.org/)
[![Python Version](https://img.shields.io/badge/python-3.10+-3776AB.svg)](https://www.python.org/)
[![React Version](https://img.shields.io/badge/react-19.2-61DAFB.svg)](https://reactjs.org/)

## 📋 專案簡介

DES Trading System V2.0 是一個專為加密貨幣交易設計的高性能量化交易系統，採用 Go + Python 混合架構，提供:

- **高性能交易引擎** - Go 語言實現的低延遲訂單執行
- **多用戶多帳戶** - 完整資料隔離，支援 SaaS 部署
- **靈活策略框架** - Python 策略開發與回測環境
- **實時風險管理** - 多層次風險控制與監控
- **Web 管理介面** - React 前端提供直觀的系統管理

## 🏗️ 系統架構

```
┌─────────────────────────────────────────────────────────┐
│                    Frontend (React)                      │
│              Web UI + Real-time Dashboard                │
└─────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────┐
│                  Backend (Go Core)                       │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │ Engine       │  │ Risk Manager │  │ Market Data  │  │
│  │   Service    │  │              │  │              │  │
│  └──────────────┘  └──────────────┘  └──────────────┘  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │ Order Engine │  │ Position Mgr │  │ Event Bus    │  │
│  └──────────────┘  └──────────────┘  └──────────────┘  │
└─────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────┐
│                Python Strategy Layer                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │  Strategies  │  │   Backtest   │  │   Alerts     │  │
│  └──────────────┘  └──────────────┘  └──────────────┘  │
└─────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────┐
│                  Exchange APIs                           │
│              Binance Spot + Futures                      │
└─────────────────────────────────────────────────────────┘
```

## 🚀 快速開始

### 前置需求

- **Go** 1.21 或更高版本
- **Python** 3.10 或更高版本
- **Node.js** 18 或更高版本
- **SQLite** 3.x (內建)

### 安裝步驟

1. **克隆專案**
   ```bash
   git clone https://github.com/monjeychiang/DES-V2.git
   cd DES-V2
   ```

2. **配置環境變數**
   ```bash
   cp .env.example .env
   # 編輯 .env 並填入你的 API 金鑰
   ```

3. **啟動後端服務**
   ```bash
   cd backend/cmd/trading-core
   go run main.go
   ```

4. **啟動前端介面**
   ```bash
   cd frontend
   npm install
   npm run dev
   ```

5. **訪問系統**
   - 前端介面: http://localhost:5173
   - 後端 API: http://localhost:8080

## 📚 文件導覽

### 核心文件
| 文件 | 說明 |
|------|------|
| [系統架構](docs/architecture/SYSTEM_ARCHITECTURE.md) | 完整的系統架構說明 |
| [多用戶架構](docs/architecture/MULTI_USER_REFACTORING.md) | 多用戶多帳戶設計 |
| [多用戶使用指南](docs/guides/MULTI_USER_USAGE_GUIDE.md) | API 使用流程 |
| [開發者入門](docs/process/DEVELOPER_ONBOARDING.md) | 新手開發指南 |
| [快速參考](docs/setup/QUICK_REFERENCE.md) | 常用指令與 API 參考 |

### API 與配置
| 文件 | 說明 |
|------|------|
| [API 文件](docs/api/API.md) | REST API 規格 |
| [Trading API 審計](docs/api/TRADING_API_AUDIT.md) | API 完整性評估 (A 級) |
| [環境變數指南](docs/setup/ENV_VARIABLES_GUIDE.md) | 配置說明 |

### 架構與規劃
| 文件 | 說明 |
|------|------|
| [服務架構路線圖 V2](docs/architecture/SERVICE_ARCHITECTURE_ROADMAP_V2.md) | 架構演進規劃 |
| [性能分析](docs/architecture/PERFORMANCE_ANALYSIS.md) | 性能瓶頸與優化 |
| [交易所串接指南](docs/development/EXCHANGE_INTEGRATION_GUIDE.md) | 新交易所開發規範 |

## 🔧 專案結構

```
DES-V2/
├── backend/                      # Go 後端核心
│   └── cmd/trading-core/
│       ├── main.go               # 應用程式入口
│       ├── internal/
│       │   ├── engine/           # Engine 服務層 (NEW)
│       │   ├── api/              # REST API
│       │   ├── strategy/         # 策略引擎
│       │   ├── order/            # 訂單執行
│       │   ├── risk/             # 風險管理
│       │   └── events/           # 事件匯流排
│       └── pkg/
│           ├── db/               # 資料庫
│           └── exchanges/        # 交易所 Gateway
├── frontend/                     # React 前端
├── python/                       # Python 策略層
├── proto/                        # gRPC 協議定義
├── docs/                         # 詳細文件
└── scripts/                      # 工具腳本
```

## 🎯 核心功能

### ✅ 已實現
| 功能 | 狀態 | 說明 |
|------|------|------|
| Binance 市場數據 | ✅ | Spot + USDT Futures + Coin Futures |
| 訂單執行 | ✅ | 同步/異步執行，含 Worker Pool |
| 風險控制 | ✅ | 多層次風險檢查，止損/止盈 |
| 倉位追蹤 | ✅ | 實時對帳，支援 User Data Stream |
| **多用戶架構** | ✅ | 資料隔離、API Key 加密、Per-User 風控 |
| Engine 服務層 | ✅ | 介面隔離架構 (Phase 1 完成) |
| REST API | ✅ | 完整 CRUD + JWT Auth |
| React 管理後台 | ✅ | 實時儀表板 |

### 🚧 規劃中
| 功能 | 狀態 | 說明 |
|------|------|------|
| WebSocket 推送 | 🔄 | 部分實現 |
| 高級回測引擎 | 📋 | 規劃中 |
| 多交易所支援 | 📋 | [開發指南](docs/development/EXCHANGE_INTEGRATION_GUIDE.md) |
| 服務拆分 (Phase 2) | 📋 | [架構路線圖](docs/architecture/SERVICE_ARCHITECTURE_ROADMAP_V2.md) |

## 🛡️ 風險管理

系統內建多層風險控制:
- **訂單級別**: 價格偏差檢查、數量限制
- **帳戶級別**: 總倉位限制、保證金監控
- **系統級別**: 緊急熔斷機制、Panic Recovery

## 📊 性能改進

V2 版本包含多項性能優化：
- **Worker Pool** - 限制並發 Goroutine
- **Async Executor** - 非阻塞訂單執行
- **Lazy Stats** - O(1) 統計查詢
- **Batched Drain** - 減少鎖競爭
- **Multi-User Sharding** - 索引優化查詢

壓力測試結果：
- **250,000 訂單 / 500 用戶** - 25 秒完成
- **吞吐量** - ~10,000 orders/sec
- **資料隔離** - 100% 驗證通過

詳見 [性能優化計畫](docs/architecture/MULTI_USER_PERFORMANCE_OPTIMIZATION.md)

## 🤝 貢獻指南

歡迎提交 Issue 和 Pull Request!

1. Fork 本專案
2. 建立特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交變更 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 開啟 Pull Request

## 📄 授權

本專案採用 MIT 授權 - 詳見 [LICENSE](LICENSE) 文件

## 📞 聯絡方式

- **專案維護者**: monjeychiang
- **Email**: aabb5744176@gmail.com
- **問題回報**: [GitHub Issues](https://github.com/monjeychiang/DES-V2/issues)

## 🙏 致謝

- [Binance API](https://binance-docs.github.io/apidocs/) - 交易所 API
- [Go](https://golang.org/) - 後端核心語言
- [React](https://reactjs.org/) - 前端框架
- [Python](https://www.python.org/) - 策略開發語言

---

**⚠️ 風險提示**: 本系統僅供學習和研究使用。加密貨幣交易存在高風險，請謹慎使用並自負盈虧。
