# Strategy Instance Features Proposal

除了基礎的 **開始 (Start)**、**暫停 (Pause)**、**結束 (Stop)**、**修改參數 (Edit Params)**、**止盈止損 (TP/SL)** 之外，一個成熟的策略實例通常還包含以下功能：

## 1. 生命週期與狀態控制 (Lifecycle & State)
- **平倉並停止 (Panic Sell / Force Exit)**: 立即市價平掉該策略的所有持倉並停止策略。
- **重置狀態 (Reset State)**: 清除策略的內部狀態（如指標記憶、累計虧損計數），使其像新策略一樣重新開始。
- **克隆 (Clone)**: 複製當前策略及其參數到一個新實例，方便進行 A/B 測試。
- **歸檔 (Archive)**: 將已結束的策略隱藏，但保留歷史數據以供分析，不佔用活躍列表。

## 2. 資金與倉位管理 (Money & Position Management)
- **加倉/補倉設置 (DCA / Safety Orders)**:
    - 當價格向不利方向移動 X% 時，自動加倉。
    - 設置最大加倉次數 (Max Safety Orders)。
    - 加倉倍率 (Volume Scale)。
- **槓桿設置 (Leverage)**: 針對該策略的專屬槓桿倍數（如 5x, 10x）。
- **最大持倉限制 (Max Position Amount)**: 該策略允許持有的最大名義價值（防止無限加倉）。
- **冷卻時間 (Cooldown)**: 平倉後暫停 X 分鐘再開倉（防止頻繁磨損）。

## 3. 執行邏輯 (Execution Logic)
- **訂單類型 (Order Type)**:
    - **Maker Only (Post Only)**: 僅掛單，節省手續費。
    - **Taker**: 立即吃單，保證成交。
- **交易時段 (Trading Hours)**: 僅在特定時間段（如美股開盤）交易。
- **滑點控制 (Max Slippage)**: 超過預期價格 X% 則不成交。

## 4. 高級觸發器 (Advanced Triggers)
- **自動啟動條件 (Auto Start Condition)**: 當 RSI < 30 或價格低於 X 時自動啟動。
- **自動停止條件 (Auto Stop Condition)**:
    - **利潤目標**: 總利潤達到 X USDT 時自動停止。
    - **回撤保護**: 總資產回撤超過 X% 時自動停止。
- **Webhook 控制**: 允許 TradingView 或外部信號控制該策略的開關。

## 5. 分析與標籤 (Analysis & Meta)
- **標籤 (Tags)**: 給策略打標籤（如 "Bull Market", "High Risk"）。
- **備註 (Notes)**: 記錄策略的思路或調整日誌。
- **專屬日誌 (Instance Logs)**: 僅顯示該策略相關的報錯和交易日誌，過濾掉系統雜訊。
