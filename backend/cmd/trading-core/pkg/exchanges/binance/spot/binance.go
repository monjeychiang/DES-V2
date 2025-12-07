package spot

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"trading-core/pkg/exchanges/common"
)

// Config holds Binance credentials.
type Config struct {
	APIKey     string
	APISecret  string
	Testnet    bool
	RecvWindow int64 // ms
}

// Client is a Binance spot trading client (new structure).
type Client struct {
	cfg         Config
	baseURL     string
	httpClient  *http.Client
	timeSync    *common.TimeSync
	rateLimiter *common.RateLimiter
}

func New(cfg Config) *Client {
	base := "https://api.binance.com"
	if cfg.Testnet {
		base = "https://testnet.binance.vision"
	}
	if cfg.RecvWindow == 0 {
		cfg.RecvWindow = 5000
	}
	client := &Client{
		cfg:        cfg,
		baseURL:    base,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	// Initialize TimeSync (will sync on first use)
	client.timeSync = common.NewTimeSync(func() (int64, error) {
		return client.GetServerTime()
	})
	// Rate limiter: 1200 weight/min for spot
	client.rateLimiter = common.NewRateLimiter(1200, time.Minute)
	return client
}

func (c *Client) SubmitOrder(ctx context.Context, req common.OrderRequest) (common.OrderResult, error) {
	if c.cfg.APIKey == "" || c.cfg.APISecret == "" {
		return common.OrderResult{}, errors.New("binance: API key/secret required")
	}

	side := strings.ToUpper(string(req.Side))
	ordType := strings.ToUpper(string(req.Type))
	if ordType == "" {
		ordType = string(common.OrderTypeLimit)
	}
	params := url.Values{}
	params.Set("symbol", req.Symbol)
	params.Set("side", side)
	params.Set("type", ordType)
	params.Set("quantity", formatFloat(req.Qty))

	// Set price for limit orders
	if req.Type == common.OrderTypeLimit ||
		req.Type == common.OrderTypeStopLossLimit ||
		req.Type == common.OrderTypeTakeProfitLimit ||
		req.Type == common.OrderTypeLimitMaker {
		params.Set("price", formatFloat(req.Price))
		params.Set("timeInForce", string(toBinanceTIF(req.TimeInForce)))
	}

	// Set stopPrice for stop-loss/take-profit orders
	if req.Type == common.OrderTypeStopLoss ||
		req.Type == common.OrderTypeStopLossLimit ||
		req.Type == common.OrderTypeTakeProfit ||
		req.Type == common.OrderTypeTakeProfitLimit {
		params.Set("stopPrice", formatFloat(req.StopPrice))
	}

	// Set icebergQty for iceberg orders
	if req.IcebergQty > 0 {
		params.Set("icebergQty", formatFloat(req.IcebergQty))
	}

	if req.ClientID != "" {
		params.Set("newClientOrderId", req.ClientID)
	}
	// Use synchronized time to avoid timestamp errors
	timestamp := time.Now().UnixMilli()
	if c.timeSync != nil && c.timeSync.Offset() != 0 {
		timestamp = c.timeSync.Now()
	}
	params.Set("timestamp", strconv.FormatInt(timestamp, 10))
	params.Set("recvWindow", strconv.FormatInt(c.cfg.RecvWindow, 10))

	endpoint := c.baseURL + "/api/v3/order"
	body, err := c.doSigned(ctx, http.MethodPost, endpoint, params)
	if err != nil {
		return common.OrderResult{}, err
	}

	var resp orderResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return common.OrderResult{}, fmt.Errorf("decode order response: %w", err)
	}

	return common.OrderResult{
		ExchangeOrderID: fmt.Sprintf("%d", resp.OrderID),
		Status:          mapStatus(resp.Status),
		ClientID:        resp.ClientOrderID,
	}, nil
}

func (c *Client) CancelOrder(ctx context.Context, symbol, exchangeOrderID string) error {
	if c.cfg.APIKey == "" || c.cfg.APISecret == "" {
		return errors.New("binance: API key/secret required")
	}
	params := url.Values{}
	params.Set("symbol", symbol)
	if exchangeOrderID != "" {
		params.Set("orderId", exchangeOrderID)
	}

	// Use synchronized time
	timestamp := time.Now().UnixMilli()
	if c.timeSync != nil && c.timeSync.Offset() != 0 {
		timestamp = c.timeSync.Now()
	}
	params.Set("timestamp", strconv.FormatInt(timestamp, 10))
	params.Set("recvWindow", strconv.FormatInt(c.cfg.RecvWindow, 10))

	endpoint := c.baseURL + "/api/v3/order"
	_, err := c.doSigned(ctx, http.MethodDelete, endpoint, params)
	return err
}

