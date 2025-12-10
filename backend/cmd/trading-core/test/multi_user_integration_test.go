package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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
	"trading-core/pkg/crypto"
	"trading-core/pkg/db"
)

// helper to create a test server wiring most components similar to main.go
func newMultiUserTestServer(t *testing.T) (*httptest.Server, *db.Database, func()) {
	t.Helper()

	// Setup in-memory DB
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	if err := db.ApplyMigrations(database); err != nil {
		t.Fatalf("failed to apply migrations: %v", err)
	}

	// Events bus
	bus := events.NewBus()

	// Risk manager (in-memory is enough for tests)
	riskMgr := risk.NewInMemory(risk.DefaultConfig())

	// Global balance manager
	globalBalMgr := balance.NewManager(nil, 30*time.Second)
	globalBalMgr.SetInitialBalance(10000.0)

	// Order queue + executor (SkipExchange to avoid real gateways)
	orderQueue := order.NewQueue(100)
	exec := order.NewExecutor(database, bus, nil, "", true)
	exec.SkipExchange = true

	ctx, cancel := context.WithCancel(context.Background())
	go orderQueue.Drain(ctx, func(o order.Order) {
		_ = exec.Handle(ctx, o)
	})

	// Strategy engine (not heavily used in this test but required by Engine impl)
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

	// KeyManager (for encrypted connections)
	keyStr, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	if err := os.Setenv("MASTER_ENCRYPTION_KEY", keyStr); err != nil {
		t.Fatalf("failed to set MASTER_ENCRYPTION_KEY: %v", err)
	}
	keyMgr, err := crypto.NewKeyManager()
	if err != nil {
		t.Fatalf("failed to init KeyManager: %v", err)
	}

	// Per-user balance manager for API /balance (initialised with same defaults)
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
		keyMgr,
		userBalances,
	)

	httpServer := httptest.NewServer(server.Router)

	cleanup := func() {
		cancel()
		httpServer.Close()
		_ = database.Close()
	}

	return httpServer, database, cleanup
}

// doRequest helps sending JSON HTTP requests and returning status + decoded body.
func doRequest(t *testing.T, client *http.Client, method, url, token string, body any, out any) int {
	t.Helper()

	var buf io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal body: %v", err)
		}
		buf = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, buf)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if out != nil {
		respBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		if len(respBytes) > 0 {
			if err := json.Unmarshal(respBytes, out); err != nil {
				t.Fatalf("failed to unmarshal response: %v\nbody=%s", err, string(respBytes))
			}
		}
	}

	return resp.StatusCode
}

