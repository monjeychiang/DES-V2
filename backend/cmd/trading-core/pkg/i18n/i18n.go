package i18n

import (
	"reflect"
	"sync"
)

// Language type
type Language string

const (
	LangEN Language = "en"
	LangZH Language = "zh"
)

// Messages holds all translatable strings
type Messages struct {
	// System
	Starting             string
	ConfigLoaded         string
	UsingDBPath          string
	ServerListening      string
	ShuttingDown         string
	StrategySaveComplete string
	SystemMetricsInit    string
	EngineServiceInit    string
	DryRunMode           string
	ConfigLoadFailed     string
	DBInitFailed         string
	DBMigrationsFailed   string
	StateLoadFailed      string
	APIServerError       string

	// Balance
	BalanceInitialized     string
	BalanceLocked          string
	BalanceDeducted        string
	BalanceAdded           string
	InsufficientBalance    string
	BalanceManagerStarted  string
	BalanceManagerFallback string

	// Orders
	OrderExecuted          string
	OrderFailed            string
	OrderRetrying          string
	OrderWalEnabled        string
	PersistentQueueFailed  string
	WalRecoveryError       string
	StopLossTriggered      string
	UnknownFilledOrderType string
	UsingCachedPrice       string
	FillPriceZeroFallback  string
	AsyncExecutionFailed   string

	// Risk
	RiskRejected            string
	RiskWarning             string
	RiskManagerInit         string
	RiskManagerInitFailed   string
	DailyLossLimitReached   string
	OrderTooSmall           string
	OrderTooLarge           string
	RiskMetricsUpdateFailed string
	BalanceLockFailed       string

	// Positions
	PositionUpdated string
	PositionOpened  string
	PositionClosed  string
	RealizedPnL     string

	// Strategy
	StrategySignal           string
	StrategyLoaded           string
	StrategyPaused           string
	StrategyStopped          string
	StrategyConfigLoadFailed string
	StrategySyncFailed       string
	StrategyLoadFromDBFailed string
	PythonWorkerEnabled      string
	PythonWorkerInitFailed   string
	SignalProcessingPanic    string
	GlobalRiskDisabled       string
	StrategyRiskDisabled     string

	// Services
	ReconStarted       string
	ReconNotSupported  string
	BinanceFeedStarted string
	MockFeedStarted    string
}

var (
	currentLang Language = LangEN
	mu          sync.RWMutex
	messages    *Messages
)

