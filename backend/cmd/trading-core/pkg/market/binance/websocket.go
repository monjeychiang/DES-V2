package market

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// StreamClient manages lightweight streaming from Binance public websockets.
type StreamClient struct {
	StreamURL       string
	dialer          *websocket.Dialer
	ReconnectConfig *ReconnectConfig
}

// ReconnectConfig defines the reconnection behavior.
type ReconnectConfig struct {
	Enabled      bool          // Whether auto-reconnect is enabled
	MaxRetries   int           // Maximum number of reconnection attempts (0 = unlimited)
	InitialDelay time.Duration // Initial delay before first reconnect attempt
	MaxDelay     time.Duration // Maximum delay between reconnect attempts
	Multiplier   float64       // Delay multiplier for exponential backoff
}

// DefaultReconnectConfig returns sensible defaults for reconnection.
func DefaultReconnectConfig() *ReconnectConfig {
	return &ReconnectConfig{
		Enabled:      true,
		MaxRetries:   10,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
}

// NewStreamClient builds a websocket client; testnet toggles the host.
func NewStreamClient(testnet bool) *StreamClient {
	host := "stream.binance.com:9443"
	if testnet {
		host = "testnet.binance.vision"
	}
	return &StreamClient{
		StreamURL:       (&url.URL{Scheme: "wss", Host: host, Path: "/ws"}).String(),
		dialer:          websocket.DefaultDialer,
		ReconnectConfig: DefaultReconnectConfig(),
	}
}

// NewStreamClientWithConfig builds a websocket client with custom reconnect config.
func NewStreamClientWithConfig(testnet bool, reconnectCfg *ReconnectConfig) *StreamClient {
	c := NewStreamClient(testnet)
	if reconnectCfg != nil {
		c.ReconnectConfig = reconnectCfg
	}
	return c
}

// calculateBackoff returns the delay for the given retry attempt using exponential backoff.
func (c *StreamClient) calculateBackoff(attempt int) time.Duration {
	if c.ReconnectConfig == nil {
		return time.Second
	}
	delay := float64(c.ReconnectConfig.InitialDelay)
	for i := 0; i < attempt; i++ {
		delay *= c.ReconnectConfig.Multiplier
	}
	if time.Duration(delay) > c.ReconnectConfig.MaxDelay {
		return c.ReconnectConfig.MaxDelay
	}
	return time.Duration(delay)
}

// SubscribeKlines listens to kline stream and pushes parsed klines into a channel.
// It returns the channel and a stop function. Auto-reconnection is enabled by default.
func (c *StreamClient) SubscribeKlines(ctx context.Context, symbol, interval string) (<-chan Kline, func(), error) {
	// Binance requires lowercase symbols for WebSocket streams
	stream := fmt.Sprintf("%s@kline_%s", strings.ToLower(symbol), interval)
	u := fmt.Sprintf("%s/%s", c.StreamURL, stream)

	conn, _, err := c.dialer.DialContext(ctx, u, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("dial binance ws: %w", err)
	}

	out := make(chan Kline, 100)
	stopCh := make(chan struct{})
	var stopOnce sync.Once
	var mu sync.Mutex
	currentConn := conn

	stop := func() {
		stopOnce.Do(func() {
			close(stopCh)
			mu.Lock()
			if currentConn != nil {
				_ = currentConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				_ = currentConn.Close()
			}
			mu.Unlock()
			close(out)
		})
	}

	// reconnect attempts to establish a new connection with exponential backoff.
	reconnect := func() (*websocket.Conn, error) {
		if c.ReconnectConfig == nil || !c.ReconnectConfig.Enabled {
			return nil, fmt.Errorf("reconnect disabled")
		}

		maxRetries := c.ReconnectConfig.MaxRetries
		if maxRetries == 0 {
			maxRetries = 100 // Effectively unlimited but bounded
		}

		for attempt := 0; attempt < maxRetries; attempt++ {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-stopCh:
				return nil, fmt.Errorf("stopped")
			default:
			}

			delay := c.calculateBackoff(attempt)
			log.Printf("🔄 [%s] WebSocket reconnecting in %v (attempt %d/%d)", symbol, delay, attempt+1, maxRetries)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-stopCh:
				return nil, fmt.Errorf("stopped")
			}

			newConn, _, err := c.dialer.DialContext(ctx, u, nil)
			if err != nil {
				log.Printf("❌ [%s] Reconnect failed: %v", symbol, err)
				continue
			}

			log.Printf("✅ [%s] WebSocket reconnected successfully", symbol)
			return newConn, nil
		}
		return nil, fmt.Errorf("max retries (%d) exceeded", maxRetries)
	}

	go func() {
		defer stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stopCh:
				return
			default:
			}

			mu.Lock()
			activeConn := currentConn
			mu.Unlock()

			if activeConn == nil {
				return
			}

			_, msg, err := activeConn.ReadMessage()
			if err != nil {
				// Check if explicitly stopped
				select {
				case <-stopCh:
					return
				case <-ctx.Done():
					return
				default:
				}

				// If connection closed normally, exit quietly
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) ||
					strings.Contains(err.Error(), "use of closed network connection") {
					return
				}

				log.Printf("⚠️ [%s] WebSocket read error: %v", symbol, err)

				// Attempt reconnection
				if c.ReconnectConfig != nil && c.ReconnectConfig.Enabled {
					mu.Lock()
					_ = currentConn.Close()
					mu.Unlock()

					newConn, reconErr := reconnect()
					if reconErr != nil {
						log.Printf("❌ [%s] Failed to reconnect: %v", symbol, reconErr)
						return
					}

					mu.Lock()
					currentConn = newConn
					mu.Unlock()
					continue
				}
				return
			}

			parsed, err := parseKlineMessage(msg)
			if err != nil {
				log.Printf("binance ws parse error: %v", err)
				continue
			}

			select {
			case out <- parsed:
			default:
				// Channel full, drop oldest to avoid blocking
			}
		}
	}()

	return out, stop, nil
}

