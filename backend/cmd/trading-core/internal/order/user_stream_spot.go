package order

import (
	"context"
	"encoding/json"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"trading-core/internal/events"
	"trading-core/pkg/db"
	exspot "trading-core/pkg/exchanges/binance/spot"
)

// SpotUserStream listens to Binance Spot user data stream for real fills.
type SpotUserStream struct {
	Client   *exspot.Client
	DB       *db.Database
	Bus      *events.Bus
	Testnet  bool
	stopChan chan struct{}
}

func NewSpotUserStream(client *exspot.Client, database *db.Database, bus *events.Bus, testnet bool) *SpotUserStream {
	return &SpotUserStream{
		Client:   client,
		DB:       database,
		Bus:      bus,
		Testnet:  testnet,
		stopChan: make(chan struct{}),
	}
}

// Start begins listening. It will log errors but not return them.
func (s *SpotUserStream) Start(ctx context.Context) {
	if s.Client == nil || s.DB == nil {
		log.Println("spot user stream: client or DB not set; skipping")
		return
	}

	listenKey, err := s.Client.CreateListenKey(ctx)
	if err != nil {
		log.Printf("spot user stream: create listen key error: %v", err)
		return
	}

	wsURL := buildStreamURL(s.Testnet, listenKey)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Printf("spot user stream: ws dial error: %v", err)
		return
	}
	log.Printf("spot user stream started (testnet=%v)", s.Testnet)

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
					log.Printf("spot user stream keepalive error: %v", err)
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
				log.Printf("spot user stream read error: %v", err)
				return
			}
			s.handleMessage(ctx, msg)
		}
	}()
}

func (s *SpotUserStream) Stop() {
	close(s.stopChan)
}

func buildStreamURL(testnet bool, listenKey string) string {
	host := "stream.binance.com:9443"
	if testnet {
		host = "testnet.binance.vision"
	}
	u := url.URL{Scheme: "wss", Host: host, Path: "/ws/" + listenKey}
	return u.String()
}

func (s *SpotUserStream) handleMessage(ctx context.Context, msg []byte) {
	// Binance 有時會在某些訊息裡把 e 以數字型別回傳，直接綁定成 string 會造成解碼錯誤。
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(msg, &raw); err != nil {
		log.Printf("spot user stream parse error: %v", err)
		return
	}

	var eventType string
	if v, ok := raw["e"]; ok {
		// 優先嘗試解成字串；如果不是字串就忽略該訊息。
		if err := json.Unmarshal(v, &eventType); err != nil {
			log.Printf("spot user stream unknown event type payload: %s", string(v))
			return
		}
	} else {
		return
	}

	switch eventType {
	case "executionReport":
		s.handleExecutionReport(ctx, msg)
	default:
		// ignore other events
	}
}

func (s *SpotUserStream) handleExecutionReport(ctx context.Context, msg []byte) {
	var rep struct {
		Symbol          string `json:"s"`
		Side            string `json:"S"`
		OrderType       string `json:"o"`
		Status          string `json:"X"`
		ExecutionType   string `json:"x"`
		OrderID         int64  `json:"i"`
		ClientOrderID   string `json:"c"`
		Price           string `json:"p"`
		Qty             string `json:"q"`
		LastQty         string `json:"l"`
		LastPrice       string `json:"L"`
		CumulativeQty   string `json:"z"`
		CumulativeQuote string `json:"Z"`
		Commission      string `json:"n"`
		CommissionAsset string `json:"N"`
		TradeTime       int64  `json:"T"`
		IsMaker         bool   `json:"m"`
	}
	if err := json.Unmarshal(msg, &rep); err != nil {
		log.Printf("spot user stream: execution report parse error: %v", err)
		return
	}

	// Only handle trade executions
	if rep.ExecutionType != "TRADE" {
		return
	}

	lastQty := toFloat(rep.LastQty)
	lastPrice := toFloat(rep.LastPrice)
	cumQty := toFloat(rep.CumulativeQty)
	cumQuote := toFloat(rep.CumulativeQuote)
	status := strings.ToUpper(rep.Status)

	// Update order status/fill in DB
	fillPrice := lastPrice
	if fillPrice == 0 && cumQty > 0 {
		fillPrice = cumQuote / cumQty
	}
	if err := s.DB.UpdateOrderFill(ctx, rep.ClientOrderID, status, cumQty, fillPrice); err != nil {
		log.Printf("spot user stream: update order fill error: %v", err)
	}

	// Insert trade row
	trade := db.Trade{
		ID:        uuid.NewString(),
		OrderID:   rep.ClientOrderID,
		Symbol:    rep.Symbol,
		Side:      rep.Side,
		Price:     fillPrice,
		Qty:       lastQty,
		Fee:       toFloat(rep.Commission),
		CreatedAt: time.Now(),
	}
	if err := s.DB.CreateTrade(ctx, trade); err != nil {
		log.Printf("spot user stream: create trade error: %v", err)
	}

	// Publish filled event with updated info
	if s.Bus != nil && status == "FILLED" {
		s.Bus.Publish(events.EventOrderFilled, struct {
			ID     string
			Symbol string
			Side   string
			Qty    float64
			Price  float64
		}{
			ID:     rep.ClientOrderID,
			Symbol: rep.Symbol,
			Side:   rep.Side,
			Qty:    lastQty,
			Price:  lastPrice,
		})
	}
}

func toFloat(v string) float64 {
	f, _ := strconv.ParseFloat(v, 64)
	return f
}
