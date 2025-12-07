# DES Trading System - Frontend

React 前端管理介面，基於 Vite 建構。

## 📋 功能

- 📊 即時交易儀表板
- 📈 訂單管理與監控
- 💼 倉位追蹤
- ⚠️ 風險控制面板
- 🔧 系統設定

## 🚀 快速開始

### 安裝依賴

```bash
npm install
```

### 開發模式

```bash
npm run dev
```

訪問 http://localhost:5173

### 建置生產版本

```bash
npm run build
```

### 程式碼檢查

```bash
npm run lint
```

## 📁 目錄結構

```
frontend/
├── src/
│   ├── components/     # React 元件
│   ├── pages/          # 頁面元件
│   ├── hooks/          # 自定義 Hooks
│   ├── services/       # API 服務
│   ├── utils/          # 工具函數
│   ├── App.jsx         # 主應用程式
│   └── main.jsx        # 入口點
├── public/             # 靜態資源
├── index.html          # HTML 模板
├── vite.config.js      # Vite 配置
└── package.json        # 依賴配置
```

## 🔧 技術棧

- **React** 19.2 - UI 框架
- **Vite** 7.2 - 建構工具
- **React Router** 7.9 - 路由管理
- **Axios** 1.13 - HTTP 請求
- **TailwindCSS** 3.4 - CSS 框架

## 🔗 後端連接

預設連接到 `http://localhost:8080` 的 Go 後端 API。

可以在 `.env` 中配置：

```
VITE_API_URL=http://localhost:8080
```

## 📚 相關文件

- [系統架構](../docs/architecture/SYSTEM_ARCHITECTURE.md)
- [API 文件](../docs/api/API.md)
- [開發者指南](../docs/process/DEVELOPER_ONBOARDING.md)
