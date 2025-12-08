# 前端完整產品化設計 (V2 - TypeScript + shadcn/ui)

> **狀態**: 🚀 實作中  
> **策略**: 完全重寫  
> **目標**: 專業級量化交易系統界面

---

## 📊 技術選型

### 核心技術棧

| 類別 | 技術 | 版本 | 說明 |
|------|------|------|------|
| **語言** | TypeScript | 5.x | 類型安全、IDE 支援 |
| **框架** | React | 19.x | UI 框架 |
| **構建** | Vite | 7.x | 快速 HMR、ESM |
| **路由** | React Router | 7.x | 聲明式路由 |

### UI 與樣式

| 類別 | 技術 | 說明 |
|------|------|------|
| **組件庫** | shadcn/ui | Radix UI + Tailwind 整合 |
| **樣式** | Tailwind CSS v3 | 原子化 CSS |
| **圖標** | Lucide React | 輕量一致風格 |
| **動畫** | Framer Motion | 流暢過渡效果 |

### 數據與狀態

| 類別 | 技術 | 說明 |
|------|------|------|
| **圖表** | Recharts | 聲明式 React 圖表 |
| **狀態管理** | Zustand | 輕量 TypeScript 友好 |
| **數據獲取** | TanStack Query v5 | 緩存、重試、背景刷新 |
| **表單驗證** | React Hook Form + Zod | 類型安全驗證 |
| **國際化** | react-i18next | 中/英文切換 |
| **日期處理** | date-fns | 輕量 Tree-shakable |

---

## 🏗️ 專案結構

```
frontend/
├── src/
│   ├── app/                    # 應用入口
│   │   ├── App.tsx
│   │   ├── main.tsx
│   │   └── routes.tsx
│   │
│   ├── components/
│   │   ├── ui/                 # shadcn/ui 組件 (自動生成)
│   │   │   ├── button.tsx
│   │   │   ├── card.tsx
│   │   │   ├── dialog.tsx
│   │   │   └── ...
│   │   └── shared/             # 業務通用組件
│   │       ├── StatusBadge.tsx
│   │       ├── PnlDisplay.tsx
│   │       └── LanguageSwitcher.tsx
│   │
│   ├── features/               # 功能模組 (按業務劃分)
│   │   ├── auth/
│   │   ├── dashboard/
│   │   ├── strategies/
│   │   ├── orders/
│   │   ├── performance/
│   │   └── settings/
│   │
│   ├── layouts/                # 佈局組件
│   │   ├── MainLayout.tsx
│   │   ├── AuthLayout.tsx
│   │   ├── Sidebar.tsx
│   │   ├── Header.tsx
│   │   └── StatusBar.tsx
│   │
│   ├── pages/                  # 頁面組件
│   │   ├── LoginPage.tsx
│   │   ├── DashboardPage.tsx
│   │   ├── StrategiesPage.tsx
│   │   ├── StrategyDetailPage.tsx
│   │   ├── OrdersPage.tsx
│   │   ├── PerformancePage.tsx
│   │   └── SettingsPage.tsx
│   │
│   ├── hooks/                  # 全局 Hooks
│   ├── lib/                    # 工具函數 (api, utils)
│   ├── stores/                 # Zustand Stores
│   ├── types/                  # TypeScript 類型
│   │
│   ├── i18n/                   # 國際化
│   │   ├── index.ts
│   │   └── locales/
│   │       ├── zh-TW.json
│   │       └── en.json
│   │
│   └── styles/
│       └── globals.css
```

---

## 📱 頁面規劃

### 頁面清單

| 頁面 | 路徑 | 功能 |
|------|------|------|
| 登入 | `/login` | 認證頁面 (登入/註冊) |
| 儀表板 | `/` | 總覽、餘額、快速操作 |
| 策略管理 | `/strategies` | 列表、控制、創建 |
| 策略詳情 | `/strategies/:id` | 單策略詳細、圖表、日誌 |
| 訂單歷史 | `/orders` | 訂單列表、篩選、導出 |
| 績效報告 | `/performance` | PnL 曲線、統計指標 |
| 設置 | `/settings` | 連線、風控、通知、外觀 |

### 佈局結構

```
┌─────────────────────────────────────────────────────────────────┐
│                        Header                                    │
│  [Logo]  [Search]              [Lang] [Theme] [Notifications] [👤]│
├────────────┬────────────────────────────────────────────────────┤
│            │                                                    │
│  Sidebar   │              Main Content                          │
│  ┌──────┐  │                                                    │
│  │ 儀表板 │  │   ┌─────────────────────────────────────────────┐  │
│  │ 策略  │  │   │  Page Content                               │  │
│  │ 訂單  │  │   │                                             │  │
│  │ 績效  │  │   │                                             │  │
│  │ 設置  │  │   └─────────────────────────────────────────────┘  │
│  └──────┘  │                                                    │
├────────────┴────────────────────────────────────────────────────┤
│                    Status Bar (系統狀態)                          │
└─────────────────────────────────────────────────────────────────┘
```

---

## 🧩 頁面組件詳細

### 儀表板 (DashboardPage)

| 組件 | 類型 | 功能 |
|------|------|------|
| BalanceCard | Card | 總資產、可用餘額、保證金 |
| QuickActionsCard | Card | 快速啟動/停止策略 |
| ActiveStrategiesWidget | Custom | 運行中策略 (Top 5) |
| RecentOrdersWidget | Custom | 最近訂單 (Top 10) |
| PositionSummary | Custom | 持倉概覽 |
| SystemHealthCard | Card | 連線狀態 |

### 策略管理 (StrategiesPage)

