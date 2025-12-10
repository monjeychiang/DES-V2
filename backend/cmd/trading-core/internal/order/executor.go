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

// KeyManager interface for API key decryption.
type KeyManager interface {
	Decrypt(ciphertext string) (string, error)
}

// GatewayPool provides per-connection gateways (typically backed by gateway.Manager).
type GatewayPool interface {
	GetOrCreate(ctx context.Context, userID, connectionID string) (exchange.Gateway, error)
}

// Executor persists orders, sends them to an exchange gateway, and emits updates.
type Executor struct {
	DB      *db.Database
	Bus     *events.Bus
	Gateway exchange.Gateway // global fallback gateway

	Exchange     string // name/id for logging (fallback)
	Testnet      bool
	SkipExchange bool // when true, never call external gateways (used by dry-run wrapper)

	// Multi-user: KeyManager for decrypting API keys (optional)
	KeyManager KeyManager

	// Optional per-connection gateway pool (multi-user mode)
	Pool GatewayPool

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

// SetKeyManager sets the KeyManager for API key decryption.
func (e *Executor) SetKeyManager(km KeyManager) {
	e.KeyManager = km
}

// SetGatewayPool configures the per-connection gateway pool.
func (e *Executor) SetGatewayPool(pool GatewayPool) {
	e.Pool = pool
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
		UserID:             o.UserID,
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
			UserID:    o.UserID,
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

			// Check profit target (Phase 2 feature)
			e.checkProfitTarget(ctx, model.StrategyInstanceID)
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
	// Priority 1: Use ConnectionID directly if specified (multi-user mode)
	if o.ConnectionID != "" {
		gw, venue, ok := e.gatewayForConnection(ctx, o.UserID, o.ConnectionID)
		if ok {
			return gw, venue
		}
		// ConnectionID specified but not found - don't fallback
		log.Printf("executor: connection %s not found for user %s", o.ConnectionID, o.UserID)
		return nil, ""
	}

	// Priority 2: If the order is associated with a strategy, try to resolve a connection-specific gateway.
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

// gatewayForConnection returns a gateway for a specific connection, with user validation.
func (e *Executor) gatewayForConnection(ctx context.Context, userID, connID string) (exchange.Gateway, string, bool) {
	// Prefer external gateway pool when configured (multi-user mode).
	if e.Pool != nil {
		gw, err := e.Pool.GetOrCreate(ctx, userID, connID)
		if err != nil {
			log.Printf("executor: gateway pool error for connection %s (user %s): %v", connID, userID, err)
			return nil, "", false
		}
		return gw, "", true
	}

	if e.DB == nil {
		return nil, "", false
	}

	// Check cache first
	e.mu.RLock()
	gw, ok := e.connGateways[connID]
	e.mu.RUnlock()
	if ok && gw != nil {
		return gw, "", true // venue is unknown from cache, but gateway works
	}

	// Query connection with user ownership validation
	row := e.DB.DB.QueryRowContext(ctx, `
		SELECT id, exchange_type, 
		       COALESCE(api_key_encrypted, '') as api_key_encrypted,
		       COALESCE(api_secret_encrypted, '') as api_secret_encrypted,
		       api_key, api_secret
		FROM connections 
		WHERE id = ? AND user_id = ? AND is_active = 1
	`, connID, userID)

	var id, exchangeType, apiKeyEnc, apiSecretEnc, apiKey, apiSecret string
	if err := row.Scan(&id, &exchangeType, &apiKeyEnc, &apiSecretEnc, &apiKey, &apiSecret); err != nil {
		if err != sql.ErrNoRows {
			log.Printf("executor: failed to get connection %s: %v", connID, err)
		}
		return nil, "", false
	}

	// Use encrypted keys if available, otherwise fallback to plaintext
	finalAPIKey := apiKey
	finalAPISecret := apiSecret
	if apiKeyEnc != "" && e.KeyManager != nil {
		decryptedKey, err := e.KeyManager.Decrypt(apiKeyEnc)
		if err != nil {
			log.Printf("executor: failed to decrypt api_key for connection %s: %v", connID, err)
			return nil, "", false
		}
		decryptedSecret, err := e.KeyManager.Decrypt(apiSecretEnc)
		if err != nil {
			log.Printf("executor: failed to decrypt api_secret for connection %s: %v", connID, err)
			return nil, "", false
		}
		finalAPIKey = decryptedKey
		finalAPISecret = decryptedSecret
	} else if apiKeyEnc != "" && e.KeyManager == nil {
		log.Printf("executor: connection %s has encrypted keys but KeyManager not configured", connID)
		return nil, "", false
	}

	// Create gateway
	newGw := e.createGateway(exchangeType, finalAPIKey, finalAPISecret)
	if newGw == nil {
		return nil, "", false
	}

	// Cache it
	e.mu.Lock()
	e.connGateways[connID] = newGw
	e.mu.Unlock()

	return newGw, exchangeType, true
}

func (e *Executor) gatewayForStrategy(ctx context.Context, strategyID string) (exchange.Gateway, string, bool) {
	if e.DB == nil {
		return nil, "", false
	}

	// Lookup owning user and bound connection for this strategy.
	// We intentionally require a non-empty user_id so that we can
	// go through the same per-user connection validation and
	// gateway pool as manual orders.
	row := e.DB.DB.QueryRowContext(ctx, `
		SELECT 
			COALESCE(si.user_id, '')  AS user_id,
			COALESCE(si.connection_id, '') AS connection_id,
			COALESCE(c.exchange_type, '') AS exchange_type
		FROM strategy_instances si
		LEFT JOIN connections c ON si.connection_id = c.id AND c.is_active = 1
		WHERE si.id = ?
	`, strategyID)

	var userID, connID, exchangeType string
	if err := row.Scan(&userID, &connID, &exchangeType); err != nil {
		if err != sql.ErrNoRows {
			log.Printf("executor: failed to resolve connection for strategy %s: %v", strategyID, err)
		}
		return nil, "", false
	}

	if userID == "" || connID == "" {
		log.Printf("executor: strategy %s missing user_id or connection_id (user=%q, conn=%q)", strategyID, userID, connID)
		return nil, "", false
	}

	// Reuse the same per-user connection gateway resolution path as manual orders.
	gw, _, ok := e.gatewayForConnection(ctx, userID, connID)
	if !ok || gw == nil {
		log.Printf("executor: no gateway for strategy %s (user=%s, connection=%s)", strategyID, userID, connID)
		return nil, "", false
	}

	return gw, exchangeType, true
}

// createGateway creates an exchange.Gateway based on exchange type.
func (e *Executor) createGateway(exchangeType, apiKey, apiSecret string) exchange.Gateway {
	switch exchangeType {
	case "binance-spot":
		return exspot.New(exspot.Config{
			APIKey:    apiKey,
			APISecret: apiSecret,
			Testnet:   e.Testnet,
		})
	case "binance-usdtfut":
		return exfutusdt.NewClient(exfutusdt.Config{
			APIKey:    apiKey,
			APISecret: apiSecret,
			Testnet:   e.Testnet,
		})
	case "binance-coinfut":
		return exfutcoin.NewClient(exfutcoin.Config{
			APIKey:    apiKey,
			APISecret: apiSecret,
			Testnet:   e.Testnet,
		})
	default:
		log.Printf("executor: unsupported exchange_type %q", exchangeType)
		return nil
	}
}

// checkProfitTarget checks if the strategy has reached its profit target and stops it if so.
// Supports both USDT (absolute) and PERCENT (percentage of initial balance) targets.
func (e *Executor) checkProfitTarget(ctx context.Context, strategyID string) {
	// 1. Get strategy profit target settings
	var profitTarget float64
	var profitTargetType string
	err := e.DB.DB.QueryRowContext(ctx, `
		SELECT COALESCE(profit_target, 0), COALESCE(profit_target_type, 'USDT')
		FROM strategy_instances WHERE id = ?
	`, strategyID).Scan(&profitTarget, &profitTargetType)
	if err != nil || profitTarget <= 0 {
		// No profit target configured or error
		return
	}

	// 2. Get current realized PnL
	var realizedPnL float64
	err = e.DB.DB.QueryRowContext(ctx, `
		SELECT COALESCE(realized_pnl, 0) FROM strategy_positions WHERE strategy_instance_id = ?
	`, strategyID).Scan(&realizedPnL)
	if err != nil {
		return
	}

	// 3. Check if target reached
	targetReached := false
	switch profitTargetType {
	case "USDT":
		targetReached = realizedPnL >= profitTarget
	case "PERCENT":
		// For percent, we would need initial balance - simplified: treat as USDT for now
		// TODO: Implement percentage calculation based on initial capital
		targetReached = realizedPnL >= profitTarget
	}

	if !targetReached {
		return
	}

	// 4. Profit target reached - stop the strategy
	log.Printf("dYZ_ Profit target reached for strategy %s: %.2f %s (target: %.2f)",
		strategyID, realizedPnL, profitTargetType, profitTarget)

	// Update strategy status to STOPPED
	_, err = e.DB.DB.ExecContext(ctx, `
		UPDATE strategy_instances SET status = 'STOPPED', is_active = 0 WHERE id = ?
	`, strategyID)
	if err != nil {
		log.Printf("executor: failed to stop strategy after profit target: %v", err)
		return
	}

	// Publish event
	if e.Bus != nil {
		e.Bus.Publish(events.EventRiskAlert, map[string]any{
			"type":         "PROFIT_TARGET_REACHED",
			"strategy_id":  strategyID,
			"realized_pnl": realizedPnL,
			"target":       profitTarget,
			"target_type":  profitTargetType,
		})
	}
}
