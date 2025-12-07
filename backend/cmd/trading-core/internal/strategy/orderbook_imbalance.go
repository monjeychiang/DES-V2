package strategy

import (
	"encoding/json"
	"fmt"
)

// OrderBookImbalanceStrategy trades based on order book depth imbalance.
// When buy-side depth significantly exceeds sell-side, it signals buying pressure.
// When sell-side depth significantly exceeds buy-side, it signals selling pressure.
//
// This strategy requires DepthUpdate events from the EventBus.
type OrderBookImbalanceStrategy struct {
	id                 string
	symbol             string
	imbalanceThreshold float64 // e.g., 1.5 means 50% more on one side
	size               float64
	depthLevels        int // number of price levels to analyze

	bidDepth   float64 // total quantity on bid side
	askDepth   float64 // total quantity on ask side
	lastSignal string
}

// DepthData represents order book depth snapshot.
type DepthData struct {
	Symbol string
	Bids   [][]float64 // [price, quantity]
	Asks   [][]float64 // [price, quantity]
}

// NewOrderBookImbalanceStrategy creates a strategy based on order book depth.
func NewOrderBookImbalanceStrategy(id, symbol string, imbalanceThreshold, size float64, depthLevels int) *OrderBookImbalanceStrategy {
	return &OrderBookImbalanceStrategy{
		id:                 id,
		symbol:             symbol,
		imbalanceThreshold: imbalanceThreshold,
		size:               size,
		depthLevels:        depthLevels,
		lastSignal:         "HOLD",
	}
}

func (s *OrderBookImbalanceStrategy) ID() string {
	return s.id
}

func (s *OrderBookImbalanceStrategy) Name() string {
	return fmt.Sprintf("OrderBookImbalance_%.1f", s.imbalanceThreshold)
}

// State defines the serializable state for OrderBookImbalanceStrategy
type OrderBookImbalanceState struct {
	LastSignal string `json:"last_signal"`
}

func (s *OrderBookImbalanceStrategy) GetState() (json.RawMessage, error) {
	state := OrderBookImbalanceState{
		LastSignal: s.lastSignal,
	}
	return json.Marshal(state)
}

func (s *OrderBookImbalanceStrategy) SetState(data json.RawMessage) error {
	var state OrderBookImbalanceState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	s.lastSignal = state.LastSignal
	return nil
}

// OnTick processes price updates (can be used for reference price).
func (s *OrderBookImbalanceStrategy) OnTick(symbol string, price float64, ind map[string]float64) (*Signal, error) {
	// This strategy primarily relies on OnDepthUpdate
	// Price can be used for reference or validation
	return nil, nil
}

// OnDepthUpdate should be called when depth data arrives.
// This is a custom method that needs to be called by a specialized engine.
func (s *OrderBookImbalanceStrategy) OnDepthUpdate(depth DepthData) *Signal {
	if depth.Symbol != s.symbol {
		return nil
	}

	// Calculate total depth on each side
	s.bidDepth = s.calculateDepth(depth.Bids)
	s.askDepth = s.calculateDepth(depth.Asks)

	if s.bidDepth == 0 || s.askDepth == 0 {
		return nil
	}

	// Calculate imbalance ratio
	ratio := s.bidDepth / s.askDepth

	var signal *Signal

	// Strong buy pressure: bid depth >> ask depth
	if ratio >= s.imbalanceThreshold {
		signal = &Signal{
			Action: "BUY",
			Symbol: s.symbol,
			Size:   s.size,
			Note:   fmt.Sprintf("Order book imbalance: bid/ask = %.2f (%.0f/%.0f)", ratio, s.bidDepth, s.askDepth),
		}
	}

	// Strong sell pressure: ask depth >> bid depth
	if ratio <= (1.0 / s.imbalanceThreshold) {
		signal = &Signal{
			Action: "SELL",
			Symbol: s.symbol,
			Size:   s.size,
			Note:   fmt.Sprintf("Order book imbalance: ask/bid = %.2f (%.0f/%.0f)", 1.0/ratio, s.askDepth, s.bidDepth),
		}
	}

	// Only emit signal if it changed
	if signal != nil && signal.Action != s.lastSignal {
		s.lastSignal = signal.Action
		return signal
	}

	return nil
}

func (s *OrderBookImbalanceStrategy) calculateDepth(levels [][]float64) float64 {
	total := 0.0
	count := 0

	for _, level := range levels {
		if count >= s.depthLevels {
			break
		}
		if len(level) >= 2 {
			qty := level[1]
			total += qty
			count++
		}
	}

	return total
}

// Usage example in a separate engine or handler:
//
// depthStream, _ := bus.Subscribe(events.EventDepthUpdate, 100)
// obStrategy := strategy.NewOrderBookImbalanceStrategy("BTCUSDT", 1.5, 0.001, 10)
//
// go func() {
//     for data := range depthStream {
//         depth := data.(strategy.DepthData)
//         signal := obStrategy.OnDepthUpdate(depth)
//         if signal != nil {
//             bus.Publish(events.EventStrategySignal, *signal)
//         }
//     }
// }()
