# DES Trading System V2.0

> **Dynamic Execution System** - 高性能量化交易系統

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8.svg)](https://golang.org/)
[![Python Version](https://img.shields.io/badge/python-3.10+-3776AB.svg)](https://www.python.org/)
[![React Version](https://img.shields.io/badge/react-19.2-61DAFB.svg)](https://reactjs.org/)

## 📋 專案簡介

DES Trading System V2.0 是一個專為加密貨幣交易設計的高性能量化交易系統,採用 Go + Python 混合架構,提供:

- **高性能交易引擎** - Go 語言實現的低延遲訂單執行
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
│  │ Trading Core │  │ Risk Manager │  │ Market Data  │  │
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
   # 複製範例配置檔
   cp .env.example .env
   
   # 編輯 .env 並填入你的 API 金鑰
   # 詳見 docs/setup/ENV_VARIABLES_GUIDE.md
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
- [系統架構](docs/architecture/SYSTEM_ARCHITECTURE.md) - 完整的系統架構說明
- [開發者入門](docs/process/DEVELOPER_ONBOARDING.md) - 新手開發指南
- [快速參考](docs/setup/QUICK_REFERENCE.md) - 常用指令與 API 參考

### 詳細文件 (docs/)
- [API 文件](docs/api/API.md) - REST API 規格
- [環境變數指南](docs/setup/ENV_VARIABLES_GUIDE.md) - 配置說明
- [專案狀態](docs/roadmap/PROJECT_STATUS.md) - 當前開發進度
- [開發路線圖](docs/roadmap/DEVELOPMENT_ROADMAP_DES_V2.md) - 未來規劃

## 🔧 專案結構

```
DES-V2/
├── backend/              # Go 後端核心
│   └── cmd/
│       └── trading-core/ # 主要交易引擎
├── frontend/             # React 前端
├── python/               # Python 策略層
│   ├── strategies/       # 交易策略
│   ├── worker/           # 策略執行器
│   └── alert/            # 告警系統
├── proto/                # gRPC 協議定義
├── scripts/              # 工具腳本
├── docs/                 # 詳細文件
└── license-server/       # 授權服務 (規劃中)
```

## 🎯 核心功能

### ✅ 已實現
- ✓ Binance Spot/Futures 市場數據訂閱
- ✓ 實時訂單執行與管理
- ✓ 多層次風險控制系統
- ✓ 倉位追蹤與對帳
- ✓ SQLite 數據持久化
- ✓ RESTful API 介面
- ✓ React 管理後台
- ✓ Python 策略框架

### 🚧 開發中
- ⏳ WebSocket 實時推送
- ⏳ 高級策略回測引擎
- ⏳ 多交易所支援
- ⏳ 雲端部署方案

## 🛡️ 風險管理

系統內建多層風險控制:
- **訂單級別**: 價格偏差檢查、數量限制
- **帳戶級別**: 總倉位限制、保證金監控
- **系統級別**: 緊急熔斷機制、異常檢測

詳見 [風險管理文件](docs/design/ADVANCED_FEATURES_DESIGN.md)

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

**⚠️ 風險提示**: 本系統僅供學習和研究使用。加密貨幣交易存在高風險,請謹慎使用並自負盈虧。
