package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"

	"trading-core/internal/api"
	"trading-core/internal/balance"
	"trading-core/internal/events"
	"trading-core/internal/indicators"
	"trading-core/internal/market"
	"trading-core/internal/monitor"
	"trading-core/internal/order"
	"trading-core/internal/reconciliation"
	"trading-core/internal/risk"
	"trading-core/internal/state"
	"trading-core/internal/strategy"
	"trading-core/pkg/binance"
	"trading-core/pkg/config"
	"trading-core/pkg/db"
	exfutcoin "trading-core/pkg/exchanges/binance/futures_coin"
	exfutusdt "trading-core/pkg/exchanges/binance/futures_usdt"
	exspot "trading-core/pkg/exchanges/binance/spot"
	exchange "trading-core/pkg/exchanges/common"
	marketbinance "trading-core/pkg/market/binance"
)

type priceCache struct {
	mu sync.RWMutex
	m  map[string]float64
}

type exposureCache struct {
	mu  sync.RWMutex
	val float64
	ts  time.Time
	ttl time.Duration
}

func (p *priceCache) set(sym string, price float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.m == nil {
		p.m = make(map[string]float64)
	}
	p.m[sym] = price
}

func (p *priceCache) get(sym string) float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.m[sym]
}

func (e *exposureCache) get(compute func() float64) float64 {
	e.mu.RLock()
	if time.Since(e.ts) < e.ttl && e.ttl > 0 {
		val := e.val
		e.mu.RUnlock()
		return val
	}
	e.mu.RUnlock()

	// Recompute
	val := compute()
	e.mu.Lock()
	e.val = val
	e.ts = time.Now()
	e.mu.Unlock()
	return val
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("Starting DES Trading System...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("failed to load config:", err)
	}
	log.Printf("Config loaded (Port: %s)", cfg.Port)

	dbPath := cfg.DBPath
	if cfg.DryRun && cfg.DryRunDBPath != "" {
		dbPath = cfg.DryRunDBPath
	}
	log.Printf("Using DB path: %s", dbPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Core services
	bus := events.NewBus()

	database, err := db.New(dbPath)
	if err != nil {
		log.Fatal("failed to init database:", err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(database); err != nil {
		log.Fatal("failed to apply migrations:", err)
	}

	// In-memory state seeded from DB
	stateMgr := state.NewManager(database)
	if err := stateMgr.Load(ctx); err != nil {
		log.Fatal("failed to load state:", err)
	}

	indEngine := indicators.NewEngine(7, 25, 14, 200)

	// Risk managers
	riskMgr, err := risk.NewManager(database.DB)
	if err != nil {
		log.Printf("risk manager init failed, fallback to default in-memory: %v", err)
		riskMgr = risk.NewInMemory(risk.DefaultConfig())
	}
	stopLossMgr := risk.NewStopLossManager()
	priceCache := &priceCache{m: make(map[string]float64)}
	expCache := &exposureCache{ttl: 1 * time.Second}

	// Exchange gateway selection
	var gateway exchange.Gateway
	venue := "none"
	buildVersion := os.Getenv("APP_VERSION")
	if buildVersion == "" {
		buildVersion = "v2.0-dev"
	}
	switch {
	case cfg.EnableBinanceTrading:
		venue = "binance-spot"
		gateway = exspot.New(exspot.Config{
			APIKey:    cfg.BinanceAPIKey,
			APISecret: cfg.BinanceAPISecret,
			Testnet:   false,
		})
	case cfg.EnableBinanceUSDTFutures:
		venue = "binance-usdtfut"
		gateway = exfutusdt.NewClient(exfutusdt.Config{
			APIKey:    cfg.BinanceUSDTKey,
			APISecret: cfg.BinanceUSDTSecret,
			Testnet:   false,
		})
	case cfg.EnableBinanceCoinFutures:
		venue = "binance-coinfut"
		gateway = exfutcoin.NewClient(exfutcoin.Config{
			APIKey:    cfg.BinanceCoinKey,
			APISecret: cfg.BinanceCoinSecret,
			Testnet:   false,
		})
	}

	// Balance manager with exchange integration
	var balanceMgr *balance.Manager
	if cfg.DryRun {
		// Dry-run mode: no exchange client needed
		balanceMgr = balance.NewManager(nil, 30*time.Second)
		balanceMgr.SetInitialBalance(cfg.DryRunInitialBalance)
		log.Printf("💰 Dry-run balance initialized: %.2f", cfg.DryRunInitialBalance)
	} else {
		// Production mode: Try to use gateway if it implements balance.ExchangeClient
		if balClient, ok := gateway.(balance.ExchangeClient); ok {
			balanceMgr = balance.NewManager(balClient, 30*time.Second)
			balanceMgr.Start(ctx)
			log.Println("✓ Balance manager started with exchange sync")
		} else {
			// Fallback: no balance API support
			balanceMgr = balance.NewManager(nil, 30*time.Second)
			balanceMgr.SetInitialBalance(10000.0)
			log.Println("⚠ Balance manager: gateway doesn't support GetBalance, using default")
		}
	}

	// Order flow with dry-run wrapper
	orderQueue := order.NewQueue(200)
	exec := order.NewExecutor(database, bus, gateway, venue, cfg.BinanceTestnet)
	mode := order.ModeProduction
	if cfg.DryRun {
		mode = order.ModeDryRun
		log.Println("⚠ Running in DRY-RUN mode (orders will NOT hit exchange)")
	}
	dryRunner := order.NewDryRunExecutor(mode, exec, cfg.DryRunInitialBalance)
	asyncExec := order.NewAsyncExecutorWithDryRun(dryRunner, 4) // V2 P0-B: Async Execution

	// System metrics for monitoring
	sysMetrics := monitor.NewSystemMetrics()
	log.Println("✓ System metrics initialized")

	// Reconciliation service (only in production mode)
	if !cfg.DryRun {
		if reconClient, ok := gateway.(reconciliation.ExchangeClient); ok {
			reconService := reconciliation.NewService(reconClient, stateMgr, database, 5*time.Minute)
			reconService.Start(ctx)
			log.Println("✓ Reconciliation service started")
		} else {
			log.Println("⚠ Reconciliation: gateway doesn't support GetPositions")
		}
	}

	// Market data (mock first, real later)
	binanceClient := binance.NewClient(cfg.BinanceAPIKey, cfg.BinanceAPISecret, false)
	streamClient := binance.NewStreamClient(false)
	if cfg.UseMockFeed {
		mock := market.MockFeed{
			Bus:        bus,
			Symbols:    cfg.BinanceSymbols,
			StartPrice: 100,
			Step:       0.8,
			Interval:   time.Second,
		}
		mock.Start(ctx)
		log.Println("✓ Mock feed started")
	} else {
		feed := market.Feed{
			Client:   binanceClient,
			Stream:   streamClient,
			Bus:      bus,
			Symbols:  cfg.BinanceSymbols,
			Interval: "1m",
		}
		feed.Start(ctx)
		log.Println("✓ Binance feed started")
	}

	// Price cache subscriber (for risk pricing + trailing stop + auto-close)
	priceSub, unsubPrice := bus.Subscribe(events.EventPriceTick, 100)
	defer unsubPrice()
	filledSub, unsubFilled := bus.Subscribe(events.EventOrderFilled, 100)
	defer unsubFilled()

	// Helper function to handle stop loss trigger
	handleStopLossTrigger := func(symbol string, decision *risk.StopLossDecision) {
		pos := stateMgr.Position(symbol)
		qty := math.Abs(pos.Qty)
		if qty > 0 {
			closeSide := oppositeSide(sideFromQty(pos.Qty))
			orderQueue.Enqueue(order.Order{
				ID:        uuid.NewString(),
				Symbol:    symbol,
				Side:      closeSide,
				Type:      "MARKET",
				Qty:       qty,
				Status:    "NEW",
				CreatedAt: time.Now(),
				Market:    marketFromVenue(venue),
			})
			log.Printf("🛑 Stop Loss Triggered: %s %s %.4f - %s", symbol, closeSide, qty, decision.Reason)
		}
	}

	go func() {
		for msg := range priceSub {
			var symbol string
			var price float64

			switch v := msg.(type) {
			case marketbinance.Kline:
				symbol, price = v.Symbol, v.Close
			case struct {
				Symbol string
				Close  float64
			}:
				symbol, price = v.Symbol, v.Close
			default:
				continue
			}

			if symbol == "" {
				continue
			}

			priceCache.set(symbol, price)

			// Check stop loss trigger
			if decision := stopLossMgr.UpdatePrice(symbol, price); decision != nil && decision.Triggered {
				handleStopLossTrigger(symbol, decision)
			}
		}
	}()

	// Filled orders -> update positions and risk metrics (price fallback to latest cache)
	go func() {
		for msg := range filledSub {
			var (
				symbol string
				side   string
				qty    float64
				price  float64
			)
			switch v := msg.(type) {
			case order.Order:
				symbol, side, qty, price = v.Symbol, v.Side, v.Qty, v.Price
			case struct {
				ID     string
				Symbol string
				Side   string
				Qty    float64
				Price  float64
			}:
				symbol, side, qty, price = v.Symbol, v.Side, v.Qty, v.Price
			default:
				log.Printf("⚠️ Unknown filled order type: %T", msg)
				continue
			}

			fillPrice := price
			if fillPrice == 0 {
				if p := priceCache.get(symbol); p > 0 {
					fillPrice = p
					log.Printf("⚠️ Using cached price for %s: %.2f", symbol, fillPrice)
				}
			}
			if fillPrice == 0 {
				fillPrice = 1 // last-resort guard to avoid zero
				log.Printf("⚠️ Warning: fillPrice is 0 for %s, using fallback", symbol)
			}

			// Snapshot previous position for realized PnL
			prev := stateMgr.Position(symbol)

			// Update in-memory + DB position
			_, _ = stateMgr.RecordFill(ctx, symbol, side, qty, fillPrice)

			// Get updated position for cleanup check
			newPos := stateMgr.Position(symbol)

			// Compute simple realized PnL on closing quantity
			var pnl float64
			closeQty := math.Min(math.Abs(prev.Qty), qty)
			if closeQty > 0 {
				switch {
				case prev.Qty > 0 && strings.ToUpper(side) == "SELL":
					pnl = (fillPrice - prev.AvgPrice) * closeQty
				case prev.Qty < 0 && strings.ToUpper(side) == "BUY":
					pnl = (prev.AvgPrice - fillPrice) * closeQty
				}
				log.Printf("💰 Realized PnL: %.2f (%s %s %.4f @ %.2f)", pnl, symbol, side, closeQty, fillPrice)
			} else {
				log.Printf("📈 Opening/Adding position: %s %s %.4f @ %.2f", symbol, side, qty, fillPrice)
			}

			// Lookup fee for this order (best-effort; default 0 if not found)
			var fee float64
			switch v := msg.(type) {
			case order.Order:
				row := database.DB.QueryRowContext(ctx,
					"SELECT COALESCE(SUM(fee),0) FROM trades WHERE order_id = ?", v.ID)
				_ = row.Scan(&fee)
			case struct {
				ID     string
				Symbol string
				Side   string
				Qty    float64
				Price  float64
			}:
				row := database.DB.QueryRowContext(ctx,
					"SELECT COALESCE(SUM(fee),0) FROM trades WHERE order_id = ?", v.ID)
				_ = row.Scan(&fee)
			}
			netPnL := pnl - fee

			// Update risk metrics with net PnL
			if err := riskMgr.UpdateMetrics(risk.TradeResult{
				Symbol: symbol,
				Side:   side,
				Size:   qty,
				Price:  fillPrice,
				PnL:    netPnL,
				Fee:    fee,
			}); err != nil {
				log.Printf("❌ Failed to update risk metrics: %v", err)
			}

			// Handle balance updates based on trade side
			orderValue := qty * fillPrice
			if strings.ToUpper(side) == "BUY" {
				// Buy order - deduct locked balance
				balanceMgr.Deduct(orderValue)
			} else if strings.ToUpper(side) == "SELL" {
				// Sell order - add proceeds (unlock was already done if partial fill)
				balanceMgr.Add(orderValue)
			}

			// Clean up stop loss tracking if position is closed
			if math.Abs(newPos.Qty) < 0.0001 {
				stopLossMgr.RemovePosition(symbol)
				log.Printf("🔓 Position closed, stop loss removed: %s", symbol)
			} else {
				log.Printf("📊 Position updated: %s %.4f @ %.2f", symbol, newPos.Qty, newPos.AvgPrice)
			}
		}
	}()

	// Strategies
	priceStream, unsubscribe := bus.Subscribe(events.EventPriceTick, 100)
	defer unsubscribe()
	stratEngine := strategy.NewEngine(bus, database.DB, strategy.Context{Indicators: indEngine})

	// Load strategies from YAML config and sync to DB
	stratConfigs, err := strategy.LoadConfig("strategies.yaml")
	if err != nil {
		log.Printf("⚠️ Failed to load strategies.yaml: %v", err)
	} else {
		if err := strategy.SyncConfigToDB(database.DB, stratConfigs); err != nil {
			log.Printf("❌ Failed to sync strategies to DB: %v", err)
		} else {
			log.Println("✓ Strategies synced to DB")
		}
	}

	// Load active strategies from DB
	if err := stratEngine.LoadStrategies(database.DB); err != nil {
		log.Printf("❌ Failed to load strategies from DB: %v", err)
	}

	// Optional: delegate to Python worker via gRPC
	var pyClient *strategy.WorkerClient
	if cfg.EnablePythonWorker {
		c, err := strategy.NewWorkerClient(cfg.PythonWorkerAddr)
		if err != nil {
			log.Printf("python worker client init failed: %v", err)
		} else {
			pyClient = c
			// Python strategy loading might need similar DB logic in future
			// For now, keep it as is or adapt if needed.
			// stratEngine.Add(strategy.NewPythonStrategy("python_worker", pyClient))
			log.Printf("✓ Python worker enabled at %s", cfg.PythonWorkerAddr)
		}
	}
	stratEngine.Start(ctx, priceStream)
	defer func() {
		if pyClient != nil {
			_ = pyClient.Close()
		}
	}()

	sigStream, unsubSig := bus.Subscribe(events.EventStrategySignal, 100)
	defer unsubSig()
	go func() {
		for msg := range sigStream {
			sig, ok := msg.(strategy.Signal)
			if !ok {
				continue
			}

			// Gather context for risk decision
			price := priceCache.get(sig.Symbol)
			pos := stateMgr.Position(sig.Symbol)
			position := risk.Position{
				Symbol:        pos.Symbol,
				Side:          sideFromQty(pos.Qty),
				EntryPrice:    pos.AvgPrice,
				CurrentPrice:  price,
				Quantity:      pos.Qty,
				Value:         pos.Qty * price,
				UnrealizedPnL: (price - pos.AvgPrice) * pos.Qty,
			}
			// Build account snapshot for risk evaluation
			balSnap := balanceMgr.GetBalance()
			totalExposure := expCache.get(func() float64 {
				sum := 0.0
				for _, p := range stateMgr.Positions() {
					px := priceCache.get(p.Symbol)
					sum += math.Abs(p.Qty * px)
				}
				return sum
			})
			account := risk.Account{
				Balance:          balSnap.Total,
				AvailableBalance: balSnap.Available,
				LockedBalance:    balSnap.Locked,
				TotalExposure:    totalExposure,
			}

			// Calculate order value and lock balance
			orderValue := sig.Size * price
			if err := balanceMgr.Lock(orderValue); err != nil {
				log.Printf("❌ Balance lock failed: %v", err)
				bus.Publish(events.EventRiskAlert, fmt.Sprintf("Insufficient balance: %v", err))
				continue
			}

			decision := riskMgr.EvaluateSignal(
				risk.SignalInput{
					Symbol: sig.Symbol,
					Action: sig.Action,
					Size:   sig.Size,
					Price:  price,
				},
				position,
				account,
			)
			if !decision.Allowed {
				balanceMgr.Unlock(orderValue) // Release locked balance
				bus.Publish(events.EventRiskAlert, decision.Reason)
				continue
			}

			size := decision.AdjustedSize
			if size == 0 {
				size = sig.Size
			}

			// Adjust locked amount if size changed
			adjustedValue := size * price
			if adjustedValue < orderValue {
				balanceMgr.Unlock(orderValue - adjustedValue)
			}

			// Register SL/TP for trailing logic (does not auto-place orders)
			cfgCopy := riskMgr.GetConfig()
			stopLossMgr.AddPosition(risk.StopLossPosition{
				Symbol:         sig.Symbol,
				Side:           sideFromAction(sig.Action),
				EntryPrice:     price,
				CurrentPrice:   price,
				StopLoss:       decision.StopLoss,
				TakeProfit:     decision.TakeProfit,
				TrailingStop:   cfgCopy.UseTrailingStop,
				TrailingOffset: cfgCopy.TrailingPercent,
			})

			// Create order with locked balance
			o := order.Order{
				ID:                 uuid.NewString(),
				StrategyInstanceID: sig.StrategyID,
				Symbol:             sig.Symbol,
				Side:               sig.Action,
				Type:               "MARKET",
				Qty:                size,
				Status:             "NEW",
				CreatedAt:          time.Now(),
				Market:             marketFromVenue(venue),
				StopPrice:          decision.StopLoss,
				ActivationPrice:    decision.TakeProfit,
			}
			orderQueue.Enqueue(o)
		}
	}()

	go orderQueue.Drain(ctx, func(o order.Order) {
		asyncExec.ExecuteAsync(ctx, o) // V2 P0-B: Async Execution
	})

	// Monitor async execution results (V2 P0-B)
	go func() {
		for result := range asyncExec.Results() {
			if !result.Success {
				log.Printf("❌ Async execution failed for order %s: %v", result.OrderID, result.Error)
				sysMetrics.IncrementErrors()
			} else {
				sysMetrics.IncrementOrders()
			}
			sysMetrics.OrderLatency.RecordDuration(result.Latency)
		}
	}()

	// Start Spot User Data Stream (only when using spot gateway)
	if cfg.EnableBinanceTrading && cfg.BinanceAPIKey != "" && cfg.BinanceAPISecret != "" && !cfg.DryRun {
		spotStream := order.NewSpotUserStream(exspot.New(exspot.Config{
			APIKey:    cfg.BinanceAPIKey,
			APISecret: cfg.BinanceAPISecret,
			Testnet:   cfg.BinanceTestnet,
		}), database, bus, cfg.BinanceTestnet)
		spotStream.Start(ctx)
	}
	// Start Futures User Data Stream (USDT)
	if cfg.EnableBinanceUSDTFutures && cfg.BinanceUSDTKey != "" && cfg.BinanceUSDTSecret != "" && !cfg.DryRun {
		usdtStream := order.NewFuturesUserStream(exfutusdt.NewClient(exfutusdt.Config{
			APIKey:    cfg.BinanceUSDTKey,
			APISecret: cfg.BinanceUSDTSecret,
			Testnet:   cfg.BinanceTestnet,
		}), database, bus, cfg.BinanceTestnet, false)
		usdtStream.Start(ctx)
	}
	// Start Futures User Data Stream (COIN)
	if cfg.EnableBinanceCoinFutures && cfg.BinanceCoinKey != "" && cfg.BinanceCoinSecret != "" && !cfg.DryRun {
		coinStream := order.NewFuturesUserStream(exfutcoin.NewClient(exfutcoin.Config{
			APIKey:    cfg.BinanceCoinKey,
			APISecret: cfg.BinanceCoinSecret,
			Testnet:   cfg.BinanceTestnet,
		}), database, bus, cfg.BinanceTestnet, true)
		coinStream.Start(ctx)
	}

	// API
	server := api.NewServer(
		bus,
		database,
		riskMgr,
		balanceMgr,
		stratEngine,
		sysMetrics,
		orderQueue,
		api.SystemMeta{
			DryRun:      cfg.DryRun,
			Venue:       venue,
			Symbols:     cfg.BinanceSymbols,
			UseMockFeed: cfg.UseMockFeed,
			Version:     buildVersion,
		},
		cfg.JWTSecret,
	)
	go func() {
		if err := server.Start(":" + cfg.Port); err != nil {
			log.Fatalf("api server error: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	log.Println("Shutting down...")
}

func sideFromQty(qty float64) string {
	if qty > 0 {
		return "LONG"
	}
	if qty < 0 {
		return "SHORT"
	}
	return ""
}

func sideFromAction(action string) string {
	if strings.ToUpper(action) == "BUY" {
		return "LONG"
	}
	if strings.ToUpper(action) == "SELL" {
		return "SHORT"
	}
	return ""
}

func marketFromVenue(venue string) string {
	switch venue {
	case "binance-spot":
		return string(exchange.MarketSpot)
	case "binance-usdtfut":
		return string(exchange.MarketUSDTFut)
	case "binance-coinfut":
		return string(exchange.MarketCoinFut)
	default:
		return ""
	}
}

// oppositeSide returns SELL for BUY and BUY for SELL.
func oppositeSide(side string) string {
	switch strings.ToUpper(side) {
	case "BUY":
		return "SELL"
	case "SELL":
		return "BUY"
	default:
		return ""
	}
}
