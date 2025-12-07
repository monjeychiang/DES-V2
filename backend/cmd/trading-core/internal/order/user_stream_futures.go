package order

import (
	"context"
	"encoding/json"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"trading-core/internal/events"
	"trading-core/pkg/db"
)

// FuturesUserStream listens to Binance Futures user data stream (USDT-M or COIN-M).
type FuturesUserStream struct {
	Client   futClient
	DB       *db.Database
	Bus      *events.Bus
	Testnet  bool
	stopChan chan struct{}
	basePath string // "/ws" for usdt, "/dstream" for coin
}

type futClient interface {
	CreateListenKey(ctx context.Context) (string, error)
	KeepAliveListenKey(ctx context.Context, listenKey string) error
}

func NewFuturesUserStream(client futClient, database *db.Database, bus *events.Bus, testnet bool, coinMargin bool) *FuturesUserStream {
	base := "/ws"
	if coinMargin {
		base = "/dstream"
	}
	return &FuturesUserStream{
		Client:   client,
		DB:       database,
		Bus:      bus,
		Testnet:  testnet,
		stopChan: make(chan struct{}),
		basePath: base,
	}
}

// Start begins listening. It logs errors but keeps running until ctx done or Stop.
func (s *FuturesUserStream) Start(ctx context.Context) {
	if s.Client == nil || s.DB == nil {
		log.Println("futures user stream: client or DB not set; skipping")
		return
	}
	listenKey, err := s.Client.CreateListenKey(ctx)
	// FUTURES listen key uses different endpoints for keepalive; handled by client.
	if err != nil {
		log.Printf("futures user stream: create listen key error: %v", err)
		return
	}

	wsURL := s.buildStreamURL(listenKey)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Printf("futures user stream: ws dial error: %v", err)
		return
	}
	log.Printf("futures user stream started (testnet=%v, path=%s)", s.Testnet, s.basePath)

	// Keep alive ticker
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stopChan:
				return
			case <-ticker.C:
				if err := s.Client.KeepAliveListenKey(ctx, listenKey); err != nil {
					log.Printf("futures user stream keepalive error: %v", err)
				}
			}
		}
	}()

	// Reader
	go func() {
		defer conn.Close()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Printf("futures user stream read error: %v", err)
				return
			}
			s.handleMessage(ctx, msg)
		}
	}()
}

func (s *FuturesUserStream) Stop() {
	close(s.stopChan)
}

func (s *FuturesUserStream) buildStreamURL(listenKey string) string {
	host := "fstream.binance.com"
	if s.basePath == "/dstream" {
		host = "dstream.binance.com"
	}
	if s.Testnet {
		host = "testnet.binancefuture.com"
		if s.basePath == "/dstream" {
			host = "dstream.binancefuture.com"
		}
	}
	u := url.URL{Scheme: "wss", Host: host, Path: s.basePath + "/" + listenKey}
	return u.String()
}

func (s *FuturesUserStream) handleMessage(ctx context.Context, msg []byte) {
	// 與 Spot 一樣，e 可能不是單純字串，先用 RawMessage 解再判斷。
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(msg, &raw); err != nil {
		log.Printf("futures user stream parse error: %v", err)
		return
	}

	var eventType string
	if v, ok := raw["e"]; ok {
		if err := json.Unmarshal(v, &eventType); err != nil {
			log.Printf("futures user stream unknown event type payload: %s", string(v))
			return
		}
	} else {
		return
	}

	switch eventType {
	case "ORDER_TRADE_UPDATE":
		s.handleOrderTradeUpdate(ctx, msg)
	default:
		// ignore other events
	}
}

func (s *FuturesUserStream) handleOrderTradeUpdate(ctx context.Context, msg []byte) {
	var wrap struct {
		EventTime json.RawMessage `json:"E"`
		Data      struct {
			Symbol        string          `json:"s"`
			Side          string          `json:"S"`
			OrderType     string          `json:"o"`
			Status        string          `json:"X"`
			ExecutionType string          `json:"x"`
			OrderID       int64           `json:"i"`
			ClientOrderID string          `json:"c"`
			AvgPrice      string          `json:"ap"`
			LastPrice     string          `json:"L"`
			LastQty       string          `json:"l"`
			CumQty        string          `json:"z"`
			CumQuote      string          `json:"Z"`
			Commission    string          `json:"n"`
			CommissionAst string          `json:"N"`
			TradeTime     json.RawMessage `json:"T"`
			IsMaker       bool            `json:"m"`
		} `json:"o"`
	}
	if err := json.Unmarshal(msg, &wrap); err != nil {
		log.Printf("futures user stream: order update parse error: %v", err)
		return
	}

	// Only handle trade executions
	if strings.ToUpper(wrap.Data.ExecutionType) != "TRADE" {
		return
	}

	lastQty := toFloat(wrap.Data.LastQty)
	lastPrice := toFloat(wrap.Data.LastPrice)
	cumQty := toFloat(wrap.Data.CumQty)
	cumQuote := toFloat(wrap.Data.CumQuote)
	status := strings.ToUpper(wrap.Data.Status)

	fillPrice := lastPrice
	if fillPrice == 0 && cumQty > 0 {
		fillPrice = cumQuote / cumQty
	}

	// Update order fill
	if err := s.DB.UpdateOrderFill(ctx, wrap.Data.ClientOrderID, status, cumQty, fillPrice); err != nil {
		log.Printf("futures user stream: update order fill error: %v", err)
	}

	// Insert trade
	trade := db.Trade{
		ID:        uuid.NewString(),
		OrderID:   wrap.Data.ClientOrderID,
		Symbol:    wrap.Data.Symbol,
		Side:      wrap.Data.Side,
		Price:     fillPrice,
		Qty:       lastQty,
		Fee:       toFloat(wrap.Data.Commission),
		CreatedAt: time.Now(),
	}
	if err := s.DB.CreateTrade(ctx, trade); err != nil {
		log.Printf("futures user stream: create trade error: %v", err)
	}

	// Publish filled event
	if s.Bus != nil && status == "FILLED" {
		s.Bus.Publish(events.EventOrderFilled, struct {
			ID     string
			Symbol string
			Side   string
			Qty    float64
			Price  float64
		}{
			ID:     wrap.Data.ClientOrderID,
			Symbol: wrap.Data.Symbol,
			Side:   wrap.Data.Side,
			Qty:    lastQty,
			Price:  fillPrice,
		})
	}
}