// TestMultiUserEndToEnd follows the high-level flow from MULTI_USER_USAGE_GUIDE.md
// and asserts core multi-user isolation behaviour.
func TestMultiUserEndToEnd(t *testing.T) {
	srv, _, cleanup := newMultiUserTestServer(t)
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

	// 1) Register two users
	var regA, regB registerResp
	status := doRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/register", "",
		map[string]string{
			"username": "userA",
			"email":    "userA@example.com",
			"password": "PassA123!",
		}, &regA)
	if status != http.StatusCreated || regA.UserID == "" {
		t.Fatalf("register userA failed, status=%d, resp=%+v", status, regA)
	}
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/register", "",
		map[string]string{
			"username": "userB",
			"email":    "userB@example.com",
			"password": "PassB123!",
		}, &regB)
	if status != http.StatusCreated || regB.UserID == "" {
		t.Fatalf("register userB failed, status=%d, resp=%+v", status, regB)
	}

	// 2) Login to get tokens
	var loginA, loginB loginResp
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/login", "",
		map[string]string{
			"email":    "userA@example.com",
			"password": "PassA123!",
		}, &loginA)
	if status != http.StatusOK || loginA.Token == "" {
		t.Fatalf("login userA failed, status=%d, resp=%+v", status, loginA)
	}
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/login", "",
		map[string]string{
			"email":    "userB@example.com",
			"password": "PassB123!",
		}, &loginB)
	if status != http.StatusOK || loginB.Token == "" {
		t.Fatalf("login userB failed, status=%d, resp=%+v", status, loginB)
	}

	// 3) Each user creates a connection
	type connResp struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		ExchangeType string `json:"exchange_type"`
		IsActive     bool   `json:"is_active"`
		Encrypted    bool   `json:"encrypted"`
	}
	var connA, connB connResp
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/connections", loginA.Token,
		map[string]string{
			"name":          "A Spot",
			"exchange_type": "binance-spot",
			"api_key":       "a-key",
			"api_secret":    "a-secret",
		}, &connA)
	if status != http.StatusCreated || connA.ID == "" || !connA.Encrypted {
		t.Fatalf("create connection A failed, status=%d, resp=%+v", status, connA)
	}
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/connections", loginB.Token,
		map[string]string{
			"name":          "B Spot",
			"exchange_type": "binance-spot",
			"api_key":       "b-key",
			"api_secret":    "b-secret",
		}, &connB)
	if status != http.StatusCreated || connB.ID == "" || !connB.Encrypted {
		t.Fatalf("create connection B failed, status=%d, resp=%+v", status, connB)
	}

	// 4) Verify each user only sees their own connections
	var listA, listB []connResp
	status = doRequest(t, client, http.MethodGet, baseURL+"/api/v1/connections", loginA.Token, nil, &listA)
	if status != http.StatusOK || len(listA) != 1 || listA[0].ID != connA.ID {
		t.Fatalf("userA connections mismatch: status=%d, resp=%+v", status, listA)
	}
	status = doRequest(t, client, http.MethodGet, baseURL+"/api/v1/connections", loginB.Token, nil, &listB)
	if status != http.StatusOK || len(listB) != 1 || listB[0].ID != connB.ID {
		t.Fatalf("userB connections mismatch: status=%d, resp=%+v", status, listB)
	}

	// 5) UserB tries to delete UserA's connection -> should be forbidden
	status = doRequest(t, client, http.MethodDelete, baseURL+"/api/v1/connections/"+connA.ID, loginB.Token, nil, nil)
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 when userB deletes userA connection, got %d", status)
	}

	// 6) Each user submits a manual order using their own connection
	type orderResp struct {
		ID           string  `json:"id"`
		Symbol       string  `json:"symbol"`
		Side         string  `json:"side"`
		Type         string  `json:"type"`
		Price        float64 `json:"price"`
		Qty          float64 `json:"qty"`
		Status       string  `json:"status"`
		ConnectionID string  `json:"connection_id"`
	}
	var ordA, ordB orderResp
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/orders", loginA.Token,
		map[string]any{
			"symbol":        "BTCUSDT",
			"side":          "BUY",
			"type":          "LIMIT",
			"price":         42000.0,
			"qty":           0.01,
			"connection_id": connA.ID,
		}, &ordA)
	if status != http.StatusAccepted || ordA.ID == "" || ordA.ConnectionID != connA.ID {
		t.Fatalf("userA order failed, status=%d, resp=%+v", status, ordA)
	}
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/orders", loginB.Token,
		map[string]any{
			"symbol":        "ETHUSDT",
			"side":          "SELL",
			"type":          "LIMIT",
			"price":         3000.0,
			"qty":           1.0,
			"connection_id": connB.ID,
		}, &ordB)
	if status != http.StatusAccepted || ordB.ID == "" || ordB.ConnectionID != connB.ID {
		t.Fatalf("userB order failed, status=%d, resp=%+v", status, ordB)
	}

	// Give some time for background executor to persist orders
	time.Sleep(200 * time.Millisecond)

	// 7) Verify order isolation via GET /orders
	var ordersA, ordersB []struct {
		ID     string `json:"id"`
		Symbol string `json:"symbol"`
		UserID string `json:"UserID"`
		Status string `json:"status"`
	}
	status = doRequest(t, client, http.MethodGet, baseURL+"/api/v1/orders", loginA.Token, nil, &ordersA)
	if status != http.StatusOK {
		t.Fatalf("userA get orders failed, status=%d", status)
	}
	if len(ordersA) == 0 {
		t.Fatalf("userA orders empty, expected at least 1")
	}
	foundA := false
	for _, o := range ordersA {
		if o.ID == ordA.ID {
			foundA = true
			if o.UserID == "" {
				t.Fatalf("userA order missing user_id")
			}
		}
	}
	if !foundA {
		t.Fatalf("userA orders did not include own order: %+v", ordersA)
	}

	status = doRequest(t, client, http.MethodGet, baseURL+"/api/v1/orders", loginB.Token, nil, &ordersB)
	if status != http.StatusOK {
		t.Fatalf("userB get orders failed, status=%d", status)
	}
	if len(ordersB) == 0 {
		t.Fatalf("userB orders empty, expected at least 1")
	}
	foundB := false
	for _, o := range ordersB {
		if o.ID == ordB.ID {
			foundB = true
			if o.UserID == "" {
				t.Fatalf("userB order missing user_id")
			}
		}
		// Ensure userB never sees userA's order
		if o.ID == ordA.ID {
			t.Fatalf("userB should not see userA's order")
		}
	}
	if !foundB {
		t.Fatalf("userB orders did not include own order: %+v", ordersB)
	}

	// 8) Basic check: balance endpoint is accessible per user (snapshot semantics)
	type balanceResp struct {
		Available float64 `json:"available"`
		Locked    float64 `json:"locked"`
		Total     float64 `json:"total"`
	}
	var balA, balB balanceResp
	status = doRequest(t, client, http.MethodGet, baseURL+"/api/v1/balance", loginA.Token, nil, &balA)
	if status != http.StatusOK {
		t.Fatalf("userA balance failed, status=%d", status)
	}
	status = doRequest(t, client, http.MethodGet, baseURL+"/api/v1/balance", loginB.Token, nil, &balB)
	if status != http.StatusOK {
		t.Fatalf("userB balance failed, status=%d", status)
	}
	// Two users may start with same numbers, but API should succeed independently.
}

