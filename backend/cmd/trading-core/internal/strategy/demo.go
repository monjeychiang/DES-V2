package strategy

import "encoding/json"

// DemoStrategy is a simple momentum-like strategy for exercising order flow
// with mock data. It emits BUY when price jumps up and SELL when it jumps down
// by a small threshold.
type DemoStrategy struct {
	id        string
	symbol    string
	size      float64
	threshold float64
	lastPrice float64
}

func NewDemoStrategy(id, symbol string, size, threshold float64) *DemoStrategy {
	if threshold <= 0 {
		threshold = 0.001 // 0.1%
	}
	if size <= 0 {
		size = 0.001
	}
	return &DemoStrategy{
		id:        id,
		symbol:    symbol,
		size:      size,
		threshold: threshold,
	}
}

func (d *DemoStrategy) ID() string {
	return d.id
}

func (d *DemoStrategy) Name() string { return "demo_" + d.symbol }

// State defines the serializable state for DemoStrategy
type DemoState struct {
	LastPrice float64 `json:"last_price"`
}

func (d *DemoStrategy) GetState() (json.RawMessage, error) {
	state := DemoState{
		LastPrice: d.lastPrice,
	}
	return json.Marshal(state)
}

func (d *DemoStrategy) SetState(data json.RawMessage) error {
	var state DemoState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	d.lastPrice = state.LastPrice
	return nil
}

func (d *DemoStrategy) OnTick(symbol string, price float64, ind map[string]float64) (*Signal, error) {
	if symbol != "" && symbol != d.symbol {
		return nil, nil
	}
	if price <= 0 {
		return nil, nil
	}
	if d.lastPrice == 0 {
		d.lastPrice = price
		return nil, nil
	}

	change := (price - d.lastPrice) / d.lastPrice
	d.lastPrice = price

	if change >= d.threshold {
		return &Signal{
			Action: "BUY",
			Symbol: d.symbol,
			Size:   d.size,
			Note:   "demo momentum buy",
		}, nil
	}
	if change <= -d.threshold {
		return &Signal{
			Action: "SELL",
			Symbol: d.symbol,
			Size:   d.size,
			Note:   "demo momentum sell",
		}, nil
	}

	return nil, nil
}