// English messages
var messagesEN = Messages{
	// System
	Starting:             "Starting DES Trading System...",
	ConfigLoaded:         "Config loaded (Port: %s)",
	UsingDBPath:          "Using DB path: %s",
	ServerListening:      "Server listening on :%s",
	ShuttingDown:         "Shutting down gracefully...",
	StrategySaveComplete: "All strategy states saved.",
	SystemMetricsInit:    "System metrics initialized",
	EngineServiceInit:    "Engine service initialized",
	DryRunMode:           "Running in DRY-RUN mode (orders will NOT hit exchange)",
	ConfigLoadFailed:     "Failed to load config: %v",
	DBInitFailed:         "Failed to init database: %v",
	DBMigrationsFailed:   "Failed to apply migrations: %v",
	StateLoadFailed:      "Failed to load state: %v",
	APIServerError:       "API server error: %v",

	// Balance
	BalanceInitialized:     "Dry-run balance initialized: %.2f",
	BalanceLocked:          "Balance locked: %.2f (Available: %.2f)",
	BalanceDeducted:        "Balance deducted: %.2f (Total: %.2f)",
	BalanceAdded:           "Balance added: %.2f (Total: %.2f)",
	InsufficientBalance:    "Insufficient balance: need %.2f, have %.2f",
	BalanceManagerStarted:  "Balance manager started with exchange sync",
	BalanceManagerFallback: "Balance manager: gateway doesn't support GetBalance, using default",

	// Orders
	OrderExecuted:          "Order %s executed (latency: %v)",
	OrderFailed:            "Order %s failed: %v (latency: %v)",
	OrderRetrying:          "Order %s retrying (attempt %d/%d)...",
	OrderWalEnabled:        "Persistent order queue enabled: %s",
	PersistentQueueFailed:  "Failed to create persistent queue: %v, falling back to in-memory",
	WalRecoveryError:       "WAL recovery error: %v",
	StopLossTriggered:      "Stop loss triggered: %s %s %.4f - %s",
	UnknownFilledOrderType: "Unknown filled order type: %T",
	UsingCachedPrice:       "Using cached price for %s: %.2f",
	FillPriceZeroFallback:  "Warning: fillPrice is 0 for %s, using fallback",
	AsyncExecutionFailed:   "Async execution failed for order %s: %v",

	// Risk
	RiskRejected:            "Risk rejected: %s",
	RiskWarning:             "Risk warning: %s",
	RiskManagerInit:         "Risk manager initialized: stop_loss=%.1f%% take_profit=%.1f%%",
	RiskManagerInitFailed:   "Risk manager init failed, fallback to default in-memory: %v",
	DailyLossLimitReached:   "Daily loss limit reached (LIMIT)",
	OrderTooSmall:           "Order too small: %.2f < %.2f",
	OrderTooLarge:           "Order too large: %.2f > %.2f",
	RiskMetricsUpdateFailed: "Failed to update risk metrics: %v",
	BalanceLockFailed:       "Balance lock failed: %v",

	// Positions
	PositionUpdated: "Position updated: %s %.4f @ %.2f",
	PositionOpened:  "Opening/Adding position: %s %s %.4f @ %.2f",
	PositionClosed:  "Position closed, stop loss removed: %s",
	RealizedPnL:     "Realized PnL: %.2f (%s %s %.4f @ %.2f)",

	// Strategy
	StrategySignal:           "Strategy %s signal: %+v",
	StrategyLoaded:           "Loaded strategy: %s (%s)",
	StrategyPaused:           "Strategy %s paused",
	StrategyStopped:          "Strategy %s stopped",
	StrategyConfigLoadFailed: "Failed to load strategies.yaml: %v",
	StrategySyncFailed:       "Failed to sync strategies to DB: %v",
	StrategyLoadFromDBFailed: "Failed to load strategies from DB: %v",
	PythonWorkerEnabled:      "Python worker enabled at %s",
	PythonWorkerInitFailed:   "Python worker client init failed: %v",
	SignalProcessingPanic:    "PANIC in signal processing: %v",
	GlobalRiskDisabled:       "Global risk checks disabled",
	StrategyRiskDisabled:     "Risk checks disabled for strategy %s",

	// Services
	ReconStarted:       "Reconciliation service started",
	ReconNotSupported:  "Reconciliation: gateway doesn't support GetPositions",
	BinanceFeedStarted: "Binance feed started",
	MockFeedStarted:    "Mock feed started",
}