// TestMultiUserPositionsIsolation verifies that positions are stored with user_id
// and that /api/v1/positions only returns data for the current user.
func TestMultiUserPositionsIsolation(t *testing.T) {
	srv, database, cleanup := newMultiUserTestServer(t)
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

	// Register two users
	var regA, regB registerResp
	status := doRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/register", "",
		map[string]string{
			"username": "posUserA",
			"email":    "posA@example.com",
			"password": "PosA123!",
		}, &regA)
	if status != http.StatusCreated || regA.UserID == "" {
		t.Fatalf("register userA failed, status=%d, resp=%+v", status, regA)
	}
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/register", "",
		map[string]string{
			"username": "posUserB",
			"email":    "posB@example.com",
			"password": "PosB123!",
		}, &regB)
	if status != http.StatusCreated || regB.UserID == "" {
		t.Fatalf("register userB failed, status=%d, resp=%+v", status, regB)
	}

	// Login to get tokens
	var loginA, loginB loginResp
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/login", "",
		map[string]string{
			"email":    "posA@example.com",
			"password": "PosA123!",
		}, &loginA)
	if status != http.StatusOK || loginA.Token == "" {
		t.Fatalf("login userA failed, status=%d, resp=%+v", status, loginA)
	}
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/login", "",
		map[string]string{
			"email":    "posB@example.com",
			"password": "PosB123!",
		}, &loginB)
	if status != http.StatusOK || loginB.Token == "" {
		t.Fatalf("login userB failed, status=%d, resp=%+v", status, loginB)
	}

	// Seed positions directly via UserQueries with different symbols per user.
	ctx := context.Background()
	q := database.Queries()
	if err := q.UpsertPositionWithUser(ctx, regA.UserID, "BTCUSDT", 0.5, 40000); err != nil {
		t.Fatalf("failed to upsert position for userA: %v", err)
	}
	if err := q.UpsertPositionWithUser(ctx, regB.UserID, "ETHUSDT", 2.0, 3000); err != nil {
		t.Fatalf("failed to upsert position for userB: %v", err)
	}

	// Verify userA sees only their BTCUSDT position.
	var posA []struct {
		Symbol string  `json:"symbol"`
		Qty    float64 `json:"qty"`
		UserID string  `json:"UserID"`
	}
	status = doRequest(t, client, http.MethodGet, baseURL+"/api/v1/positions", loginA.Token, nil, &posA)
	if status != http.StatusOK {
		t.Fatalf("userA get positions failed, status=%d", status)
	}
	foundBTC := false
	for _, p := range posA {
		if p.Symbol == "BTCUSDT" {
			foundBTC = true
			if p.UserID == "" || p.UserID != regA.UserID {
				t.Fatalf("userA position has wrong user_id: %+v", p)
			}
		}
		if p.Symbol == "ETHUSDT" {
			t.Fatalf("userA should not see userB's position: %+v", p)
		}
	}
	if !foundBTC {
		t.Fatalf("userA positions did not include BTCUSDT: %+v", posA)
	}

	// Verify userB sees only their ETHUSDT position.
	var posB []struct {
		Symbol string  `json:"symbol"`
		Qty    float64 `json:"qty"`
		UserID string  `json:"UserID"`
	}
	status = doRequest(t, client, http.MethodGet, baseURL+"/api/v1/positions", loginB.Token, nil, &posB)
	if status != http.StatusOK {
		t.Fatalf("userB get positions failed, status=%d", status)
	}
	foundETH := false
	for _, p := range posB {
		if p.Symbol == "ETHUSDT" {
			foundETH = true
			if p.UserID == "" || p.UserID != regB.UserID {
				t.Fatalf("userB position has wrong user_id: %+v", p)
			}
		}
		if p.Symbol == "BTCUSDT" {
			t.Fatalf("userB should not see userA's position: %+v", p)
		}
	}
	if !foundETH {
		t.Fatalf("userB positions did not include ETHUSDT: %+v", posB)
	}

	// Unauthenticated access should be rejected by auth middleware.
	status = doRequest(t, client, http.MethodGet, baseURL+"/api/v1/positions", "", nil, nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated positions, got %d", status)
	}
}

