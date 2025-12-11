package risk

import "testing"

// Ensures UpdateMetrics does not double-subtract fees from already net PnL for
// either wins or losses.
func TestUpdateMetricsUsesNetPnL(t *testing.T) {
	tests := []struct {
		name              string
		trade             TradeResult
		wantDailyLosses   float64
		wantMaxDrawdown   float64
		wantMaxProfitGain float64
	}{
		{
			name: "profit",
			trade: TradeResult{
				Symbol: "BTCUSDT",
				Side:   "SELL",
				Size:   0.1,
				Price:  50000,
				PnL:    120.5, // already net of fee
				Fee:    5.5,
			},
			wantMaxProfitGain: 120.5,
		},
		{
			name: "loss",
			trade: TradeResult{
				Symbol: "ETHUSDT",
				Side:   "BUY",
				Size:   2,
				Price:  3000,
				PnL:    -42.75, // already net of fee
				Fee:    1.25,
			},
			wantDailyLosses: 42.75,
			wantMaxDrawdown: 42.75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewInMemory(DefaultConfig())

			if err := mgr.UpdateMetrics(tt.trade); err != nil {
				t.Fatalf("UpdateMetrics returned error: %v", err)
			}

			metrics := mgr.GetMetrics()
			if metrics.DailyPnL != tt.trade.PnL {
				t.Fatalf("DailyPnL=%v, expected %v", metrics.DailyPnL, tt.trade.PnL)
			}
			if metrics.TotalRealizedPnL != tt.trade.PnL {
				t.Fatalf("TotalRealizedPnL=%v, expected %v", metrics.TotalRealizedPnL, tt.trade.PnL)
			}
			if metrics.DailyLosses != tt.wantDailyLosses {
				t.Fatalf("DailyLosses=%v, expected %v", metrics.DailyLosses, tt.wantDailyLosses)
			}
			if metrics.MaxDrawdown != tt.wantMaxDrawdown {
				t.Fatalf("MaxDrawdown=%v, expected %v", metrics.MaxDrawdown, tt.wantMaxDrawdown)
			}
			if metrics.MaxProfit != tt.wantMaxProfitGain {
				t.Fatalf("MaxProfit=%v, expected %v", metrics.MaxProfit, tt.wantMaxProfitGain)
			}
			if metrics.DailyTrades != 1 {
				t.Fatalf("DailyTrades=%v, expected 1", metrics.DailyTrades)
			}
		})
	}
}
