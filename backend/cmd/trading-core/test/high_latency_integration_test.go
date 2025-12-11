package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"trading-core/internal/api"
	"trading-core/internal/balance"
	"trading-core/internal/engine"
	"trading-core/internal/events"
	"trading-core/internal/monitor"
	"trading-core/internal/order"
	"trading-core/internal/risk"
	"trading-core/internal/strategy"
	"trading-core/pkg/db"
	exchange "trading-core/pkg/exchanges/common"
)

// delayedGateway simulates a slow exchange gateway.
type delayedGateway struct {
	delay time.Duration
}

func (g *delayedGateway) SubmitOrder(ctx context.Context, req exchange.OrderRequest) (exchange.OrderResult, error) {
	time.Sleep(g.delay)
	return exchange.OrderResult{
		Status:          exchange.StatusFilled,
		ExchangeOrderID: "ex-" + req.ClientID,
	}, nil
}

func (g *delayedGateway) CancelOrder(ctx context.Context, symbol, exchangeOrderID string) error {
	return nil
}

// delayedPool returns a new delayedGateway for any connection.
type delayedPool struct {
	delay time.Duration
}

func (p *delayedPool) GetOrCreate(ctx context.Context, userID, connectionID string) (exchange.Gateway, error) {
	return &delayedGateway{delay: p.delay}, nil
}

// newHighLatencyTestServer wires a test server with AsyncExecutor and a slow gateway.
func newHighLatencyTestServer(t *testing.T, delay time.Duration) (*httptest.Server, func()) {
	t.Helper()

	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	if err := db.ApplyMigrations(database); err != nil {
		t.Fatalf("failed to apply migrations: %v", err)
	}

	bus := events.NewBus()
	riskMgr := risk.NewInMemory(risk.DefaultConfig())

	// Global balance manager
	globalBalMgr := balance.NewManager(nil, 30*time.Second)
	globalBalMgr.SetInitialBalance(10000.0)

	// Order queue + async executor with delayed gateway pool
	orderQueue := order.NewQueue(200)
	exec := order.NewExecutor(database, bus, nil, "fake", false)
	exec.SetGatewayPool(&delayedPool{delay: delay})
	exec.SkipExchange = false
	asyncExec := order.NewAsyncExecutor(exec, 4)

	ctx, cancel := context.WithCancel(context.Background())
	go orderQueue.Drain(ctx, func(o order.Order) {
		asyncExec.ExecuteAsync(ctx, o)
	})

	// Strategy engine (minimal)
	stratEngine := strategy.NewEngine(bus, database.DB, strategy.Context{Indicators: nil})
	engService := engine.NewImpl(engine.Config{
		StratEngine: stratEngine,
		RiskMgr:     riskMgr,
		BalanceMgr:  globalBalMgr,
		OrderQueue:  orderQueue,
		Bus:         bus,
		DB:          database,
		Meta: engine.SystemStatus{
			Mode:        "TEST",
			DryRun:      true,
			Venue:       "none",
			Symbols:     []string{"BTCUSDT"},
			UseMockFeed: true,
			Version:     "test",
		},
	})

	// Per-user balances
	userBalances := balance.NewMultiUserManager(func(userID string) (*balance.Manager, error) {
		mgr := balance.NewManager(nil, 30*time.Second)
		mgr.SetInitialBalance(10000.0)
		return mgr, nil
	})

	sysMetrics := monitor.NewSystemMetrics()

	server := api.NewServer(
		bus,
		database,
		engService,
		sysMetrics,
		orderQueue,
		api.SystemMeta{
			DryRun:      true,
			Venue:       "none",
			Symbols:     []string{"BTCUSDT"},
			UseMockFeed: true,
			Version:     "test",
		},
		"test-jwt-secret",
		nil, // KeyManager not needed for this test (plaintext keys)
		userBalances,
	)

	httpServer := httptest.NewServer(server.Router)

	cleanup := func() {
		cancel()
		httpServer.Close()
		_ = database.Close()
	}
	return httpServer, cleanup
}

// TestHighLatencyAsyncOrders ensures that even在 gateway 延遲時，API 仍快速回應，且最終狀態落庫。
func TestHighLatencyAsyncOrders(t *testing.T) {
	delay := 500 * time.Millisecond
	srv, cleanup := newHighLatencyTestServer(t, delay)
	defer cleanup()

	client := srv.Client()
	baseURL := srv.URL

	type registerResp struct {
		UserID   string `json:"user_id"`
		Username string `json:"username"`
	}
	type loginResp struct {
		Token string `json:"token"`
	}
	type connResp struct {
		ID string `json:"id"`
	}

	// Register & login
	var reg registerResp
	status := doRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/register", "",
		map[string]string{
			"username": "latencyUser",
			"email":    "latency@example.com",
			"password": "Latency123!",
		}, &reg)
	if status != http.StatusCreated || reg.UserID == "" {
		t.Fatalf("register failed, status=%d resp=%+v", status, reg)
	}

	var login loginResp
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/login", "",
		map[string]string{
			"email":    "latency@example.com",
			"password": "Latency123!",
		}, &login)
	if status != http.StatusOK || login.Token == "" {
		t.Fatalf("login failed, status=%d resp=%+v", status, login)
	}

	// Create a connection
	var conn connResp
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/connections", login.Token,
		map[string]string{
			"name":          "Latency Spot",
			"exchange_type": "binance-spot",
			"api_key":       "k",
			"api_secret":    "s",
		}, &conn)
	if status != http.StatusCreated || conn.ID == "" {
		t.Fatalf("create connection failed, status=%d resp=%+v", status, conn)
	}

	// Rapidly send multiple orders; total API time should be far less than N*delay (non-blocking).
	const totalOrders = 5
	start := time.Now()
	for i := 0; i < totalOrders; i++ {
		status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/orders", login.Token,
			map[string]any{
				"symbol":        "BTCUSDT",
				"side":          "BUY",
				"type":          "LIMIT",
				"price":         30000.0,
				"qty":           0.01,
				"connection_id": conn.ID,
			}, nil)
		if status != http.StatusAccepted && status != http.StatusCreated && status != http.StatusOK {
			t.Fatalf("order %d failed, status=%d", i, status)
		}
	}
	elapsed := time.Since(start)
	// 如果是同步，預期 ~ totalOrders*delay；這裡驗證確實小於 1 秒。
	if elapsed >= delay*2 {
		t.Fatalf("orders were blocked by gateway delay: elapsed=%v", elapsed)
	}

	// 等待後台處理完成（略大於 gateway 延遲）
	time.Sleep(delay * 2)

	// 查詢 orders，確認已落庫且數量正確。
	var orders []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		UserID string `json:"user_id"`
	}
	status = doRequest(t, client, http.MethodGet, baseURL+"/api/v1/orders", login.Token, nil, &orders)
	if status != http.StatusOK {
		t.Fatalf("get orders failed, status=%d", status)
	}
	if len(orders) < totalOrders {
		t.Fatalf("expected at least %d orders, got %d", totalOrders, len(orders))
	}
	for _, o := range orders {
		if o.UserID != "" && o.UserID != reg.UserID {
			t.Fatalf("order %s has wrong user_id %s", o.ID, o.UserID)
		}
	}
}