// SubscribeTrades subscribes to trade stream and emits parsed trades.
func (c *StreamClient) SubscribeTrades(ctx context.Context, symbol string) (<-chan Trade, func(), error) {
	stream := fmt.Sprintf("%s@trade", symbol)
	u := fmt.Sprintf("%s/%s", c.StreamURL, stream)

	conn, _, err := c.dialer.DialContext(ctx, u, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("dial binance ws trades: %w", err)
	}

	out := make(chan Trade, 100)
	var once sync.Once
	stop := func() {
		once.Do(func() {
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			_ = conn.Close()
			close(out)
		})
	}

	go func() {
		defer stop()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			_, msg, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) ||
					strings.Contains(err.Error(), "use of closed network connection") {
					return
				}
				log.Printf("binance ws trade read error: %v", err)
				return
			}

			parsed, err := parseTradeMessage(msg)
			if err != nil {
				log.Printf("binance ws trade parse error: %v", err)
				continue
			}
			out <- parsed
		}
	}()

	return out, stop, nil
}

// SubscribeBookTicker subscribes to best bid/ask updates.
func (c *StreamClient) SubscribeBookTicker(ctx context.Context, symbol string) (<-chan BookTicker, func(), error) {
	stream := fmt.Sprintf("%s@bookTicker", symbol)
	u := fmt.Sprintf("%s/%s", c.StreamURL, stream)

	conn, _, err := c.dialer.DialContext(ctx, u, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("dial binance ws bookTicker: %w", err)
	}

	out := make(chan BookTicker, 100)
	var once sync.Once
	stop := func() {
		once.Do(func() {
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			_ = conn.Close()
			close(out)
		})
	}

	go func() {
		defer stop()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			_, msg, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) ||
					strings.Contains(err.Error(), "use of closed network connection") {
					return
				}
				log.Printf("binance ws bookTicker read error: %v", err)
				return
			}

			parsed, err := parseBookTickerMessage(msg)
			if err != nil {
				log.Printf("binance ws bookTicker parse error: %v", err)
				continue
			}
			out <- parsed
		}
	}()

	return out, stop, nil
}

// SubscribeDepth subscribes to diff depth stream.
func (c *StreamClient) SubscribeDepth(ctx context.Context, symbol string) (<-chan DepthUpdate, func(), error) {
	stream := fmt.Sprintf("%s@depth", symbol)
	u := fmt.Sprintf("%s/%s", c.StreamURL, stream)

	conn, _, err := c.dialer.DialContext(ctx, u, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("dial binance ws depth: %w", err)
	}

	out := make(chan DepthUpdate, 100)
	var once sync.Once
	stop := func() {
		once.Do(func() {
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			_ = conn.Close()
			close(out)
		})
	}

	go func() {
		defer stop()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			_, msg, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) ||
					strings.Contains(err.Error(), "use of closed network connection") {
					return
				}
				log.Printf("binance ws depth read error: %v", err)
				return
			}

			parsed, err := parseDepthMessage(msg)
			if err != nil {
				log.Printf("binance ws depth parse error: %v", err)
				continue
			}
			out <- parsed
		}
	}()

	return out, stop, nil
}

