package order

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"trading-core/internal/events"
	"trading-core/pkg/db"
	exfutcoin "trading-core/pkg/exchanges/binance/futures_coin"
	exfutusdt "trading-core/pkg/exchanges/binance/futures_usdt"
	exspot "trading-core/pkg/exchanges/binance/spot"
	exchange "trading-core/pkg/exchanges/common"

	"github.com/google/uuid"
)

// Executor persists orders, sends them to an exchange gateway, and emits updates.
type Executor struct {
	DB      *db.Database
	Bus     *events.Bus
	Gateway exchange.Gateway // global fallback gateway

	Exchange     string // name/id for logging (fallback)
	Testnet      bool
	SkipExchange bool // when true, never call external gateways (used by dry-run wrapper)

	mu           sync.RWMutex
	connGateways map[string]exchange.Gateway // connection_id -> gateway
}

func NewExecutor(database *db.Database, bus *events.Bus, gw exchange.Gateway, venue string, testnet bool) *Executor {
	return &Executor{
		DB:           database,
		Bus:          bus,
		Gateway:      gw,
		Exchange:     venue,
		Testnet:      testnet,
		connGateways: make(map[string]exchange.Gateway),
	}
}

func (e *Executor) Handle(ctx context.Context, o Order) error {
	if e.DB == nil {
		err := fmt.Errorf("executor: DB not configured")
		log.Println(err)
		return err
	}

	// Build exchange request with all advanced parameters
	req := exchange.OrderRequest{
		Symbol:       o.Symbol,
		Side:         exchange.Side(o.Side),
		Type:         exchange.OrderType(o.Type), // use actual order type from Order
		Qty:          o.Qty,
		Price:        o.Price,
		StopPrice:    o.StopPrice,
		TimeInForce:  exchange.TimeInForce(o.TimeInForce),
		IcebergQty:   o.IcebergQty,
		ClientID:     o.ID,
		ReduceOnly:   o.ReduceOnly,
		PositionSide: o.PositionSide,
		Market:       exchange.MarketType(o.Market), // route to correct market
		// Futures-specific
		WorkingType:     o.WorkingType,
		PriceProtect:    o.PriceProtect,
		ActivationPrice: o.ActivationPrice,
		CallbackRate:    o.CallbackRate,
	}

	// Publish submitted event
	if e.Bus != nil {
		e.Bus.Publish(events.EventOrderSubmitted, o)
	}

	// Send to exchange (if configured)
	var exchID string
	status := "NEW"
	filled := false
	var execErr error

	if e.SkipExchange {
		log.Printf("executor: SkipExchange enabled, not sending order %s to external gateway", o.ID)
	} else {
		gw, venue := e.gatewayForOrder(ctx, o)
		if gw != nil {
			res, err := gw.SubmitOrder(ctx, req)
			if err != nil {
				log.Printf("executor: submit to %s failed: %v", venue, err)
				status = "REJECTED"
				execErr = err
				if e.Bus != nil {
					e.Bus.Publish(events.EventOrderRejected, err.Error())
				}
			} else {
				exchID = res.ExchangeOrderID
				status = string(res.Status)
				if e.Bus != nil {
					e.Bus.Publish(events.EventOrderAccepted, o)
					if res.Status == exchange.StatusFilled {
						e.Bus.Publish(events.EventOrderFilled, o)
						filled = true
					}
				}
			}
		} else {
			log.Printf("executor: no gateway resolved for order %s, marking as REJECTED (no external send)", o.ID)
			status = "REJECTED"
			execErr = fmt.Errorf("no gateway resolved")
			if e.Bus != nil {
				e.Bus.Publish(events.EventOrderRejected, "no gateway for order")
			}
		}
	}

	model := db.Order{
		ID:                 o.ID,
		StrategyInstanceID: o.StrategyInstanceID,
		Symbol:             o.Symbol,
		Side:               o.Side,
		Price:              o.Price,
		Qty:                o.Qty,
		Status:             status,
		CreatedAt:          time.Now(),
	}
	if err := e.DB.CreateOrder(ctx, model); err != nil {
		log.Printf("executor: store order error: %v", err)
		return err
	}

	// If filled, store a trade row (price may be 0 for market; will be reconciled later)
	if filled {
		trade := db.Trade{
			ID:        uuid.NewString(),
			OrderID:   model.ID,
			Symbol:    model.Symbol,
			Side:      model.Side,
			Price:     model.Price,
			Qty:       model.Qty,
			Fee:       0,
			CreatedAt: time.Now(),
		}
		if err := e.DB.CreateTrade(ctx, trade); err != nil {
			log.Printf("executor: store trade error: %v", err)
		}

		// Update Strategy Position
		if model.StrategyInstanceID != "" {
			if err := e.DB.UpdateStrategyPosition(ctx, model.StrategyInstanceID, model.Symbol, model.Side, model.Qty, model.Price); err != nil {
				log.Printf("executor: update strategy position error: %v", err)
			}
		}
	}

	log.Printf("executor: stored order %s %s qty=%.6f exch_id=%s", model.Symbol, model.Side, model.Qty, exchID)

	if e.Bus != nil {
		e.Bus.Publish(events.EventOrderUpdate, model)
	}

	return execErr
}