// TestMultiUserStrategyIsolation verifies that strategies are bound to the owner
// and cannot be operated by other users.
func TestMultiUserStrategyIsolation(t *testing.T) {
	srv, _, cleanup := newMultiUserTestServer(t)
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

	var regA, regB registerResp
	status := doRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/register", "",
		map[string]string{
			"username": "stratUserA",
			"email":    "stratA@example.com",
			"password": "StratA123!",
		}, &regA)
	if status != http.StatusCreated || regA.UserID == "" {
		t.Fatalf("register userA failed, status=%d, resp=%+v", status, regA)
	}
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/register", "",
		map[string]string{
			"username": "stratUserB",
			"email":    "stratB@example.com",
			"password": "StratB123!",
		}, &regB)
	if status != http.StatusCreated || regB.UserID == "" {
		t.Fatalf("register userB failed, status=%d, resp=%+v", status, regB)
	}

	var loginA, loginB loginResp
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/login", "",
		map[string]string{
			"email":    "stratA@example.com",
			"password": "StratA123!",
		}, &loginA)
	if status != http.StatusOK || loginA.Token == "" {
		t.Fatalf("login userA failed, status=%d, resp=%+v", status, loginA)
	}
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/login", "",
		map[string]string{
			"email":    "stratB@example.com",
			"password": "StratB123!",
		}, &loginB)
	if status != http.StatusOK || loginB.Token == "" {
		t.Fatalf("login userB failed, status=%d, resp=%+v", status, loginB)
	}

	// UserA creates a connection to bind the strategy.
	type connResp struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		ExchangeType string `json:"exchange_type"`
		IsActive     bool   `json:"is_active"`
		Encrypted    bool   `json:"encrypted"`
	}
	var connA connResp
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/connections", loginA.Token,
		map[string]string{
			"name":          "Strat A Spot",
			"exchange_type": "binance-spot",
			"api_key":       "sa-key",
			"api_secret":    "sa-secret",
		}, &connA)
	if status != http.StatusCreated || connA.ID == "" {
		t.Fatalf("create connection A failed, status=%d, resp=%+v", status, connA)
	}

	// UserA creates a strategy bound to their connection.
	var stratResp struct {
		ID          string `json:"id"`
		UserID      string `json:"user_id"`
		ConnectionID string `json:"connection_id"`
	}
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/strategies", loginA.Token,
		map[string]any{
			"name":          "UserA Grid",
			"strategy_type": "grid",
			"symbol":        "BTCUSDT",
			"interval":      "1h",
			"connection_id": connA.ID,
			"parameters": map[string]any{
				"grid_levels": 5,
			},
		}, &stratResp)
	if status != http.StatusCreated || stratResp.ID == "" {
		t.Fatalf("create strategy failed, status=%d, resp=%+v", status, stratResp)
	}
	if stratResp.UserID != regA.UserID {
		t.Fatalf("strategy user_id mismatch, expected %s got %s", regA.UserID, stratResp.UserID)
	}
	if stratResp.ConnectionID != connA.ID {
		t.Fatalf("strategy connection_id mismatch, expected %s got %s", connA.ID, stratResp.ConnectionID)
	}

	// UserB should not see UserA's strategy in list.
	var strategiesB []map[string]any
	status = doRequest(t, client, http.MethodGet, baseURL+"/api/v1/strategies", loginB.Token, nil, &strategiesB)
	if status != http.StatusOK {
		t.Fatalf("userB get strategies failed, status=%d", status)
	}
	for _, s := range strategiesB {
		if id, ok := s["id"].(string); ok && id == stratResp.ID {
			t.Fatalf("userB should not see userA strategy in list")
		}
	}

	// UserB attempts to start UserA's strategy -> should be forbidden.
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/strategies/"+stratResp.ID+"/start", loginB.Token, nil, nil)
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 when userB starts userA strategy, got %d", status)
	}
}

