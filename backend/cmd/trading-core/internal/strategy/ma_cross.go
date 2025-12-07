package strategy

import (
	"encoding/json"
	"fmt"
)

// MACrossStrategy implements a simple moving average crossover strategy.
// Generates BUY signal when fast MA crosses above slow MA (golden cross).
// Generates SELL signal when fast MA crosses below slow MA (death cross).
type MACrossStrategy struct {
	id         string
	symbol     string
	fastPeriod int     // e.g., 10
	slowPeriod int     // e.g., 30
	size       float64 // order size

	fastMA     float64
	slowMA     float64
	prices     []float64
	prevSignal string // track last signal to avoid repeats
}

// NewMACrossStrategy creates a new MA cross strategy.
func NewMACrossStrategy(id, symbol string, fastPeriod, slowPeriod int, size float64) *MACrossStrategy {
	return &MACrossStrategy{
		id:         id,
		symbol:     symbol,
		fastPeriod: fastPeriod,
		slowPeriod: slowPeriod,
		size:       size,
		prices:     make([]float64, 0, slowPeriod),
		prevSignal: "HOLD",
	}
}

func (s *MACrossStrategy) ID() string {
	return s.id
}

func (s *MACrossStrategy) Name() string {
	return fmt.Sprintf("MA_Cross_%d_%d", s.fastPeriod, s.slowPeriod)
}

// State defines the serializable state for MACrossStrategy
type MACrossState struct {
	PrevSignal string  `json:"prev_signal"`
	FastMA     float64 `json:"fast_ma"`
	SlowMA     float64 `json:"slow_ma"`
}

func (s *MACrossStrategy) GetState() (json.RawMessage, error) {
	state := MACrossState{
		PrevSignal: s.prevSignal,
		FastMA:     s.fastMA,
		SlowMA:     s.slowMA,
	}
	return json.Marshal(state)
}

func (s *MACrossStrategy) SetState(data json.RawMessage) error {
	var state MACrossState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	s.prevSignal = state.PrevSignal
	s.fastMA = state.FastMA
	s.slowMA = state.SlowMA
	return nil
}

func (s *MACrossStrategy) OnTick(symbol string, price float64, ind map[string]float64) (*Signal, error) {
	// Only trade configured symbol
	if symbol != "" && symbol != s.symbol {
		return nil, nil
	}

	// Update price history
	s.prices = append(s.prices, price)
	if len(s.prices) > s.slowPeriod {
		s.prices = s.prices[1:]
	}

	// Need enough data for slow MA
	if len(s.prices) < s.slowPeriod {
		return nil, nil
	}

	// Calculate MAs
	oldFastMA := s.fastMA
	oldSlowMA := s.slowMA

	s.fastMA = calculateMA(s.prices, s.fastPeriod)
	s.slowMA = calculateMA(s.prices, s.slowPeriod)

	// Detect crossover
	signal := s.detectCross(oldFastMA, oldSlowMA)

	if signal != nil && signal.Action != s.prevSignal {
		s.prevSignal = signal.Action
		return signal, nil
	}

	return nil, nil
}

func (s *MACrossStrategy) detectCross(oldFast, oldSlow float64) *Signal {
	// Golden cross: fast MA crosses above slow MA
	if oldFast <= oldSlow && s.fastMA > s.slowMA {
		return &Signal{
			Action: "BUY",
			Symbol: s.symbol,
			Size:   s.size,
			Note:   fmt.Sprintf("Golden cross: MA%d(%.2f) > MA%d(%.2f)", s.fastPeriod, s.fastMA, s.slowPeriod, s.slowMA),
		}
	}

	// Death cross: fast MA crosses below slow MA
	if oldFast >= oldSlow && s.fastMA < s.slowMA {
		return &Signal{
			Action: "SELL",
			Symbol: s.symbol,
			Size:   s.size,
			Note:   fmt.Sprintf("Death cross: MA%d(%.2f) < MA%d(%.2f)", s.fastPeriod, s.fastMA, s.slowPeriod, s.slowMA),
		}
	}

	return nil
}

// calculateMA calculates simple moving average for the last n periods.
func calculateMA(prices []float64, period int) float64 {
	if len(prices) < period {
		return 0
	}

	sum := 0.0
	start := len(prices) - period
	for i := start; i < len(prices); i++ {
		sum += prices[i]
	}

	return sum / float64(period)
}