// SubscribeTicker subscribes to 24h ticker stream.
func (c *StreamClient) SubscribeTicker(ctx context.Context, symbol string) (<-chan Ticker, func(), error) {
	stream := fmt.Sprintf("%s@ticker", symbol)
	u := fmt.Sprintf("%s/%s", c.StreamURL, stream)

	conn, _, err := c.dialer.DialContext(ctx, u, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("dial binance ws ticker: %w", err)
	}

	out := make(chan Ticker, 100)
	var once sync.Once
	stop := func() {
		once.Do(func() {
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			_ = conn.Close()
			close(out)
		})
	}

	go func() {
		defer stop()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			_, msg, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) ||
					strings.Contains(err.Error(), "use of closed network connection") {
					return
				}
				log.Printf("binance ws ticker read error: %v", err)
				return
			}

			parsed, err := parseTickerMessage(msg)
			if err != nil {
				log.Printf("binance ws ticker parse error: %v", err)
				continue
			}
			out <- parsed
		}
	}()

	return out, stop, nil
}

// parseKlineMessage decodes only the fields we need.
func parseKlineMessage(msg []byte) (Kline, error) {
	var raw struct {
		Data struct {
			StartTime int64       `json:"t"`
			CloseTime int64       `json:"T"`
			Symbol    string      `json:"s"`
			Interval  string      `json:"i"`
			Open      interface{} `json:"o"`
			Close     interface{} `json:"c"`
			High      interface{} `json:"h"`
			Low       interface{} `json:"l"`
			Volume    interface{} `json:"v"`
		} `json:"k"`
	}
	if err := json.Unmarshal(msg, &raw); err != nil {
		return Kline{}, err
	}
	return Kline{
		Symbol:    raw.Data.Symbol,
		OpenTime:  raw.Data.StartTime,
		CloseTime: raw.Data.CloseTime,
		Open:      toFloat(raw.Data.Open),
		Close:     toFloat(raw.Data.Close),
		High:      toFloat(raw.Data.High),
		Low:       toFloat(raw.Data.Low),
		Volume:    toFloat(raw.Data.Volume),
	}, nil
}

func parseTradeMessage(msg []byte) (Trade, error) {
	var raw struct {
		EventTime interface{} `json:"E"`
		Symbol    string      `json:"s"`
		Price     interface{} `json:"p"`
		Qty       interface{} `json:"q"`
		TradeTime interface{} `json:"T"`
		BuyerIsMM bool        `json:"m"`
	}
	if err := json.Unmarshal(msg, &raw); err != nil {
		return Trade{}, err
	}
	return Trade{
		Symbol:       raw.Symbol,
		Price:        toFloat(raw.Price),
		Qty:          toFloat(raw.Qty),
		Time:         toInt64(raw.TradeTime),
		IsBuyerMaker: raw.BuyerIsMM,
	}, nil
}

func parseBookTickerMessage(msg []byte) (BookTicker, error) {
	var raw struct {
		Symbol string      `json:"s"`
		Bid    interface{} `json:"b"`
		Ask    interface{} `json:"a"`
	}
	if err := json.Unmarshal(msg, &raw); err != nil {
		return BookTicker{}, err
	}
	return BookTicker{
		Symbol:   raw.Symbol,
		BidPrice: toFloat(raw.Bid),
		AskPrice: toFloat(raw.Ask),
		Time:     0,
	}, nil
}

func parseDepthMessage(msg []byte) (DepthUpdate, error) {
	var raw struct {
		Symbol string          `json:"s"`
		Time   interface{}     `json:"E"`
		Bids   [][]interface{} `json:"b"`
		Asks   [][]interface{} `json:"a"`
	}
	if err := json.Unmarshal(msg, &raw); err != nil {
		return DepthUpdate{}, err
	}
	var bids [][2]float64
	for _, b := range raw.Bids {
		if len(b) < 2 {
			continue
		}
		bids = append(bids, [2]float64{toFloat(b[0]), toFloat(b[1])})
	}
	var asks [][2]float64
	for _, a := range raw.Asks {
		if len(a) < 2 {
			continue
		}
		asks = append(asks, [2]float64{toFloat(a[0]), toFloat(a[1])})
	}
	return DepthUpdate{
		Symbol: raw.Symbol,
		Bids:   bids,
		Asks:   asks,
		Time:   toInt64(raw.Time),
	}, nil
}

func parseTickerMessage(msg []byte) (Ticker, error) {
	var raw struct {
		Symbol string      `json:"s"`
		Last   interface{} `json:"c"`
		CloseT int64       `json:"C"` // close time
	}
	if err := json.Unmarshal(msg, &raw); err != nil {
		return Ticker{}, err
	}
	return Ticker{
		Symbol: raw.Symbol,
		Price:  toFloat(raw.Last),
		Time:   raw.CloseT,
	}, nil
}

// Ping keeps the connection alive; useful if the caller wants manual control.
func (c *StreamClient) Ping(conn *websocket.Conn) error {
	return conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(time.Second))
}
