package order

import (
	"math"
	"time"

	"trading-core/internal/events"
)

// EmitPositionUpdate publishes a position change event (hook point for DB/state listeners).
func EmitPositionUpdate(bus *events.Bus, sym, side string, qty, price float64) {
	if bus == nil {
		return
	}
	bus.Publish(events.EventPositionChange, struct {
		Symbol string
		Side   string
		Qty    float64
		Price  float64
		Time   time.Time
	}{
		Symbol: sym,
		Side:   side,
		Qty:    qty,
		Price:  price,
		Time:   time.Now(),
	})
}

// CalculatePnL is a helper to compute simple realized PnL for flatting trades.
func CalculatePnL(side string, qty, entry, exit float64, fee float64) float64 {
	q := math.Abs(qty)
	if q == 0 {
		return 0
	}
	var pnl float64
	if side == "BUY" {
		pnl = (exit - entry) * q
	} else {
		pnl = (entry - exit) * q
	}
	return pnl - fee
}
