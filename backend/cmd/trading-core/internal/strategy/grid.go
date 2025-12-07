package strategy

import (
	"encoding/json"
	"fmt"
)

// GridStrategy is a minimal Go-native example strategy.
type GridStrategy struct {
	id           string
	symbol       string
	upperBound   float64
	lowerBound   float64
	orderSize    float64
	lastAction   string
	minStepRatio float64
}

func NewGridStrategy(id, symbol string, lower, upper, size float64) *GridStrategy {
	return &GridStrategy{
		id:           id,
		symbol:       symbol,
		upperBound:   upper,
		lowerBound:   lower,
		orderSize:    size,
		minStepRatio: 0.002, // 0.2% default step filter
	}
}

func (g *GridStrategy) ID() string {
	return g.id
}

func (g *GridStrategy) Name() string { return "grid_" + g.symbol }

// State defines the serializable state for GridStrategy
type GridState struct {
	LastAction string `json:"last_action"`
}

func (g *GridStrategy) GetState() (json.RawMessage, error) {
	state := GridState{
		LastAction: g.lastAction,
	}
	return json.Marshal(state)
}

func (g *GridStrategy) SetState(data json.RawMessage) error {
	var state GridState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	g.lastAction = state.LastAction
	return nil
}

// OnTick emits BUY near lower bound, SELL near upper bound.
func (g *GridStrategy) OnTick(symbol string, price float64, ind map[string]float64) (*Signal, error) {
	if symbol != "" && symbol != g.symbol {
		return nil, nil
	}
	if price <= 0 {
		return nil, nil
	}

	// simple debounce to avoid spamming when price hovers
	if g.lastAction == "BUY" && price > g.lowerBound*(1+g.minStepRatio) {
		g.lastAction = ""
	}
	if g.lastAction == "SELL" && price < g.upperBound*(1-g.minStepRatio) {
		g.lastAction = ""
	}

	if price <= g.lowerBound && g.lastAction != "BUY" {
		g.lastAction = "BUY"
		return &Signal{
			Action: "BUY",
			Symbol: g.symbol,
			Size:   g.orderSize,
			Note:   fmt.Sprintf("grid buy at %.2f", price),
		}, nil
	}

	if price >= g.upperBound && g.lastAction != "SELL" {
		g.lastAction = "SELL"
		return &Signal{
			Action: "SELL",
			Symbol: g.symbol,
			Size:   g.orderSize,
			Note:   fmt.Sprintf("grid sell at %.2f", price),
		}, nil
	}

	return nil, nil
}
