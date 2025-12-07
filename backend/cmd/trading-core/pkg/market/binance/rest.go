package market

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Client wraps REST access to Binance.
type Client struct {
	APIKey     string
	APISecret  string
	BaseURL    string
	HTTPClient *http.Client
	Testnet    bool
}

// NewClient builds a REST client; use Testnet to switch base URLs.
func NewClient(apiKey, apiSecret string, testnet bool) *Client {
	base := "https://api.binance.com"
	if testnet {
		base = "https://testnet.binance.vision"
	}
	return &Client{
		APIKey:     apiKey,
		APISecret:  apiSecret,
		BaseURL:    base,
		Testnet:    testnet,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetKlines fetches historical klines using the public endpoint.
// Set startTime/endTime to 0 to use default behavior (most recent klines).
func (c *Client) GetKlines(symbol, interval string, limit int, startTime, endTime int64) ([]Kline, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("interval", interval)
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if startTime > 0 {
		params.Set("startTime", strconv.FormatInt(startTime, 10))
	}
	if endTime > 0 {
		params.Set("endTime", strconv.FormatInt(endTime, 10))
	}

	u := fmt.Sprintf("%s/api/v3/klines?%s", c.BaseURL, params.Encode())
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance klines status %d", res.StatusCode)
	}

	var raw [][]any
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, err
	}

	klines := make([]Kline, 0, len(raw))
	for _, item := range raw {
		// Binance returns 12 fields per kline
		if len(item) < 11 {
			continue
		}
		k := Kline{
			OpenTime:            toInt64(item[0]),
			Open:                toFloat(item[1]),
			High:                toFloat(item[2]),
			Low:                 toFloat(item[3]),
			Close:               toFloat(item[4]),
			Volume:              toFloat(item[5]),
			CloseTime:           toInt64(item[6]),
			QuoteVolume:         toFloat(item[7]),
			NumberOfTrades:      toInt(item[8]),
			TakerBuyBaseVolume:  toFloat(item[9]),
			TakerBuyQuoteVolume: toFloat(item[10]),
			// item[11] is unused/ignore
		}
		klines = append(klines, k)
	}
	return klines, nil
}

// GetServerTime fetches Binance server time in milliseconds.
func (c *Client) GetServerTime() (int64, error) {
	u := fmt.Sprintf("%s/api/v3/time", c.BaseURL)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("binance server time status %d", res.StatusCode)
	}

	var resp struct {
		ServerTime int64 `json:"serverTime"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return 0, err
	}
	return resp.ServerTime, nil
}

func toFloat(v any) float64 {
	switch t := v.(type) {
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	case json.Number:
		f, _ := t.Float64()
		return f
	case float64:
		return t
	default:
		return 0
	}
}

func toInt64(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case json.Number:
		i, _ := t.Int64()
		return i
	default:
		return 0
	}
}

func toInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case json.Number:
		i, _ := t.Int64()
		return int(i)
	default:
		return 0
	}
}