// gatewayForOrder picks an exchange gateway for the given order based on its strategy binding.
// It falls back to the global gateway when no per-connection binding is found.
func (e *Executor) gatewayForOrder(ctx context.Context, o Order) (exchange.Gateway, string) {
	// If the order is associated with a strategy, try to resolve a connection-specific gateway.
	if o.StrategyInstanceID != "" {
		gw, venue, ok := e.gatewayForStrategy(ctx, o.StrategyInstanceID)
		if ok {
			return gw, venue
		}
		// No gateway for this strategy and no fallback: do not hit exchange.
		return nil, ""
	}

	// Fallback to global gateway if present.
	if e.Gateway != nil {
		return e.Gateway, e.Exchange
	}
	return nil, ""
}

func (e *Executor) gatewayForStrategy(ctx context.Context, strategyID string) (exchange.Gateway, string, bool) {
	if e.DB == nil {
		return nil, "", false
	}

	// Lookup bound connection for this strategy.
	row := e.DB.DB.QueryRowContext(ctx, `
		SELECT c.id, c.exchange_type, c.api_key, c.api_secret
		FROM strategy_instances si
		JOIN connections c ON si.connection_id = c.id
		WHERE si.id = ? AND c.is_active = 1
	`, strategyID)

	var connID, exchangeType, apiKey, apiSecret string
	if err := row.Scan(&connID, &exchangeType, &apiKey, &apiSecret); err != nil {
		if err != sql.ErrNoRows {
			log.Printf("executor: failed to resolve connection for strategy %s: %v", strategyID, err)
		}
		return nil, "", false
	}

	// Reuse cached gateway if available.
	e.mu.RLock()
	gw, ok := e.connGateways[connID]
	e.mu.RUnlock()
	if ok && gw != nil {
		return gw, exchangeType, true
	}

	// Create a new gateway for this connection.
	var newGw exchange.Gateway
	switch exchangeType {
	case "binance-spot":
		newGw = exspot.New(exspot.Config{
			APIKey:    apiKey,
			APISecret: apiSecret,
			Testnet:   e.Testnet,
		})
	case "binance-usdtfut":
		newGw = exfutusdt.NewClient(exfutusdt.Config{
			APIKey:    apiKey,
			APISecret: apiSecret,
			Testnet:   e.Testnet,
		})
	case "binance-coinfut":
		newGw = exfutcoin.NewClient(exfutcoin.Config{
			APIKey:    apiKey,
			APISecret: apiSecret,
			Testnet:   e.Testnet,
		})
	default:
		log.Printf("executor: unsupported exchange_type %q for connection %s", exchangeType, connID)
		return nil, "", false
	}

	if newGw == nil {
		return nil, "", false
	}

	e.mu.Lock()
	e.connGateways[connID] = newGw
	e.mu.Unlock()

	return newGw, exchangeType, true
}
