package order

import "time"

// Order represents a trading order intent.
type Order struct {
	ID                 string
	StrategyInstanceID string // Optional: ID of the strategy instance that generated this order
	Symbol             string
	Side               string
	Type               string // order type (MARKET, LIMIT, STOP_LOSS, etc.)
	Price              float64
	StopPrice          float64 // for stop-loss orders
	Qty                float64
	FilledQty          float64 // cumulative filled quantity
	TimeInForce        string  // GTC, IOC, FOK
	IcebergQty         float64 // for iceberg orders
	ReduceOnly         bool    // futures only-reduce
	PositionSide       string  // LONG/SHORT for hedge mode
	Market             string  // SPOT, USDT_FUTURES, COIN_FUTURES
	// Futures-specific
	WorkingType     string  // MARK_PRICE/CONTRACT_PRICE
	PriceProtect    bool    // price protection
	ActivationPrice float64 // trailing stop
	CallbackRate    float64 // trailing stop callback %
	Status          string  // NEW, SUBMITTED, ACCEPTED, PARTIALLY_FILLED, FILLED, CANCELLED, REJECTED, EXPIRED
	CreatedAt       time.Time
}

// IsFullyFilled checks if order is fully filled
func (o *Order) IsFullyFilled() bool {
	return o.FilledQty >= o.Qty
}

// IsPartiallyFilled checks if order is partially filled
func (o *Order) IsPartiallyFilled() bool {
	return o.FilledQty > 0 && o.FilledQty < o.Qty
}

// RemainingQty returns unfilled quantity
func (o *Order) RemainingQty() float64 {
	return o.Qty - o.FilledQty
}

// UpdateFill updates filled quantity and status
func (o *Order) UpdateFill(filledQty float64) {
	o.FilledQty = filledQty

	if o.IsFullyFilled() {
		o.Status = "FILLED"
	} else if o.IsPartiallyFilled() {
		o.Status = "PARTIALLY_FILLED"
	}
}
