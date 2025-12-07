package strategy

import (
	"encoding/json"
	"fmt"
	"math"
)

// BollingerStrategy implements Bollinger Bands breakout strategy.
// BUY when price touches/breaks below lower band
// SELL when price touches/breaks above upper band
type BollingerStrategy struct {
	id        string
	symbol    string
	period    int     // Period for MA and std dev (typically 20)
	numStdDev float64 // Number of standard deviations (typically 2.0)
	size      float64 // Order size

	prices     []float64
	middleBand float64 // SMA
	upperBand  float64 // SMA + numStdDev * stdDev
	lowerBand  float64 // SMA - numStdDev * stdDev
	prevSignal string
}

// NewBollingerStrategy creates a new Bollinger Bands strategy.
func NewBollingerStrategy(id, symbol string, period int, numStdDev, size float64) *BollingerStrategy {
	return &BollingerStrategy{
		id:         id,
		symbol:     symbol,
		period:     period,
		numStdDev:  numStdDev,
		size:       size,
		prices:     make([]float64, 0, period),
		prevSignal: "HOLD",
	}
}

func (s *BollingerStrategy) ID() string {
	return s.id
}

func (s *BollingerStrategy) Name() string {
	return fmt.Sprintf("Bollinger_%d_%.1f", s.period, s.numStdDev)
}

// State defines the serializable state for BollingerStrategy
type BollingerState struct {
	PrevSignal string  `json:"prev_signal"`
	MiddleBand float64 `json:"middle_band"`
	UpperBand  float64 `json:"upper_band"`
	LowerBand  float64 `json:"lower_band"`
}

func (s *BollingerStrategy) GetState() (json.RawMessage, error) {
	state := BollingerState{
		PrevSignal: s.prevSignal,
		MiddleBand: s.middleBand,
		UpperBand:  s.upperBand,
		LowerBand:  s.lowerBand,
	}
	return json.Marshal(state)
}

func (s *BollingerStrategy) SetState(data json.RawMessage) error {
	var state BollingerState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	s.prevSignal = state.PrevSignal
	s.middleBand = state.MiddleBand
	s.upperBand = state.UpperBand
	s.lowerBand = state.LowerBand
	return nil
}

func (s *BollingerStrategy) OnTick(symbol string, price float64, ind map[string]float64) (*Signal, error) {
	if symbol != "" && symbol != s.symbol {
		return nil, nil
	}

	// Update price history
	s.prices = append(s.prices, price)
	if len(s.prices) > s.period {
		s.prices = s.prices[1:]
	}

	// Need enough data
	if len(s.prices) < s.period {
		return nil, nil
	}

	// Calculate Bollinger Bands
	s.calculateBands()

	// Generate signal
	signal := s.generateSignal(price)

	if signal != nil && signal.Action != s.prevSignal {
		s.prevSignal = signal.Action
		return signal, nil
	}

	return nil, nil
}

func (s *BollingerStrategy) calculateBands() {
	// Calculate middle band (SMA)
	sum := 0.0
	for _, p := range s.prices {
		sum += p
	}
	s.middleBand = sum / float64(len(s.prices))

	// Calculate standard deviation
	variance := 0.0
	for _, p := range s.prices {
		diff := p - s.middleBand
		variance += diff * diff
	}
	stdDev := math.Sqrt(variance / float64(len(s.prices)))

	// Calculate upper and lower bands
	s.upperBand = s.middleBand + (s.numStdDev * stdDev)
	s.lowerBand = s.middleBand - (s.numStdDev * stdDev)
}

func (s *BollingerStrategy) generateSignal(price float64) *Signal {
	// Price touches/breaks lower band: BUY (oversold)
	if price <= s.lowerBand {
		return &Signal{
			Action: "BUY",
			Symbol: s.symbol,
			Size:   s.size,
			Note:   fmt.Sprintf("BB lower breakout: price %.2f <= lower %.2f", price, s.lowerBand),
		}
	}

	// Price touches/breaks upper band: SELL (overbought)
	if price >= s.upperBand {
		return &Signal{
			Action: "SELL",
			Symbol: s.symbol,
			Size:   s.size,
			Note:   fmt.Sprintf("BB upper breakout: price %.2f >= upper %.2f", price, s.upperBand),
		}
	}

	// Price in middle zone
	return &Signal{
		Action: "HOLD",
		Symbol: s.symbol,
		Size:   0,
		Note:   fmt.Sprintf("BB middle: %.2f < price %.2f < %.2f", s.lowerBand, price, s.upperBand),
	}
}
