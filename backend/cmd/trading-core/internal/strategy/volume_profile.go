package strategy

import (
	"encoding/json"
	"fmt"
)

// VolumeProfileStrategy trades based on volume patterns.
// High volume + price increase = strong bullish signal
// High volume + price decrease = strong bearish signal
// Low volume movements are ignored.
//
// This strategy requires volume data from Kline events.
type VolumeProfileStrategy struct {
	id               string
	symbol           string
	volumeMultiplier float64 // e.g., 2.0 means volume must be 2x average
	size             float64
	volumePeriod     int // period for average volume calculation

	volumes    []float64
	prices     []float64
	avgVolume  float64
	prevPrice  float64
	lastSignal string
}

// KlineData represents a full kline with volume.
type KlineData struct {
	Symbol string
	Close  float64
	Volume float64
}

// NewVolumeProfileStrategy creates a volume-based strategy.
func NewVolumeProfileStrategy(id, symbol string, volumeMultiplier, size float64, volumePeriod int) *VolumeProfileStrategy {
	return &VolumeProfileStrategy{
		id:               id,
		symbol:           symbol,
		volumeMultiplier: volumeMultiplier,
		size:             size,
		volumePeriod:     volumePeriod,
		volumes:          make([]float64, 0, volumePeriod),
		prices:           make([]float64, 0, 2),
		lastSignal:       "HOLD",
	}
}

func (s *VolumeProfileStrategy) ID() string {
	return s.id
}

func (s *VolumeProfileStrategy) Name() string {
	return fmt.Sprintf("VolumeProfile_%.1fx", s.volumeMultiplier)
}

// State defines the serializable state for VolumeProfileStrategy
type VolumeProfileState struct {
	LastSignal string `json:"last_signal"`
}

func (s *VolumeProfileStrategy) GetState() (json.RawMessage, error) {
	state := VolumeProfileState{
		LastSignal: s.lastSignal,
	}
	return json.Marshal(state)
}

func (s *VolumeProfileStrategy) SetState(data json.RawMessage) error {
	var state VolumeProfileState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	s.lastSignal = state.LastSignal
	return nil
}

// OnTick processes price-only updates (minimal info).
func (s *VolumeProfileStrategy) OnTick(symbol string, price float64, ind map[string]float64) (*Signal, error) {
	// This strategy needs volume data, price-only is insufficient
	return nil, nil
}

// OnKline processes full kline data with volume.
// This is a custom method for strategies needing more than just price.
func (s *VolumeProfileStrategy) OnKline(kline KlineData) *Signal {
	if kline.Symbol != s.symbol {
		return nil
	}

	// Update volume history
	s.volumes = append(s.volumes, kline.Volume)
	if len(s.volumes) > s.volumePeriod {
		s.volumes = s.volumes[1:]
	}

	// Update price history
	s.prices = append(s.prices, kline.Close)
	if len(s.prices) > 2 {
		s.prices = s.prices[1:]
	}

	// Need enough data
	if len(s.volumes) < s.volumePeriod || len(s.prices) < 2 {
		return nil
	}

	// Calculate average volume
	sum := 0.0
	for _, v := range s.volumes {
		sum += v
	}
	s.avgVolume = sum / float64(len(s.volumes))

	currentVolume := s.volumes[len(s.volumes)-1]
	currentPrice := s.prices[len(s.prices)-1]
	prevPrice := s.prices[len(s.prices)-2]

	// Volume must be significantly above average
	if currentVolume < s.avgVolume*s.volumeMultiplier {
		return nil
	}

	priceChange := currentPrice - prevPrice
	priceChangePercent := (priceChange / prevPrice) * 100

	var signal *Signal

	// High volume + price increase = BUY
	if priceChange > 0 {
		signal = &Signal{
			Action: "BUY",
			Symbol: s.symbol,
			Size:   s.size,
			Note: fmt.Sprintf("High volume breakout: vol=%.0f (%.1fx avg), price +%.2f%%",
				currentVolume, currentVolume/s.avgVolume, priceChangePercent),
		}
	}

	// High volume + price decrease = SELL
	if priceChange < 0 {
		signal = &Signal{
			Action: "SELL",
			Symbol: s.symbol,
			Size:   s.size,
			Note: fmt.Sprintf("High volume breakdown: vol=%.0f (%.1fx avg), price %.2f%%",
				currentVolume, currentVolume/s.avgVolume, priceChangePercent),
		}
	}

	// Only emit if signal changed
	if signal != nil && signal.Action != s.lastSignal {
		s.lastSignal = signal.Action
		return signal
	}

	return nil
}

// Usage example:
//
// // Subscribe to full kline events (not just price)
// klineStream, _ := bus.Subscribe(events.EventKlineUpdate, 100)
// volStrategy := strategy.NewVolumeProfileStrategy("BTCUSDT", 2.0, 0.001, 20)
//
// go func() {
//     for data := range klineStream {
//         kline := data.(strategy.KlineData)
//         signal := volStrategy.OnKline(kline)
//         if signal != nil {
//             bus.Publish(events.EventStrategySignal, *signal)
//         }
//     }
// }()
