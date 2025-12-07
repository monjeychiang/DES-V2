package strategy

import (
	"encoding/json"
	"fmt"
	"math"
)

// RSIStrategy implements RSI (Relative Strength Index) overbought/oversold strategy.
// BUY when RSI < oversoldThreshold (default 30)
// SELL when RSI > overboughtThreshold (default 70)
type RSIStrategy struct {
	id                  string
	symbol              string
	period              int     // RSI period (typically 14)
	oversoldThreshold   float64 // e.g., 30
	overboughtThreshold float64 // e.g., 70
	size                float64 // order size

	prices     []float64
	gains      []float64
	losses     []float64
	rsi        float64
	prevSignal string
}

// NewRSIStrategy creates a new RSI strategy.
func NewRSIStrategy(id, symbol string, period int, oversold, overbought, size float64) *RSIStrategy {
	return &RSIStrategy{
		id:                  id,
		symbol:              symbol,
		period:              period,
		oversoldThreshold:   oversold,
		overboughtThreshold: overbought,
		size:                size,
		prices:              make([]float64, 0, period+1),
		gains:               make([]float64, 0, period),
		losses:              make([]float64, 0, period),
		prevSignal:          "HOLD",
	}
}

func (s *RSIStrategy) ID() string {
	return s.id
}

func (s *RSIStrategy) Name() string {
	return fmt.Sprintf("RSI_%d", s.period)
}

// State defines the serializable state for RSIStrategy
type RSIState struct {
	PrevSignal string  `json:"prev_signal"`
	RSI        float64 `json:"rsi"`
}

func (s *RSIStrategy) GetState() (json.RawMessage, error) {
	state := RSIState{
		PrevSignal: s.prevSignal,
		RSI:        s.rsi,
	}
	return json.Marshal(state)
}

func (s *RSIStrategy) SetState(data json.RawMessage) error {
	var state RSIState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	s.prevSignal = state.PrevSignal
	s.rsi = state.RSI
	return nil
}

func (s *RSIStrategy) OnTick(symbol string, price float64, ind map[string]float64) (*Signal, error) {
	if symbol != "" && symbol != s.symbol {
		return nil, nil
	}

	// Update price history
	s.prices = append(s.prices, price)
	if len(s.prices) > s.period+1 {
		s.prices = s.prices[1:]
	}

	// Need enough data to calculate RSI
	if len(s.prices) < s.period+1 {
		return nil, nil
	}

	// Calculate RSI
	s.calculateRSI()

	// Generate signals
	signal := s.generateSignal()

	if signal != nil && signal.Action != s.prevSignal {
		s.prevSignal = signal.Action
		return signal, nil
	}

	return nil, nil
}

func (s *RSIStrategy) calculateRSI() {
	// Calculate price changes
	s.gains = s.gains[:0]
	s.losses = s.losses[:0]

	for i := 1; i < len(s.prices); i++ {
		change := s.prices[i] - s.prices[i-1]
		if change > 0 {
			s.gains = append(s.gains, change)
			s.losses = append(s.losses, 0)
		} else {
			s.gains = append(s.gains, 0)
			s.losses = append(s.losses, math.Abs(change))
		}
	}

	// Calculate average gain and loss
	avgGain := 0.0
	avgLoss := 0.0

	for i := 0; i < len(s.gains) && i < s.period; i++ {
		avgGain += s.gains[i]
		avgLoss += s.losses[i]
	}

	avgGain /= float64(s.period)
	avgLoss /= float64(s.period)

	// Calculate RS and RSI
	if avgLoss == 0 {
		s.rsi = 100
		return
	}

	rs := avgGain / avgLoss
	s.rsi = 100 - (100 / (1 + rs))
}

func (s *RSIStrategy) generateSignal() *Signal {
	// Oversold: BUY signal
	if s.rsi < s.oversoldThreshold {
		return &Signal{
			Action: "BUY",
			Symbol: s.symbol,
			Size:   s.size,
			Note:   fmt.Sprintf("RSI oversold: %.2f < %.2f", s.rsi, s.oversoldThreshold),
		}
	}

	// Overbought: SELL signal
	if s.rsi > s.overboughtThreshold {
		return &Signal{
			Action: "SELL",
			Symbol: s.symbol,
			Size:   s.size,
			Note:   fmt.Sprintf("RSI overbought: %.2f > %.2f", s.rsi, s.overboughtThreshold),
		}
	}

	return &Signal{
		Action: "HOLD",
		Symbol: s.symbol,
		Size:   0,
		Note:   fmt.Sprintf("RSI neutral: %.2f", s.rsi),
	}
}
