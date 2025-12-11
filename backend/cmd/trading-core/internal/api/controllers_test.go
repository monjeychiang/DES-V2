package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"trading-core/internal/balance"
	"trading-core/internal/engine"
	"trading-core/internal/events"
	"trading-core/internal/monitor"
	"trading-core/internal/order"
	"trading-core/pkg/db"
)

type noopEngine struct{}

func (noopEngine) StartStrategy(context.Context, string) error             { return nil }
func (noopEngine) PauseStrategy(context.Context, string) error             { return nil }
func (noopEngine) StopStrategy(context.Context, string) error              { return nil }
func (noopEngine) PanicSellStrategy(context.Context, string, string) error { return nil }
func (noopEngine) UpdateStrategyParams(context.Context, string, map[string]any) error {
	return nil
}
func (noopEngine) BindStrategyConnection(context.Context, string, string, string) error {
	return nil
}
func (noopEngine) ListStrategies(context.Context, string) ([]engine.StrategyInfo, error) {
	return nil, nil
}
func (noopEngine) GetStrategyStatus(context.Context, string) (*engine.StrategyStatus, error) {
	return nil, nil
}
func (noopEngine) GetStrategyPosition(context.Context, string) (float64, error) { return 0, nil }
func (noopEngine) GetPositions(context.Context) ([]engine.Position, error)      { return nil, nil }
func (noopEngine) GetOpenOrders(context.Context) ([]engine.Order, error)        { return nil, nil }
func (noopEngine) GetRiskMetrics(context.Context) (*engine.RiskMetrics, error)  { return nil, nil }
func (noopEngine) GetStrategyPerformance(context.Context, string, time.Time, time.Time) (*engine.Performance, error) {
	return nil, nil
}
func (noopEngine) GetBalance(context.Context) (*engine.BalanceInfo, error) { return nil, nil }
func (noopEngine) GetSystemStatus(context.Context) *engine.SystemStatus {
	return &engine.SystemStatus{}
}

type noopQueue struct{}

func (noopQueue) Enqueue(order.Order) bool                 { return true }
func (noopQueue) Drain(context.Context, func(order.Order)) {}
func (noopQueue) Len() int                                 { return 0 }
func (noopQueue) PendingNotional() float64                 { return 0 }
func (noopQueue) Close()                                   {}

type testKeyManager struct{}

func (testKeyManager) Encrypt(plaintext string) (string, error) { return "enc:" + plaintext, nil }
func (testKeyManager) Decrypt(ciphertext string) (string, error) {
	return strings.TrimPrefix(ciphertext, "enc:"), nil
}
func (testKeyManager) CurrentVersion() int { return 1 }

func newTestAPIServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	if err := db.ApplyMigrations(database); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}

	bus := events.NewBus()
	engineSvc := noopEngine{}
	metrics := monitor.NewSystemMetrics()
	queue := noopQueue{}

	balances := balance.NewMultiUserManager(func(userID string) (*balance.Manager, error) {
		mgr := balance.NewManager(nil, 30*time.Second)
		mgr.SetInitialBalance(10000.0)
		return mgr, nil
	})

	server := NewServer(
		bus,
		database,
		engineSvc,
		metrics,
		queue,
		SystemMeta{
			DryRun:      true,
			Venue:       "none",
			Symbols:     []string{"BTCUSDT"},
			UseMockFeed: true,
			Version:     "test",
		},
		"test-secret",
		testKeyManager{},
		balances,
	)

	httpServer := httptest.NewServer(server.Router)

	cleanup := func() {
		httpServer.Close()
		_ = database.Close()
	}
	return httpServer, cleanup
}

func doJSONRequest(t *testing.T, client *http.Client, method, url, token string, payload any, out any) int {
	t.Helper()

	var buf bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&buf).Encode(payload); err != nil {
			t.Fatalf("encode payload: %v", err)
		}
	}

	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
	return resp.StatusCode
}

func registerAndLogin(t *testing.T, client *http.Client, baseURL string) string {
	t.Helper()
	var regResp struct {
		UserID string `json:"user_id"`
	}
	status := doJSONRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/register", "", map[string]string{
		"username": "tester",
		"email":    "tester@example.com",
		"password": "StrongPass123!",
	}, &regResp)
	if status != http.StatusCreated {
		t.Fatalf("register status=%d resp=%+v", status, regResp)
	}

	var loginResp struct {
		Token string `json:"token"`
	}
	status = doJSONRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/login", "", map[string]string{
		"email":    "tester@example.com",
		"password": "StrongPass123!",
	}, &loginResp)
	if status != http.StatusOK || loginResp.Token == "" {
		t.Fatalf("login failed status=%d resp=%+v", status, loginResp)
	}
	return loginResp.Token
}