// CancelAllOpenOrders cancels all open orders for a symbol.
func (c *Client) CancelAllOpenOrders(ctx context.Context, symbol string) error {
	if c.cfg.APIKey == "" || c.cfg.APISecret == "" {
		return errors.New("binance: API key/secret required")
	}
	params := url.Values{}
	params.Set("symbol", symbol)

	// Use synchronized time
	timestamp := time.Now().UnixMilli()
	if c.timeSync != nil && c.timeSync.Offset() != 0 {
		timestamp = c.timeSync.Now()
	}
	params.Set("timestamp", strconv.FormatInt(timestamp, 10))
	params.Set("recvWindow", strconv.FormatInt(c.cfg.RecvWindow, 10))

	endpoint := c.baseURL + "/api/v3/openOrders"
	_, err := c.doSigned(ctx, http.MethodDelete, endpoint, params)
	return err
}

// doSigned signs the query and performs the HTTP request.
func (c *Client) doSigned(ctx context.Context, method, endpoint string, params url.Values) ([]byte, error) {
	sig := sign(params.Encode(), c.cfg.APISecret)
	params.Set("signature", sig)

	var (
		req *http.Request
		err error
	)
	encoded := params.Encode()
	switch method {
	case http.MethodGet, http.MethodDelete:
		// For GET/DELETE Binance expects signed params in query string.
		urlWithQuery := endpoint + "?" + encoded
		req, err = http.NewRequestWithContext(ctx, method, urlWithQuery, nil)
	default:
		// For POST we can send as form body.
		req, err = http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(encoded))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-MBX-APIKEY", c.cfg.APIKey)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// Track rate limit usage
	if c.rateLimiter != nil {
		weightHeader := res.Header.Get("X-MBX-USED-WEIGHT-1M")
		c.rateLimiter.UpdateFromHeader(weightHeader)
	}

	body, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("binance %s %s status %d: %s", method, endpoint, res.StatusCode, string(body))
	}
	return body, nil
}

