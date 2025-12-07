package indicators

import "sync"

// Engine maintains per-symbol price windows and calculates a few core indicators.
type Engine struct {
	mu      sync.Mutex
	prices  map[string][]float64
	window  int
	shortMA int
	longMA  int
	rsi     int
}

// NewEngine builds an indicator engine with default windows.
func NewEngine(shortMA, longMA, rsiPeriod, window int) *Engine {
	if window < longMA {
		window = longMA
	}
	return &Engine{
		prices:  make(map[string][]float64),
		window:  window,
		shortMA: shortMA,
		longMA:  longMA,
		rsi:     rsiPeriod,
	}
}

// Update ingests a new price and returns the latest computed values.
func (e *Engine) Update(symbol string, price float64) map[string]float64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	arr := append(e.prices[symbol], price)
	if len(arr) > e.window {
		arr = arr[len(arr)-e.window:]
	}
	e.prices[symbol] = arr

	values := map[string]float64{}
	values["sma_short"] = SMA(arr, e.shortMA)
	values["sma_long"] = SMA(arr, e.longMA)
	values["rsi"] = RSI(arr, e.rsi)

	return values
}
