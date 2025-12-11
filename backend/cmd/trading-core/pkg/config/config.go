package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds environment-driven settings for the trading core.
type Config struct {
	Port string

	// Binance
	BinanceTestnet       bool
	BinanceAPIKey        string
	BinanceAPISecret     string
	BinanceSymbols       []string
	UseMockFeed          bool
	EnableBinanceTrading bool
	// Binance Futures (USDT)
	EnableBinanceUSDTFutures bool
	BinanceUSDTKey           string
	BinanceUSDTSecret        string
	// Binance Futures (Coin-M)
	EnableBinanceCoinFutures bool
	BinanceCoinKey           string
	BinanceCoinSecret        string

	// Python worker
	EnablePythonWorker bool
	PythonWorkerAddr   string

	// Execution
	DryRun bool

	// Dry-run simulation
	DryRunInitialBalance float64
	DryRunDBPath         string
	DryRunEnableOrderWAL bool
	DryRunOrderWALPath   string
	DryRunFeeRate        float64 // decimal (e.g. 0.0004 = 4 bps)
	DryRunSlippageBps    float64 // slippage applied on fills (bps)
	DryRunGwLatencyMinMs int     // simulated gateway latency lower bound
	DryRunGwLatencyMaxMs int     // simulated gateway latency upper bound

	// Order persistence
	EnableOrderWAL bool
	OrderWALPath   string

	// Database
	DBPath string

	// Execution toggle and balance source
	ExecutionEnabled bool
	BalanceSource    string // "auto" (default), "exchange", "fixed"

	// Auth / licensing
	JWTSecret     string
	LicenseServer string

	// Localization
	Language string // "en" or "zh"
}

// Load reads environment variables (optionally via .env) into Config.
func Load() (*Config, error) {
	// Ignore error so the app still starts when .env is missing.
	_ = godotenv.Load()

	// Database path: prefer DB_PATH, then DATABASE_PATH for backward compatibility.
	dbPath := getEnv("DB_PATH", "")
	if dbPath == "" {
		dbPath = getEnv("DATABASE_PATH", "./data/trading.db")
	}

	return &Config{
		Port:                     getEnv("PORT", "8080"),
		BinanceTestnet:           getEnv("BINANCE_TESTNET", "false") == "true",
		BinanceAPIKey:            os.Getenv("BINANCE_API_KEY"),
		BinanceAPISecret:         os.Getenv("BINANCE_API_SECRET"),
		BinanceSymbols:           splitAndTrim(getEnv("BINANCE_SYMBOLS", "BTCUSDT,ETHUSDT")),
		UseMockFeed:              getEnv("USE_MOCK_FEED", "true") == "true",
		EnableBinanceTrading:     getEnv("ENABLE_BINANCE_TRADING", "false") == "true",
		EnableBinanceUSDTFutures: getEnv("ENABLE_BINANCE_USDT_FUTURES", "false") == "true",
		BinanceUSDTKey:           os.Getenv("BINANCE_USDT_KEY"),
		BinanceUSDTSecret:        os.Getenv("BINANCE_USDT_SECRET"),
		EnableBinanceCoinFutures: getEnv("ENABLE_BINANCE_COIN_FUTURES", "false") == "true",
		BinanceCoinKey:           os.Getenv("BINANCE_COIN_KEY"),
		BinanceCoinSecret:        os.Getenv("BINANCE_COIN_SECRET"),
		EnablePythonWorker:       getEnv("ENABLE_PYTHON_WORKER", "false") == "true",
		PythonWorkerAddr:         getEnv("PYTHON_WORKER_ADDR", "localhost:50051"),
		DryRun:                   getEnv("DRY_RUN", "false") == "true",
		DryRunInitialBalance:     getEnvFloat("DRY_RUN_INITIAL_BALANCE", 10000.0),
		DryRunDBPath:             getEnv("DRY_RUN_DB_PATH", "./trading_dry.db"),
		DryRunEnableOrderWAL:     getEnv("DRY_RUN_ENABLE_ORDER_WAL", "false") == "true",
		DryRunOrderWALPath:       getEnv("DRY_RUN_ORDER_WAL_PATH", "./data/order_wal_dry"),
		DryRunFeeRate:            getEnvFloat("DRY_RUN_FEE_RATE", 0.0004),
		DryRunSlippageBps:        getEnvFloat("DRY_RUN_SLIPPAGE_BPS", 2),
		DryRunGwLatencyMinMs:     getEnvInt("DRY_RUN_GATEWAY_LATENCY_MIN_MS", 0),
		DryRunGwLatencyMaxMs:     getEnvInt("DRY_RUN_GATEWAY_LATENCY_MAX_MS", 0),
		EnableOrderWAL:           getEnv("ENABLE_ORDER_WAL", "true") == "true",
		OrderWALPath:             getEnv("ORDER_WAL_PATH", "./data/order_wal"),
		DBPath:                   dbPath,
		JWTSecret:                getEnv("JWT_SECRET", "dev-secret"),
		LicenseServer:            getEnv("LICENSE_SERVER", ""),
		Language:                 getEnv("LANGUAGE", "en"),
		ExecutionEnabled:         getEnv("EXECUTION_ENABLED", "true") == "true",
		BalanceSource:            strings.ToLower(getEnv("BALANCE_SOURCE", "auto")),
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func splitAndTrim(val string) []string {
	parts := strings.Split(val, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func getEnvFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}