// Chinese messages
var messagesZH = Messages{
	// System
	Starting:             "啟動 DES 交易系統...",
	ConfigLoaded:         "設定已載入（埠號：%s）",
	UsingDBPath:          "使用資料庫路徑：%s",
	ServerListening:      "服務監聽於 :%s",
	ShuttingDown:         "正在優雅關閉...",
	StrategySaveComplete: "策略狀態已全部保存。",
	SystemMetricsInit:    "系統指標初始化完成",
	EngineServiceInit:    "引擎服務初始化完成",
	DryRunMode:           "DRY-RUN 模式（不會送出真實委託）",
	ConfigLoadFailed:     "讀取設定失敗：%v",
	DBInitFailed:         "初始化資料庫失敗：%v",
	DBMigrationsFailed:   "套用資料庫遷移失敗：%v",
	StateLoadFailed:      "載入狀態失敗：%v",
	APIServerError:       "API 伺服器錯誤：%v",

	// Balance
	BalanceInitialized:     "模擬資金已初始化：%.2f",
	BalanceLocked:          "資金已鎖定：%.2f（可用：%.2f）",
	BalanceDeducted:        "資金已扣減：%.2f（總額：%.2f）",
	BalanceAdded:           "資金已增加：%.2f（總額：%.2f）",
	InsufficientBalance:    "餘額不足：需求 %.2f，現有 %.2f",
	BalanceManagerStarted:  "資金管理器已啟動並同步交易所餘額",
	BalanceManagerFallback: "資金管理：通道不支援查詢餘額，使用預設模式",

	// Orders
	OrderExecuted:          "訂單 %s 已成交（延遲：%v）",
	OrderFailed:            "訂單 %s 失敗：%v（延遲：%v）",
	OrderRetrying:          "訂單 %s 重試中（第 %d/%d 次）...",
	OrderWalEnabled:        "持久化訂單佇列已啟用：%s",
	PersistentQueueFailed:  "建立持久化佇列失敗：%v，改用記憶體佇列",
	WalRecoveryError:       "WAL 還原錯誤：%v",
	StopLossTriggered:      "觸發停損：%s %s %.4f - %s",
	UnknownFilledOrderType: "未知的成交訊息型態：%T",
	UsingCachedPrice:       "使用快取價格 %s：%.2f",
	FillPriceZeroFallback:  "警告：%s 的成交價為 0，使用備援值",
	AsyncExecutionFailed:   "非同步下單失敗，訂單 %s：%v",

	// Risk
	RiskRejected:            "風控拒絕：%s",
	RiskWarning:             "風控警告：%s",
	RiskManagerInit:         "風控管理器初始化：停損=%.1f%% 停利=%.1f%%",
	RiskManagerInitFailed:   "風控管理器初始化失敗，改用預設記憶體設定：%v",
	DailyLossLimitReached:   "已達每日虧損上限（LIMIT）",
	OrderTooSmall:           "訂單金額過小：%.2f < %.2f",
	OrderTooLarge:           "訂單金額過大：%.2f > %.2f",
	RiskMetricsUpdateFailed: "更新風控指標失敗：%v",
	BalanceLockFailed:       "鎖定資金失敗：%v",

	// Positions
	PositionUpdated: "持倉更新：%s %.4f @ %.2f",
	PositionOpened:  "建立/加倉：%s %s %.4f @ %.2f",
	PositionClosed:  "持倉已平倉，移除停損：%s",
	RealizedPnL:     "已實現損益：%.2f（%s %s %.4f @ %.2f）",

	// Strategy
	StrategySignal:           "策略 %s 訊號：%+v",
	StrategyLoaded:           "已載入策略：%s（%s）",
	StrategyPaused:           "策略 %s 已暫停",
	StrategyStopped:          "策略 %s 已停止",
	StrategyConfigLoadFailed: "讀取 strategies.yaml 失敗：%v",
	StrategySyncFailed:       "同步策略到資料庫失敗：%v",
	StrategyLoadFromDBFailed: "從資料庫載入策略失敗：%v",
	PythonWorkerEnabled:      "Python worker 已啟用，位址 %s",
	PythonWorkerInitFailed:   "初始化 Python worker 客戶端失敗：%v",
	SignalProcessingPanic:    "處理策略訊號時發生 PANIC：%v",
	GlobalRiskDisabled:       "全域風控檢查已停用",
	StrategyRiskDisabled:     "策略 %s 的風控檢查已停用",

	// Services
	ReconStarted:       "對帳服務已啟動",
	ReconNotSupported:  "對帳：通道不支援取得持倉",
	BinanceFeedStarted: "Binance 行情訂閱已啟動",
	MockFeedStarted:    "模擬行情訂閱已啟動",
}

func init() {
	messages = &messagesEN
}

// SetLanguage sets the current language
func SetLanguage(lang Language) {
	mu.Lock()
	defer mu.Unlock()

	currentLang = lang
	switch lang {
	case LangZH:
		messages = &messagesZH
	default:
		messages = &messagesEN
	}
}

// GetLanguage returns the current language
func GetLanguage() Language {
	mu.RLock()
	defer mu.RUnlock()
	return currentLang
}

// M returns the current messages
func M() *Messages {
	mu.RLock()
	defer mu.RUnlock()
	return messages
}

// Get returns specific message by key dynamically using reflection
func Get(key string) string {
	msg := M()
	v := reflect.ValueOf(msg).Elem()
	f := v.FieldByName(key)
	if f.IsValid() && f.Kind() == reflect.String {
		return f.String()
	}
	return key
}
