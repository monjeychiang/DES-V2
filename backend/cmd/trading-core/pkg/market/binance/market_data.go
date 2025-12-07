package market

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// MarketDataClient wraps common spot market data endpoints.
type MarketDataClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewMarketDataClient(testnet bool) *MarketDataClient {
	base := "https://api.binance.com"
	if testnet {
		base = "https://testnet.binance.vision"
	}
	return &MarketDataClient{
		baseURL:    base,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Ping checks connectivity.
func (c *MarketDataClient) Ping(ctx context.Context) error {
	_, err := c.do(ctx, "/api/v3/ping", nil)
	return err
}

// ServerTime fetches Binance server time (milliseconds).
func (c *MarketDataClient) ServerTime(ctx context.Context) (int64, error) {
	body, err := c.do(ctx, "/api/v3/time", nil)
	if err != nil {
		return 0, err
	}
	var resp struct {
		ServerTime int64 `json:"serverTime"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, err
	}
	return resp.ServerTime, nil
}

// ExchangeInfo fetches minimal exchange info (symbols and filters trimmed).
func (c *MarketDataClient) ExchangeInfo(ctx context.Context, symbol string) (map[string]any, error) {
	params := url.Values{}
	if symbol != "" {
		params.Set("symbol", symbol)
	}
	body, err := c.do(ctx, "/api/v3/exchangeInfo", params)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return data, nil
}

// Depth returns order book snapshots.
func (c *MarketDataClient) Depth(ctx context.Context, symbol string, limit int) (map[string]any, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	body, err := c.do(ctx, "/api/v3/depth", params)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return data, nil
}

// Klines returns raw kline array.
func (c *MarketDataClient) Klines(ctx context.Context, symbol, interval string, limit int) ([]any, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("interval", interval)
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	body, err := c.do(ctx, "/api/v3/klines", params)
	if err != nil {
		return nil, err
	}
	var out []any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *MarketDataClient) do(ctx context.Context, path string, params url.Values) ([]byte, error) {
	u := c.baseURL + path
	if params != nil {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("binance market data %s status %d: %s", path, res.StatusCode, string(body))
	}
	return body, nil
}
