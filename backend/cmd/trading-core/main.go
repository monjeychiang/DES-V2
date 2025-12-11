package main

import (
	"context"
	"database/sql"
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
	"trading-core/internal/engine"
	"trading-core/internal/events"
	"trading-core/internal/gateway"
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
	"trading-core/pkg/crypto"
	"trading-core/pkg/db"
	exfutcoin "trading-core/pkg/exchanges/binance/futures_coin"
	exfutusdt "trading-core/pkg/exchanges/binance/futures_usdt"
	exspot "trading-core/pkg/exchanges/binance/spot"
	exchange "trading-core/pkg/exchanges/common"
	"trading-core/pkg/i18n"
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

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf(i18n.Get("ConfigLoadFailed"), err)
	}

	i18n.SetLanguage(i18n.Language(cfg.Language))
	log.Println(i18n.Get("Starting"))

	dbPath := cfg.DBPath
	if cfg.DryRun && cfg.DryRunDBPath != "" {
		dbPath = cfg.DryRunDBPath
	}
	log.Printf(i18n.Get("ConfigLoaded"), cfg.Port)
	log.Printf(i18n.Get("UsingDBPath"), dbPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Core services
	bus := events.NewBus()

	database, err := db.New(dbPath)
	if err != nil {
		log.Fatalf(i18n.Get("DBInitFailed"), err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(database); err != nil {
		log.Fatalf(i18n.Get("DBMigrationsFailed"), err)
	}

	// In-memory state seeded from DB
	stateMgr := state.NewManager(database)
	if err := stateMgr.Load(ctx); err != nil {
		log.Fatalf(i18n.Get("StateLoadFailed"), err)
	}

	indEngine := indicators.NewEngine(7, 25, 14, 200)

	// Risk managers
	riskMgr, err := risk.NewManager(database.DB)
	if err != nil {
		log.Printf(i18n.Get("RiskManagerInitFailed"), err)
		riskMgr = risk.NewInMemory(risk.DefaultConfig())
	}
	cfgCopy := riskMgr.GetConfig()
	log.Printf(i18n.Get("RiskManagerInit"), cfgCopy.DefaultStopLoss*100, cfgCopy.DefaultTakeProfit*100)
	stopLossMgr := risk.NewStopLossManager()
	priceCache := &priceCache{m: make(map[string]float64)}
	expCache := &exposureCache{ttl: 1 * time.Second}

	// Multi-user: Key Manager (for encrypted API keys)
	var keyMgr *crypto.KeyManager
	if os.Getenv("MASTER_ENCRYPTION_KEY") != "" {
		keyMgr, err = crypto.NewKeyManager()
		if err != nil {
			log.Printf("⚠️ KeyManager init failed: %v (encryption disabled)", err)
		} else {
			log.Printf("🔐 KeyManager initialized (version %d)", keyMgr.CurrentVersion())
		}
	}

	// Multi-user: Gateway Manager (per-connection gateways)
	var gatewayMgr *gateway.Manager
	if keyMgr != nil {
		gatewayMgr = gateway.NewManager(
			database.Queries(),
			keyMgr,
			gateway.DefaultFactory,
			gateway.DefaultConfig(),
		)
		gatewayMgr.Start(ctx)
		log.Println("🌐 GatewayManager started (multi-user mode)")
	}

	// Multi-user: per-user risk manager
	multiUserRisk := risk.NewMultiUserManager(database.DB)

	// Exchange gateway selection (fallback for single-user mode)
	var exchGateway exchange.Gateway
	venue := "none"
	buildVersion := os.Getenv("APP_VERSION")
	if buildVersion == "" {
		buildVersion = "v2.0-dev"
	}
	switch {
	case cfg.EnableBinanceTrading:
		venue = "binance-spot"
		exchGateway = exspot.New(exspot.Config{
			APIKey:    cfg.BinanceAPIKey,
			APISecret: cfg.BinanceAPISecret,
			Testnet:   false,
		})
	case cfg.EnableBinanceUSDTFutures:
		venue = "binance-usdtfut"
		exchGateway = exfutusdt.NewClient(exfutusdt.Config{
			APIKey:    cfg.BinanceUSDTKey,
			APISecret: cfg.BinanceUSDTSecret,
			Testnet:   false,
		})
	case cfg.EnableBinanceCoinFutures:
		venue = "binance-coinfut"
		exchGateway = exfutcoin.NewClient(exfutcoin.Config{
			APIKey:    cfg.BinanceCoinKey,
			APISecret: cfg.BinanceCoinSecret,
			Testnet:   false,
		})
	}

	// Balance manager with exchange integration (global account)
	var balanceMgr *balance.Manager
	useFixedBalance := cfg.DryRun || strings.EqualFold(cfg.BalanceSource, "fixed")
	if useFixedBalance {
		balanceMgr = balance.NewManager(nil, 30*time.Second)
		initial := cfg.DryRunInitialBalance
		if initial <= 0 {
			initial = 10000.0
		}
		balanceMgr.SetInitialBalance(initial)
		log.Printf(i18n.Get("BalanceInitialized"), initial)
	} else {
		// Try to use exchGateway if it implements balance.ExchangeClient
		if balClient, ok := exchGateway.(balance.ExchangeClient); ok {
			balanceMgr = balance.NewManager(balClient, 30*time.Second)
			balanceMgr.Start(ctx)
			log.Println(i18n.Get("BalanceManagerStarted"))
		} else {
			// Fallback: no balance API support (simulate with fixed initial balance)
			balanceMgr = balance.NewManager(nil, 30*time.Second)
			balanceMgr.SetInitialBalance(10000.0)
			log.Println(i18n.Get("BalanceManagerFallback"))
		}
	}

	// Multi-user balance manager: per-user in-memory balances (primarily for risk control).
	userBalanceMgr := balance.NewMultiUserManager(func(userID string) (*balance.Manager, error) {
		mgr := balance.NewManager(nil, 30*time.Second)
		initial := cfg.DryRunInitialBalance
		if initial <= 0 {
			initial = 10000.0
		}
		mgr.SetInitialBalance(initial)
		log.Printf("Multi-user balance manager created for user %s with initial balance %.2f", userID, initial)
		return mgr, nil
	})

	// Background cleanup for per-user managers to avoid unbounded growth.
	perUserIdleTTL := 60 * time.Minute
	cleanupInterval := 10 * time.Minute
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if multiUserRisk != nil {
					multiUserRisk.CleanupIdle(perUserIdleTTL)
				}
				if userBalanceMgr != nil {
					userBalanceMgr.CleanupIdle(perUserIdleTTL)
				}
			}
		}
	}()

	// Order flow with dry-run wrapper
	var orderQueue order.OrderQueue
	enableWal := cfg.EnableOrderWAL && (!cfg.DryRun || cfg.DryRunEnableOrderWAL)
	walPath := cfg.OrderWALPath
	if cfg.DryRun && cfg.DryRunEnableOrderWAL {
		walPath = cfg.DryRunOrderWALPath
	}
	if enableWal {
		pq, err := order.NewPersistentQueue(walPath, 200)
		if err != nil {
			log.Printf(i18n.Get("PersistentQueueFailed"), err)
			orderQueue = order.NewQueue(200)
		} else {
			if err := pq.Recover(); err != nil {
				log.Printf(i18n.Get("WalRecoveryError"), err)
			}
			orderQueue = pq
			log.Printf(i18n.Get("OrderWalEnabled"), walPath)
		}
	} else {
		orderQueue = order.NewQueue(200)
	}
	exec := order.NewExecutor(database, bus, exchGateway, venue, cfg.BinanceTestnet)
	mode := order.ModeProduction
	if cfg.DryRun || !cfg.ExecutionEnabled {
		mode = order.ModeDryRun
		log.Println(i18n.Get("DryRunMode"))
	}
	dryRunner := order.NewDryRunExecutor(mode, exec, cfg.DryRunInitialBalance, order.DryRunSimConfig{
		FeeRate:             cfg.DryRunFeeRate,
		SlippageBps:         cfg.DryRunSlippageBps,
		GatewayLatencyMinMs: cfg.DryRunGwLatencyMinMs,
		GatewayLatencyMaxMs: cfg.DryRunGwLatencyMaxMs,
	})
	asyncExec := order.NewAsyncExecutorWithDryRun(dryRunner, 4) // V2 P0-B: Async Execution

	// Multi-user: inject KeyManager and Gateway pool
	if keyMgr != nil {
		exec.SetKeyManager(keyMgr)
		if gatewayMgr != nil {
			exec.SetGatewayPool(gatewayMgr)
		}
		log.Println("KeyManager injected into Executor")
	}

	// System metrics for monitoring
	sysMetrics := monitor.NewSystemMetrics()
	exec.SetMetrics(sysMetrics)
	log.Println(i18n.Get("SystemMetricsInit"))

	// Periodically update metrics with gateway pool & multi-user stats.
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if gatewayMgr != nil {
					sysMetrics.SetGatewayPoolStats(gatewayMgr.Stats())
				}
				if multiUserRisk != nil || userBalanceMgr != nil {
					riskUsers := 0
					balanceUsers := 0
					if multiUserRisk != nil {
						riskUsers = multiUserRisk.UserCount()
					}
					if userBalanceMgr != nil {
						balanceUsers = userBalanceMgr.UserCount()
					}
					sysMetrics.SetMultiUserCounts(riskUsers, balanceUsers)
				}
			}
		}
	}()

	// Reconciliation service (only in production mode)
	if !cfg.DryRun {
		if reconClient, ok := exchGateway.(reconciliation.ExchangeClient); ok {
			reconService := reconciliation.NewService(reconClient, stateMgr, database, 5*time.Minute)
			reconService.Start(ctx)
			log.Println(i18n.Get("ReconStarted"))
		} else {
			log.Println(i18n.Get("ReconNotSupported"))
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
		log.Println(i18n.Get("MockFeedStarted"))
	} else {
		feed := market.Feed{
			Client:   binanceClient,
			Stream:   streamClient,
			Bus:      bus,
			Symbols:  cfg.BinanceSymbols,
			Interval: "1m",
		}
		feed.Start(ctx)
		log.Println(i18n.Get("BinanceFeedStarted"))
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
			log.Printf(i18n.Get("StopLossTriggered"), symbol, closeSide, qty, decision.Reason)
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
				userID string
			)
			switch v := msg.(type) {
			case order.Order:
				symbol, side, qty, price = v.Symbol, v.Side, v.Qty, v.Price
				userID = v.UserID
			case struct {
				ID     string
				Symbol string
				Side   string
				Qty    float64
				Price  float64
			}:
				symbol, side, qty, price = v.Symbol, v.Side, v.Qty, v.Price
			default:
				log.Printf(i18n.Get("UnknownFilledOrderType"), msg)
				continue
			}

			fillPrice := price
			if fillPrice == 0 {
				if p := priceCache.get(symbol); p > 0 {
					fillPrice = p
					log.Printf(i18n.Get("UsingCachedPrice"), symbol, fillPrice)
				}
			}
			if fillPrice == 0 {
				fillPrice = 1 // last-resort guard to avoid zero
				log.Printf(i18n.Get("FillPriceZeroFallback"), symbol)
			}

			// Snapshot previous position for realized PnL
			prev := stateMgr.Position(symbol)

			// Update in-memory + DB position
			_, _ = stateMgr.RecordFill(ctx, userID, symbol, side, qty, fillPrice)

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
				log.Printf(i18n.Get("RealizedPnL"), pnl, symbol, side, closeQty, fillPrice)
			} else {
				log.Printf(i18n.Get("PositionOpened"), symbol, side, qty, fillPrice)
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
				log.Printf(i18n.Get("RiskMetricsUpdateFailed"), err)
			}

			// Handle balance updates based on trade side (per-user when possible)
			orderValue := qty * fillPrice
			balTarget := balanceMgr
			if userID != "" && userBalanceMgr != nil {
				if userBalMgr, err := userBalanceMgr.GetOrCreate(userID); err == nil {
					balTarget = userBalMgr
				} else {
					log.Printf("per-user balance manager init failed for user %s (fill): %v - using global balance", userID, err)
				}
			}
			if strings.ToUpper(side) == "BUY" {
				// Buy order - deduct locked balance
				balTarget.Deduct(orderValue)
			} else if strings.ToUpper(side) == "SELL" {
				// Sell order - add proceeds (unlock was already done if partial fill)
				balTarget.Add(orderValue)
			}

			// Clean up stop loss tracking if position is closed
			if math.Abs(newPos.Qty) < 0.0001 {
				stopLossMgr.RemovePosition(symbol)
				log.Printf(i18n.Get("PositionClosed"), symbol)
			} else {
				log.Printf(i18n.Get("PositionUpdated"), symbol, newPos.Qty, newPos.AvgPrice)
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
		log.Printf(i18n.Get("StrategyConfigLoadFailed"), err)
	} else {
		if err := strategy.SyncConfigToDB(database.DB, stratConfigs); err != nil {
			log.Printf(i18n.Get("StrategySyncFailed"), err)
		} else {
			log.Println(i18n.Get("StrategySaveComplete"))
		}
	}

	// Load active strategies from DB
	if err := stratEngine.LoadStrategies(database.DB); err != nil {
		log.Printf(i18n.Get("StrategyLoadFromDBFailed"), err)
	}

	// Optional: delegate to Python worker via gRPC
	var pyClient *strategy.WorkerClient
	if cfg.EnablePythonWorker {
		c, err := strategy.NewWorkerClient(cfg.PythonWorkerAddr)
		if err != nil {
			log.Printf(i18n.Get("PythonWorkerInitFailed"), err)
		} else {
			pyClient = c
			// Python strategy loading might need similar DB logic in future
			// For now, keep it as is or adapt if needed.
			// stratEngine.Add(strategy.NewPythonStrategy("python_worker", pyClient))
			log.Printf(i18n.Get("PythonWorkerEnabled"), cfg.PythonWorkerAddr)
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
			// Panic recovery to prevent goroutine crash and ensure balance unlocks
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf(i18n.Get("SignalProcessingPanic"), r)
						bus.Publish(events.EventRiskAlert, fmt.Sprintf("Signal processing panic: %v", r))
					}
				}()

				sig, ok := msg.(strategy.Signal)
				if !ok {
					return
				}

				// Resolve strategy owner and bound connection (if any)
				var (
					stratUserID     sql.NullString
					stratConnID     sql.NullString
					stratExchangeTy sql.NullString
				)
				if err := database.DB.QueryRowContext(ctx, `
					SELECT si.user_id, si.connection_id, c.exchange_type
					FROM strategy_instances si
					LEFT JOIN connections c ON si.connection_id = c.id
					WHERE si.id = ?
				`, sig.StrategyID).Scan(&stratUserID, &stratConnID, &stratExchangeTy); err != nil && err != sql.ErrNoRows {
					log.Printf("strategy owner lookup failed for %s: %v", sig.StrategyID, err)
				}
				userID := ""
				if stratUserID.Valid {
					userID = stratUserID.String
				}
				connectionID := ""
				if stratConnID.Valid {
					connectionID = stratConnID.String
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
				// Build account snapshot for risk evaluation (per-user when possible)
				balSource := balanceMgr
				if userID != "" && userBalanceMgr != nil {
					if userBalMgr, err := userBalanceMgr.GetOrCreate(userID); err == nil {
						balSource = userBalMgr
					} else {
						log.Printf("per-user balance manager init failed for user %s: %v - using global balance", userID, err)
					}
				}

				balSnap := balSource.GetBalance()
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

				// I2: Single entry point for all risk checks (per-user when possible)
				signalInput := risk.SignalInput{
					Symbol: sig.Symbol,
					Action: sig.Action,
					Size:   sig.Size,
					Price:  price,
				}

				var decision risk.RiskDecision
				if userID != "" && multiUserRisk != nil {
					dec, err := multiUserRisk.EvaluateForUser(userID, signalInput, position, account, sig.StrategyID)
					if err != nil {
						log.Printf("per-user risk eval failed for user %s: %v - falling back to global", userID, err)
						decision = riskMgr.EvaluateFull(signalInput, position, account, sig.StrategyID)
					} else {
						decision = dec
					}
				} else {
					decision = riskMgr.EvaluateFull(signalInput, position, account, sig.StrategyID)
				}
				if !decision.Allowed {
					log.Printf(i18n.Get("RiskRejected"), decision.Reason)
					bus.Publish(events.EventRiskAlert, decision.Reason)
					return
				}
				if decision.Warning != "" {
					log.Printf(i18n.Get("RiskWarning"), decision.Warning)
				}

				// Determine final order size
				size := decision.AdjustedSize
				if size == 0 {
					size = sig.Size
				}

				// I3: Lock balance AFTER evaluation, with final adjusted size (per-user when possible)
				finalOrderValue := size * price
				if err := balSource.Lock(finalOrderValue); err != nil {
					log.Printf(i18n.Get("BalanceLockFailed"), err)
					bus.Publish(events.EventRiskAlert, fmt.Sprintf("Insufficient balance: %v", err))
					return
				}

				// Register SL/TP for trailing logic (does not auto-place orders)
				cfgCopy := riskMgr.GetConfig()
				stopLossMgr.AddPosition(risk.StopLossPosition{
					StrategyID:     sig.StrategyID, // I4: per-strategy tracking
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
				orderMarket := marketFromVenue(venue)
				if stratExchangeTy.Valid {
					orderMarket = marketFromVenue(stratExchangeTy.String)
				}

				o := order.Order{
					ID:                 uuid.NewString(),
					StrategyInstanceID: sig.StrategyID,
					Symbol:             sig.Symbol,
					Side:               sig.Action,
					Type:               "MARKET",
					Qty:                size,
					Status:             "NEW",
					CreatedAt:          time.Now(),
					Market:             orderMarket,
					StopPrice:          decision.StopLoss,
					ActivationPrice:    decision.TakeProfit,
					UserID:             userID,
					ConnectionID:       connectionID,
				}
				orderQueue.Enqueue(o)
			}() // End of panic recovery wrapper
		}
	}()

	go orderQueue.Drain(ctx, func(o order.Order) {
		asyncExec.ExecuteAsync(ctx, o) // V2 P0-B: Async Execution
	})

	// Monitor async execution results (V2 P0-B)
	go func() {
		for result := range asyncExec.Results() {
			if !result.Success {
				log.Printf(i18n.Get("AsyncExecutionFailed"), result.OrderID, result.Error)
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

	// Create Engine Service (Phase 1 Architecture)
	engService := engine.NewImpl(engine.Config{
		StratEngine: stratEngine,
		RiskMgr:     riskMgr,
		BalanceMgr:  balanceMgr,
		OrderQueue:  orderQueue,
		Bus:         bus,
		DB:          database,
		Meta: engine.SystemStatus{
			Mode: func() string {
				if cfg.DryRun {
					return "DRY_RUN"
				}
				return "LIVE"
			}(),
			DryRun:      cfg.DryRun,
			Venue:       venue,
			Symbols:     cfg.BinanceSymbols,
			UseMockFeed: cfg.UseMockFeed,
			Version:     buildVersion,
		},
		MultiUserRiskMgr: multiUserRisk,
	})
	log.Println(i18n.Get("EngineServiceInit"))

	// API
	server := api.NewServer(
		bus,
		database,
		engService,
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
		keyMgr,
		userBalanceMgr,
	)
	go func() {
		if err := server.Start(":" + cfg.Port); err != nil {
			log.Fatalf(i18n.Get("APIServerError"), err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	log.Println(i18n.Get("ShuttingDown"))
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