// GetServerTime fetches server time (ms).
func (c *Client) GetServerTime() (int64, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/v3/time")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("server time status %d: %s", resp.StatusCode, string(b))
	}
	var res struct {
		ServerTime int64 `json:"serverTime"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return 0, err
	}
	return res.ServerTime, nil
}

// AccountInfo holds balances and permissions.
type AccountInfo struct {
	CanTrade   bool      `json:"canTrade"`
	UpdateTime int64     `json:"updateTime"`
	Balances   []Balance `json:"balances"`
}

// Balance represents an asset balance.
type Balance struct {
	Asset  string `json:"asset"`
	Free   string `json:"free"`
	Locked string `json:"locked"`
}

// GetAccountInfo returns account balances and basic flags.
func (c *Client) GetAccountInfo(ctx context.Context) (*AccountInfo, error) {
	if c.cfg.APIKey == "" || c.cfg.APISecret == "" {
		return nil, errors.New("binance: API key/secret required")
	}
	params := url.Values{}
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	params.Set("recvWindow", strconv.FormatInt(c.cfg.RecvWindow, 10))
	endpoint := c.baseURL + "/api/v3/account"
	body, err := c.doSigned(ctx, http.MethodGet, endpoint, params)
	if err != nil {
		return nil, err
	}
	var info AccountInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("decode account info: %w", err)
	}
	return &info, nil
}

// OpenOrder represents a simplified open order view.
type OpenOrder struct {
	Symbol  string `json:"symbol"`
	OrderID int64  `json:"orderId"`
	Side    string `json:"side"`
	Type    string `json:"type"`
	Price   string `json:"price"`
	OrigQty string `json:"origQty"`
	ExecQty string `json:"executedQty"`
	Status  string `json:"status"`
}

// GetOpenOrders returns current open orders; if symbol is empty, all symbols.
func (c *Client) GetOpenOrders(ctx context.Context, symbol string) ([]OpenOrder, error) {
	if c.cfg.APIKey == "" || c.cfg.APISecret == "" {
		return nil, errors.New("binance: API key/secret required")
	}
	params := url.Values{}
	if symbol != "" {
		params.Set("symbol", symbol)
	}
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	params.Set("recvWindow", strconv.FormatInt(c.cfg.RecvWindow, 10))
	endpoint := c.baseURL + "/api/v3/openOrders"
	body, err := c.doSigned(ctx, http.MethodGet, endpoint, params)
	if err != nil {
		return nil, err
	}
	var orders []OpenOrder
	if err := json.Unmarshal(body, &orders); err != nil {
		return nil, fmt.Errorf("decode open orders: %w", err)
	}
	return orders, nil
}

// GetOrder fetches a single order by symbol and orderId.
func (c *Client) GetOrder(ctx context.Context, symbol, orderID string) (*OpenOrder, error) {
	if c.cfg.APIKey == "" || c.cfg.APISecret == "" {
		return nil, errors.New("binance: API key/secret required")
	}
	params := url.Values{}
	params.Set("symbol", symbol)
	if orderID != "" {
		params.Set("orderId", orderID)
	}
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	params.Set("recvWindow", strconv.FormatInt(c.cfg.RecvWindow, 10))
	endpoint := c.baseURL + "/api/v3/order"
	body, err := c.doSigned(ctx, http.MethodGet, endpoint, params)
	if err != nil {
		return nil, err
	}
	var ord OpenOrder
	if err := json.Unmarshal(body, &ord); err != nil {
		return nil, fmt.Errorf("decode order: %w", err)
	}
	return &ord, nil
}

// GetAllOrders returns historical orders; beware of rate limits.
func (c *Client) GetAllOrders(ctx context.Context, symbol string, limit int) ([]OpenOrder, error) {
	if c.cfg.APIKey == "" || c.cfg.APISecret == "" {
		return nil, errors.New("binance: API key/secret required")
	}
	params := url.Values{}
	params.Set("symbol", symbol)
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	params.Set("recvWindow", strconv.FormatInt(c.cfg.RecvWindow, 10))
	endpoint := c.baseURL + "/api/v3/allOrders"
	body, err := c.doSigned(ctx, http.MethodGet, endpoint, params)
	if err != nil {
		return nil, err
	}
	var orders []OpenOrder
	if err := json.Unmarshal(body, &orders); err != nil {
		return nil, fmt.Errorf("decode all orders: %w", err)
	}
	return orders, nil
}

// MyTrade represents an account trade.
type MyTrade struct {
	ID              int64  `json:"id"`
	Symbol          string `json:"symbol"`
	OrderID         int64  `json:"orderId"`
	Price           string `json:"price"`
	Qty             string `json:"qty"`
	QuoteQty        string `json:"quoteQty"`
	Commission      string `json:"commission"`
	CommissionAsset string `json:"commissionAsset"`
	Time            int64  `json:"time"`
	IsBuyer         bool   `json:"isBuyer"`
	IsMaker         bool   `json:"isMaker"`
}

// GetMyTrades returns account trades for a symbol.
func (c *Client) GetMyTrades(ctx context.Context, symbol string, limit int, fromID string) ([]MyTrade, error) {
	if c.cfg.APIKey == "" || c.cfg.APISecret == "" {
		return nil, errors.New("binance: API key/secret required")
	}
	params := url.Values{}
	params.Set("symbol", symbol)
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if fromID != "" {
		params.Set("fromId", fromID)
	}
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	params.Set("recvWindow", strconv.FormatInt(c.cfg.RecvWindow, 10))
	endpoint := c.baseURL + "/api/v3/myTrades"
	body, err := c.doSigned(ctx, http.MethodGet, endpoint, params)
	if err != nil {
		return nil, err
	}
	var trades []MyTrade
	if err := json.Unmarshal(body, &trades); err != nil {
		return nil, fmt.Errorf("decode my trades: %w", err)
	}
	return trades, nil
}

type orderResponse struct {
	Symbol        string `json:"symbol"`
	OrderID       int64  `json:"orderId"`
	ClientOrderID string `json:"clientOrderId"`
	Status        string `json:"status"`
}

func mapStatus(s string) common.OrderStatus {
	switch strings.ToUpper(s) {
	case "NEW":
		return common.StatusNew
	case "PARTIALLY_FILLED":
		return common.StatusPartial
	case "FILLED":
		return common.StatusFilled
	case "CANCELED":
		return common.StatusCanceled
	case "REJECTED":
		return common.StatusRejected
	case "EXPIRED":
		return common.StatusExpired
	default:
		return common.StatusUnknown
	}
}

func toBinanceTIF(tif common.TimeInForce) common.TimeInForce {
	if tif == "" {
		return common.TIFGTC
	}
	return tif
}

func sign(data, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