| 組件 | 類型 | 功能 |
|------|------|------|
| StrategyFilters | Select/Input | 狀態、類型篩選 |
| StrategyTable | DataTable | 策略列表 (排序/分頁) |
| StrategyActions | DropdownMenu | 啟動/暫停/停止/編輯 |
| CreateStrategyDialog | Dialog | 新建策略表單 |
| BindConnectionSelect | Select | 綁定交易連線 |

### 策略詳情 (StrategyDetailPage)

| 組件 | 類型 | 功能 |
|------|------|------|
| StrategyHeader | Custom | 名稱、狀態、操作按鈕 |
| StrategyStatsGrid | Card Grid | PnL、交易次數、勝率 |
| StrategyPnlChart | Recharts | 績效曲線 |
| StrategyParamsCard | Card | 參數配置 |
| StrategyLogsTable | Table | 策略日誌 |

### 訂單歷史 (OrdersPage)

| 組件 | 類型 | 功能 |
|------|------|------|
| OrderFilters | DatePicker/Select | 時間、狀態篩選 |
| OrderStatsBar | Custom | 統計欄 |
| OrderTable | DataTable | 訂單列表 |
| OrderDetailSheet | Sheet | 詳情側邊欄 |
| ExportButton | Button | CSV 導出 |

### 績效報告 (PerformancePage)

| 組件 | 類型 | 功能 |
|------|------|------|
| DateRangePicker | DatePicker | 時間範圍 |
| PnlCurveChart | Recharts AreaChart | 累計 PnL |
| DrawdownChart | Recharts AreaChart | 回撤曲線 |
| PerformanceMetrics | Card Grid | 夏普、最大回撤、勝率 |
| MonthlyReturnsTable | Table | 月度收益 |

### 設置 (SettingsPage)

| 組件 | 類型 | 功能 |
|------|------|------|
| SettingsTabs | Tabs | 分類標籤 |
| ConnectionsList | Table | 已保存連線 |
| AddConnectionDialog | Dialog | 新增連線 |
| RiskParamsForm | Form | 風控參數 |
| ThemeToggle | Switch | 深/淺色模式 |
| LanguageSwitcher | DropdownMenu | 中/英切換 |

### 登入 (LoginPage)

| 組件 | 類型 | 功能 |
|------|------|------|
| AuthLayout | Custom | 品牌背景 + 表單區 |
| AuthTabs | Tabs | 登入/註冊切換 |
| LoginForm | Form | Email + 密碼 |
| RegisterForm | Form | 註冊表單 |

---

## 🎨 設計系統

### 配色方案

```css
/* 主色 */
--primary: #2563eb;        /* 藍色 */
--primary-foreground: #ffffff;

/* 狀態色 */
--success: #10b981;        /* 綠色 (盈利) */
--danger: #ef4444;         /* 紅色 (虧損) */
--warning: #f59e0b;        /* 橙色 (警告) */

/* 深色模式 */
--background: #0f172a;
--foreground: #f8fafc;
--card: #1e293b;
--border: #334155;
```

### 狀態指示

| 策略狀態 | 顏色 | Badge |
|----------|------|-------|
| ACTIVE | 綠色 | `bg-green-100 text-green-700` |
| PAUSED | 黃色 | `bg-yellow-100 text-yellow-700` |
| STOPPED | 灰色 | `bg-gray-100 text-gray-700` |
| ERROR | 紅色 | `bg-red-100 text-red-700` |

### 響應式斷點

| 尺寸 | 寬度 | 佈局 |
|------|------|------|
| Mobile | < 640px | 單欄、底部導航 |
| Tablet | 640-1024px | 摺疊側邊欄 |
| Desktop | > 1024px | 完整側邊欄 |

---

## 📋 開發任務

### Phase 1: 專案初始化 (0.5 天)
- [ ] 建立 Vite + TypeScript 專案
- [ ] 安裝 shadcn/ui + 依賴
- [ ] 配置 i18n
- [ ] 建立目錄結構

### Phase 2: 佈局與認證 (1 天)
- [ ] MainLayout + AuthLayout
- [ ] Sidebar + Header + StatusBar
- [ ] LoginPage
- [ ] LanguageSwitcher

### Phase 3: 核心頁面 (2 天)
- [ ] DashboardPage
- [ ] StrategiesPage + StrategyDetailPage
- [ ] OrdersPage

### Phase 4: 進階功能 (1.5 天)
- [ ] PerformancePage (Recharts)
- [ ] SettingsPage
- [ ] WebSocket 即時更新
- [ ] Toast 通知

### Phase 5: 優化 (1 天)
- [ ] 響應式設計
- [ ] 深色模式
- [ ] 翻譯完成
- [ ] 錯誤處理

**總預估**: 6 天

---

## 🔧 shadcn/ui 組件

```bash
npx shadcn@latest add button card dialog dropdown-menu input label select table tabs toast sheet form switch badge avatar separator scroll-area command popover calendar
```

---

## 📦 依賴清單

```json
{
  "dependencies": {
    "react": "^19.0.0",
    "react-dom": "^19.0.0",
    "react-router-dom": "^7.0.0",
    "@tanstack/react-query": "^5.0.0",
    "zustand": "^5.0.0",
    "recharts": "^2.12.0",
    "react-i18next": "^14.0.0",
    "i18next": "^23.0.0",
    "react-hook-form": "^7.50.0",
    "@hookform/resolvers": "^3.3.0",
    "zod": "^3.22.0",
    "date-fns": "^3.0.0",
    "lucide-react": "^0.400.0",
    "framer-motion": "^11.0.0",
    "clsx": "^2.1.0",
    "tailwind-merge": "^2.2.0",
    "class-variance-authority": "^0.7.0",
    "@radix-ui/react-*": "latest"
  }
}
```

---

*文檔更新於 2024-12-08*