func TestCreateStrategyValidation(t *testing.T) {
	ts, cleanup := newTestAPIServer(t)
	defer cleanup()

	client := ts.Client()
	token := registerAndLogin(t, client, ts.URL)

	var resp struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	status := doJSONRequest(t, client, http.MethodPost, ts.URL+"/api/v1/strategies", token, map[string]any{
		"name": "",
	}, &resp)
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", status)
	}
	if resp.Code != "INVALID_REQUEST" {
		t.Fatalf("expected code INVALID_REQUEST, got %s", resp.Code)
	}
}

func TestCreateAndListStrategies(t *testing.T) {
	ts, cleanup := newTestAPIServer(t)
	defer cleanup()

	client := ts.Client()
	token := registerAndLogin(t, client, ts.URL)

	createPayload := map[string]any{
		"name":          "MA Cross BTC",
		"strategy_type": "ma_cross",
		"symbol":        "BTCUSDT",
		"interval":      "1m",
		"parameters": map[string]any{
			"fast": 5,
			"slow": 20,
		},
	}
	var createResp struct {
		ID string `json:"id"`
	}
	status := doJSONRequest(t, client, http.MethodPost, ts.URL+"/api/v1/strategies", token, createPayload, &createResp)
	if status != http.StatusCreated {
		t.Fatalf("create strategy status=%d resp=%+v", status, createResp)
	}
	if createResp.ID == "" {
		t.Fatalf("expected created strategy id")
	}

	var listResp []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Symbol string `json:"symbol"`
	}
	status = doJSONRequest(t, client, http.MethodGet, ts.URL+"/api/v1/strategies?limit=5", token, nil, &listResp)
	if status != http.StatusOK {
		t.Fatalf("list strategies status=%d", status)
	}
	if len(listResp) == 0 {
		t.Fatalf("expected at least one strategy")
	}
}

func TestCreateOrderValidation(t *testing.T) {
	ts, cleanup := newTestAPIServer(t)
	defer cleanup()

	client := ts.Client()
	token := registerAndLogin(t, client, ts.URL)

	var resp struct {
		Code string `json:"code"`
	}
	status := doJSONRequest(t, client, http.MethodPost, ts.URL+"/api/v1/orders", token, map[string]any{
		"symbol": "BTCUSDT",
		"side":   "BUY",
		"type":   "LIMIT",
		"qty":    0,
	}, &resp)
	if status != http.StatusBadRequest || resp.Code != "INVALID_REQUEST" {
		t.Fatalf("expected validation error, got status=%d resp=%+v", status, resp)
	}
}

func TestCreateAndListOrders(t *testing.T) {
	ts, cleanup := newTestAPIServer(t)
	defer cleanup()

	client := ts.Client()
	token := registerAndLogin(t, client, ts.URL)

	var connResp struct {
		ID string `json:"id"`
	}
	status := doJSONRequest(t, client, http.MethodPost, ts.URL+"/api/v1/connections", token, map[string]any{
		"name":          "Test Spot",
		"exchange_type": "binance-spot",
		"api_key":       "k",
		"api_secret":    "s",
	}, &connResp)
	if status != http.StatusCreated || connResp.ID == "" {
		t.Fatalf("create connection failed status=%d resp=%+v", status, connResp)
	}

	var createResp struct {
		ID string `json:"id"`
	}
	status = doJSONRequest(t, client, http.MethodPost, ts.URL+"/api/v1/orders", token, map[string]any{
		"symbol":        "BTCUSDT",
		"side":          "BUY",
		"type":          "LIMIT",
		"price":         10000.0,
		"qty":           0.01,
		"connection_id": connResp.ID,
	}, &createResp)
	if status != http.StatusAccepted || createResp.ID == "" {
		t.Fatalf("create order failed status=%d resp=%+v", status, createResp)
	}

	var listResp []struct {
		ID     string `json:"id"`
		Symbol string `json:"symbol"`
	}
	status = doJSONRequest(t, client, http.MethodGet, ts.URL+"/api/v1/orders?limit=1", token, nil, &listResp)
	if status != http.StatusOK {
		t.Fatalf("list orders status=%d", status)
	}
}

func TestStrategyParamsValidation_RSI(t *testing.T) {
	ts, cleanup := newTestAPIServer(t)
	defer cleanup()

	client := ts.Client()
	token := registerAndLogin(t, client, ts.URL)

	var resp struct {
		Code string `json:"code"`
	}
	status := doJSONRequest(t, client, http.MethodPost, ts.URL+"/api/v1/strategies", token, map[string]any{
		"name":          "bad rsi",
		"strategy_type": "rsi",
		"symbol":        "BTCUSDT",
		"interval":      "1m",
		"parameters": map[string]any{
			"period":     0,   // invalid
			"oversold":   30,  // ok
			"overbought": 70,  // ok
			"size":       0.1, // ok
		},
	}, &resp)
	if status != http.StatusBadRequest || resp.Code != "INVALID_PARAMETERS" {
		t.Fatalf("expected invalid parameters, got status=%d code=%s", status, resp.Code)
	}
}
