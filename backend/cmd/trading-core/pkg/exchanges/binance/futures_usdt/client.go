package futures_usdt

import (
	"context"
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

// Config holds Binance USDT-M futures credentials.
type Config struct {
	APIKey     string
	APISecret  string
	Testnet    bool
	RecvWindow int64 // ms
}

// Client handles Binance USDT-M futures.
type Client struct {
	cfg         Config
	baseURL     string
	httpClient  *http.Client
	timeSync    *common.TimeSync
	rateLimiter *common.RateLimiter
}

// NewClient creates a new USDT-M futures client.
func NewClient(cfg Config) *Client {
	base := "https://fapi.binance.com"
	if cfg.Testnet {
		base = "https://testnet.binancefuture.com"
	}
	if cfg.RecvWindow == 0 {
		cfg.RecvWindow = 5000
	}
	c := &Client{
		cfg:        cfg,
		baseURL:    base,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	c.timeSync = common.NewTimeSync(func() (int64, error) {
		return c.GetServerTime()
	})
	c.rateLimiter = common.NewRateLimiter(2400, time.Minute) // 2400 weight/min for futures
	return c
}

// CreateListenKey creates a listen key for user data stream.
func (c *Client) CreateListenKey(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/fapi/v1/listenKey", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-MBX-APIKEY", c.cfg.APIKey)
	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("create listen key status %d: %s", res.StatusCode, string(b))
	}
	var out struct {
		ListenKey string `json:"listenKey"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.ListenKey, nil
}

// KeepAliveListenKey extends listen key life.
func (c *Client) KeepAliveListenKey(ctx context.Context, listenKey string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/fapi/v1/listenKey?listenKey="+listenKey, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-MBX-APIKEY", c.cfg.APIKey)
	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("keepalive listen key status %d: %s", res.StatusCode, string(b))
	}
	return nil
}

// Helper: convert to consistent timestamp with time sync if available.
func (c *Client) now() int64 {
	if c.timeSync != nil && c.timeSync.Offset() != 0 {
		return c.timeSync.Now()
	}
	return time.Now().UnixMilli()
}

// SubmitOrder places an order.
func (c *Client) SubmitOrder(ctx context.Context, req common.OrderRequest) (common.OrderResult, error) {
	if c.cfg.APIKey == "" || c.cfg.APISecret == "" {
		return common.OrderResult{}, errors.New("binance usdt futures: API key/secret required")
	}
	params := url.Values{}
	params.Set("symbol", req.Symbol)
	params.Set("side", strings.ToUpper(string(req.Side)))
	params.Set("type", strings.ToUpper(string(req.Type)))
	params.Set("quantity", formatFloat(req.Qty))

	// Set price for limit orders
	if req.Type == common.OrderTypeLimit ||
		req.Type == common.OrderTypeStopLossLimit ||
		req.Type == common.OrderTypeTakeProfitLimit {
		params.Set("price", formatFloat(req.Price))
		params.Set("timeInForce", string(toBinanceTIF(req.TimeInForce)))
	}

	// Set stopPrice for stop orders
	if req.Type == common.OrderTypeStopLoss ||
		req.Type == common.OrderTypeStopLossLimit ||
		req.Type == common.OrderTypeTakeProfit ||
		req.Type == common.OrderTypeTakeProfitLimit {
		params.Set("stopPrice", formatFloat(req.StopPrice))
		if req.WorkingType != "" {
			params.Set("workingType", req.WorkingType)
		}
		if req.PriceProtect {
			params.Set("priceProtect", "TRUE")
		}
	}

	// Trailing stop parameters
	if req.Type == common.OrderTypeTrailingStop {
		params.Set("callbackRate", formatFloat(req.CallbackRate))
		if req.ActivationPrice > 0 {
			params.Set("activationPrice", formatFloat(req.ActivationPrice))
		}
	}

	if req.ClientID != "" {
		params.Set("newClientOrderId", req.ClientID)
	}
	if req.PositionSide != "" {
		params.Set("positionSide", req.PositionSide)
	}
	if req.ReduceOnly {
		params.Set("reduceOnly", "true")
	}

	// Use synchronized time
	timestamp := time.Now().UnixMilli()
	if c.timeSync != nil && c.timeSync.Offset() != 0 {
		timestamp = c.timeSync.Now()
	}
	params.Set("timestamp", strconv.FormatInt(timestamp, 10))
	params.Set("recvWindow", strconv.FormatInt(c.cfg.RecvWindow, 10))

	endpoint := c.baseURL + "/fapi/v1/order"
	body, err := c.doSigned(ctx, http.MethodPost, endpoint, params)
	if err != nil {
		return common.OrderResult{}, err
	}
	var resp orderResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return common.OrderResult{}, fmt.Errorf("decode order: %w", err)
	}
	return common.OrderResult{
		ExchangeOrderID: fmt.Sprintf("%d", resp.OrderID),
		Status:          mapStatus(resp.Status),
		ClientID:        resp.ClientOrderID,
	}, nil
}

// CancelOrder cancels an order by symbol and ID.
func (c *Client) CancelOrder(ctx context.Context, symbol, exchangeOrderID string) error {
	if c.cfg.APIKey == "" || c.cfg.APISecret == "" {
		return errors.New("binance usdt futures: API key/secret required")
	}
	params := url.Values{}
	params.Set("symbol", symbol)
	if exchangeOrderID != "" {
		params.Set("orderId", exchangeOrderID)
	}
	params.Set("timestamp", strconv.FormatInt(c.now(), 10))
	params.Set("recvWindow", strconv.FormatInt(c.cfg.RecvWindow, 10))
	endpoint := c.baseURL + "/fapi/v1/order"
	_, err := c.doSigned(ctx, http.MethodDelete, endpoint, params)
	return err
}

// CancelAllOpenOrders cancels all open orders for a symbol.
func (c *Client) CancelAllOpenOrders(ctx context.Context, symbol string) error {
	if c.cfg.APIKey == "" || c.cfg.APISecret == "" {
		return errors.New("binance usdt futures: API key/secret required")
	}
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("timestamp", strconv.FormatInt(c.now(), 10))
	params.Set("recvWindow", strconv.FormatInt(c.cfg.RecvWindow, 10))

	endpoint := c.baseURL + "/fapi/v1/allOpenOrders"
	_, err := c.doSigned(ctx, http.MethodDelete, endpoint, params)
	return err
}

// GetAccountInfo returns futures account balances and flags.
func (c *Client) GetAccountInfo(ctx context.Context) (*FuturesAccountInfo, error) {
	if c.cfg.APIKey == "" || c.cfg.APISecret == "" {
		return nil, errors.New("binance usdt futures: API key/secret required")
	}
	params := url.Values{}
	params.Set("timestamp", strconv.FormatInt(c.now(), 10))
	params.Set("recvWindow", strconv.FormatInt(c.cfg.RecvWindow, 10))
	endpoint := c.baseURL + "/fapi/v2/account"
	body, err := c.doSigned(ctx, http.MethodGet, endpoint, params)
	if err != nil {
		return nil, err
	}
	var info FuturesAccountInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("decode account info: %w", err)
	}
	return &info, nil
}

// GetPositions returns position risk view.
func (c *Client) GetPositions(ctx context.Context, symbol string) ([]PositionRisk, error) {
	if c.cfg.APIKey == "" || c.cfg.APISecret == "" {
		return nil, errors.New("binance usdt futures: API key/secret required")
	}
	params := url.Values{}
	if symbol != "" {
		params.Set("symbol", symbol)
	}
	params.Set("timestamp", strconv.FormatInt(c.now(), 10))
	params.Set("recvWindow", strconv.FormatInt(c.cfg.RecvWindow, 10))
	endpoint := c.baseURL + "/fapi/v2/positionRisk"
	body, err := c.doSigned(ctx, http.MethodGet, endpoint, params)
	if err != nil {
		return nil, err
	}
	var pos []PositionRisk
	if err := json.Unmarshal(body, &pos); err != nil {
		return nil, fmt.Errorf("decode positions: %w", err)
	}
	return pos, nil
}

// GetOpenOrders returns open orders; symbol optional.
func (c *Client) GetOpenOrders(ctx context.Context, symbol string) ([]OpenOrder, error) {
	if c.cfg.APIKey == "" || c.cfg.APISecret == "" {
		return nil, errors.New("binance usdt futures: API key/secret required")
	}
	params := url.Values{}
	if symbol != "" {
		params.Set("symbol", symbol)
	}
	params.Set("timestamp", strconv.FormatInt(c.now(), 10))
	params.Set("recvWindow", strconv.FormatInt(c.cfg.RecvWindow, 10))
	endpoint := c.baseURL + "/fapi/v1/openOrders"
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

// GetBalance returns futures balances.
func (c *Client) GetBalance(ctx context.Context) ([]FuturesBalance, error) {
	params := url.Values{}
	params.Set("timestamp", strconv.FormatInt(c.now(), 10))
	params.Set("recvWindow", strconv.FormatInt(c.cfg.RecvWindow, 10))
	endpoint := c.baseURL + "/fapi/v2/balance"
	body, err := c.doSigned(ctx, http.MethodGet, endpoint, params)
	if err != nil {
		return nil, err
	}
	var bal []FuturesBalance
	if err := json.Unmarshal(body, &bal); err != nil {
		return nil, fmt.Errorf("decode balance: %w", err)
	}
	return bal, nil
}

// SetLeverage sets leverage for a symbol.
func (c *Client) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("leverage", strconv.Itoa(leverage))
	params.Set("timestamp", strconv.FormatInt(c.now(), 10))
	params.Set("recvWindow", strconv.FormatInt(c.cfg.RecvWindow, 10))
	endpoint := c.baseURL + "/fapi/v1/leverage"
	_, err := c.doSigned(ctx, http.MethodPost, endpoint, params)
	return err
}

// SetMarginType sets margin type (ISOLATED or CROSSED).
func (c *Client) SetMarginType(ctx context.Context, symbol, marginType string) error {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("marginType", strings.ToUpper(marginType))
	params.Set("timestamp", strconv.FormatInt(c.now(), 10))
	params.Set("recvWindow", strconv.FormatInt(c.cfg.RecvWindow, 10))
	endpoint := c.baseURL + "/fapi/v1/marginType"
	_, err := c.doSigned(ctx, http.MethodPost, endpoint, params)
	return err
}

// SetPositionSideDual enables/disables hedge mode.
func (c *Client) SetPositionSideDual(ctx context.Context, dual bool) error {
	params := url.Values{}
	params.Set("dualSidePosition", strconv.FormatBool(dual))
	params.Set("timestamp", strconv.FormatInt(c.now(), 10))
	params.Set("recvWindow", strconv.FormatInt(c.cfg.RecvWindow, 10))
	endpoint := c.baseURL + "/fapi/v1/positionSide/dual"
	_, err := c.doSigned(ctx, http.MethodPost, endpoint, params)
	return err
}

// ChangePositionMargin adjusts position margin.
func (c *Client) ChangePositionMargin(ctx context.Context, symbol string, amount float64, mType int) error {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("amount", formatFloat(amount))
	params.Set("type", strconv.Itoa(mType))
	params.Set("timestamp", strconv.FormatInt(c.now(), 10))
	params.Set("recvWindow", strconv.FormatInt(c.cfg.RecvWindow, 10))
	endpoint := c.baseURL + "/fapi/v1/positionMargin"
	_, err := c.doSigned(ctx, http.MethodPost, endpoint, params)
	return err
}

// GetUserTrades returns user trades.
func (c *Client) GetUserTrades(ctx context.Context, symbol string, limit int, fromID string) ([]UserTrade, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if fromID != "" {
		params.Set("fromId", fromID)
	}
	params.Set("timestamp", strconv.FormatInt(c.now(), 10))
	params.Set("recvWindow", strconv.FormatInt(c.cfg.RecvWindow, 10))
	endpoint := c.baseURL + "/fapi/v1/userTrades"
	body, err := c.doSigned(ctx, http.MethodGet, endpoint, params)
	if err != nil {
		return nil, err
	}
	var trades []UserTrade
	if err := json.Unmarshal(body, &trades); err != nil {
		return nil, fmt.Errorf("decode user trades: %w", err)
	}
	return trades, nil
}

// GetIncome fetches income history.
func (c *Client) GetIncome(ctx context.Context, symbol, incomeType string, limit int) ([]Income, error) {
	params := url.Values{}
	if symbol != "" {
		params.Set("symbol", symbol)
	}
	if incomeType != "" {
		params.Set("incomeType", incomeType)
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	params.Set("timestamp", strconv.FormatInt(c.now(), 10))
	params.Set("recvWindow", strconv.FormatInt(c.cfg.RecvWindow, 10))
	endpoint := c.baseURL + "/fapi/v1/income"
	body, err := c.doSigned(ctx, http.MethodGet, endpoint, params)
	if err != nil {
		return nil, err
	}
	var income []Income
	if err := json.Unmarshal(body, &income); err != nil {
		return nil, fmt.Errorf("decode income: %w", err)
	}
	return income, nil
}

// GetServerTime fetches futures server time.
func (c *Client) GetServerTime() (int64, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/fapi/v1/time")
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

// doSigned handles signing and sending requests.
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
		req, err = http.NewRequestWithContext(ctx, method, endpoint+"?"+encoded, nil)
	default:
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

	if c.rateLimiter != nil {
		weightHeader := res.Header.Get("X-MBX-USED-WEIGHT-1M")
		c.rateLimiter.UpdateFromHeader(weightHeader)
	}

	body, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("binance usdt futures %s %s status %d: %s", method, endpoint, res.StatusCode, string(body))
	}
	return body, nil
}

type orderResp struct {
	Symbol        string `json:"symbol"`
	OrderID       int64  `json:"orderId"`
	ClientOrderID string `json:"clientOrderId"`
	Status        string `json:"status"`
}

type FuturesAccountInfo struct {
	CanTrade   bool  `json:"canTrade"`
	UpdateTime int64 `json:"updateTime"`
	Assets     []struct {
		Asset            string `json:"asset"`
		WalletBalance    string `json:"walletBalance"`
		UnrealizedProfit string `json:"unrealizedProfit"`
	} `json:"assets"`
	Positions []PositionRisk `json:"positions"`
}

type PositionRisk struct {
	Symbol           string `json:"symbol"`
	PositionSide     string `json:"positionSide"`
	PositionAmt      string `json:"positionAmt"`
	EntryPrice       string `json:"entryPrice"`
	UnRealizedProfit string `json:"unRealizedProfit"`
	Leverage         string `json:"leverage"`
}

func toBinanceTIF(tif common.TimeInForce) common.TimeInForce {
	if tif == "" {
		return common.TIFGTC
	}
	return tif
}