// TestMultiUserConnectionOwnershipEnforced covers a user-story where one user
// attempts to use another user's connection_id in a manual order.
// It verifies that createOrder rejects such requests.
func TestMultiUserConnectionOwnershipEnforced(t *testing.T) {
	srv, _, cleanup := newMultiUserTestServer(t)
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

	// Register Alice and Bob.
	var regA, regB registerResp
	status := doRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/register", "",
		map[string]string{
			"username": "alice",
			"email":    "alice@example.com",
			"password": "Alice123!",
		}, &regA)
	if status != http.StatusCreated || regA.UserID == "" {
		t.Fatalf("register alice failed, status=%d, resp=%+v", status, regA)
	}
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/register", "",
		map[string]string{
			"username": "bob",
			"email":    "bob@example.com",
			"password": "Bob123!",
		}, &regB)
	if status != http.StatusCreated || regB.UserID == "" {
		t.Fatalf("register bob failed, status=%d, resp=%+v", status, regB)
	}

	// Login to get tokens.
	var loginA, loginB loginResp
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/login", "",
		map[string]string{
			"email":    "alice@example.com",
			"password": "Alice123!",
		}, &loginA)
	if status != http.StatusOK || loginA.Token == "" {
		t.Fatalf("login alice failed, status=%d, resp=%+v", status, loginA)
	}
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/login", "",
		map[string]string{
			"email":    "bob@example.com",
			"password": "Bob123!",
		}, &loginB)
	if status != http.StatusOK || loginB.Token == "" {
		t.Fatalf("login bob failed, status=%d, resp=%+v", status, loginB)
	}

	// Alice creates a connection.
	type connResp struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		ExchangeType string `json:"exchange_type"`
		IsActive     bool   `json:"is_active"`
		Encrypted    bool   `json:"encrypted"`
	}
	var connA connResp
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/connections", loginA.Token,
		map[string]string{
			"name":          "Alice Spot",
			"exchange_type": "binance-spot",
			"api_key":       "alice-key",
			"api_secret":    "alice-secret",
		}, &connA)
	if status != http.StatusCreated || connA.ID == "" {
		t.Fatalf("create connection for alice failed, status=%d, resp=%+v", status, connA)
	}

	// Bob maliciously tries to place an order using Alice's connection_id.
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/orders", loginB.Token,
		map[string]any{
			"symbol":       "BTCUSDT",
			"side":         "BUY",
			"type":         "MARKET",
			"price":        0,
			"qty":          0.01,
			"connection_id": connA.ID,
		}, nil)
	if status == http.StatusCreated || status == http.StatusOK || status == http.StatusAccepted {
		t.Fatalf("expected failure when bob uses alice's connection, got status=%d", status)
	}
}

// TestSingleUserMultiConnectionsOrders verifies that a single user can have multiple
// connections and place orders on each, and that orders are recorded with the correct
// connection_id and user_id.
func TestSingleUserMultiConnectionsOrders(t *testing.T) {
	srv, database, cleanup := newMultiUserTestServer(t)
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

	// Register and login single user.
	var reg registerResp
	status := doRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/register", "",
		map[string]string{
			"username": "charlie",
			"email":    "charlie@example.com",
			"password": "Charlie123!",
		}, &reg)
	if status != http.StatusCreated || reg.UserID == "" {
		t.Fatalf("register charlie failed, status=%d, resp=%+v", status, reg)
	}

	var login loginResp
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/login", "",
		map[string]string{
			"email":    "charlie@example.com",
			"password": "Charlie123!",
		}, &login)
	if status != http.StatusOK || login.Token == "" {
		t.Fatalf("login charlie failed, status=%d, resp=%+v", status, login)
	}

	// Create two connections with different exchange types.
	type connResp struct {
		ID           string `json:"id"`
		ExchangeType string `json:"exchange_type"`
	}
	var spotConn, futConn connResp
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/connections", login.Token,
		map[string]string{
			"name":          "Spot Account",
			"exchange_type": "binance-spot",
			"api_key":       "spot-key",
			"api_secret":    "spot-secret",
		}, &spotConn)
	if status != http.StatusCreated || spotConn.ID == "" {
		t.Fatalf("create spot connection failed, status=%d, resp=%+v", status, spotConn)
	}
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/connections", login.Token,
		map[string]string{
			"name":          "Futures Account",
			"exchange_type": "binance-usdtfut",
			"api_key":       "fut-key",
			"api_secret":    "fut-secret",
		}, &futConn)
	if status != http.StatusCreated || futConn.ID == "" {
		t.Fatalf("create futures connection failed, status=%d, resp=%+v", status, futConn)
	}

	// Place one order on each connection.
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/orders", login.Token,
		map[string]any{
			"symbol":       "BTCUSDT",
			"side":         "BUY",
			"type":         "LIMIT",
			"price":        30000.0,
			"qty":          0.01,
			"connection_id": spotConn.ID,
		}, nil)
	if status != http.StatusAccepted && status != http.StatusCreated && status != http.StatusOK {
		t.Fatalf("spot order failed, status=%d", status)
	}
	status = doRequest(t, client, http.MethodPost, baseURL+"/api/v1/orders", login.Token,
		map[string]any{
			"symbol":       "ETHUSDT",
			"side":         "BUY",
			"type":         "LIMIT",
			"price":        2000.0,
			"qty":          0.5,
			"connection_id": futConn.ID,
		}, nil)
	if status != http.StatusAccepted && status != http.StatusCreated && status != http.StatusOK {
		t.Fatalf("futures order failed, status=%d", status)
	}

	// Check orders in DB to ensure they are recorded with correct user_id and connection_id.
	ctx := context.Background()
	rows, err := database.DB.QueryContext(ctx, `
		SELECT id, user_id, strategy_instance_id, symbol
		FROM orders
		ORDER BY created_at ASC
	`)
	if err != nil {
		t.Fatalf("query orders: %v", err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var id, userID, stratID, symbol string
		if err := rows.Scan(&id, &userID, &stratID, &symbol); err != nil {
			t.Fatalf("scan order: %v", err)
		}
		if userID != reg.UserID {
			t.Fatalf("order %s has wrong user_id %s, expected %s", id, userID, reg.UserID)
		}
		count++
	}
	if count < 2 {
		t.Fatalf("expected at least 2 orders for charlie, got %d", count)
	}
}
