package spot

import (
	"context"
	"strconv"
	"trading-core/internal/balance"
	"trading-core/internal/reconciliation"
)

// GetBalance implements balance.ExchangeClient interface
func (c *Client) GetBalance(ctx context.Context) (balance.Balance, error) {
	info, err := c.GetAccountInfo(ctx)
	if err != nil {
		return balance.Balance{}, err
	}

	// Sum all USDT balances (or you can specify which asset)
	var total, available, locked float64
	for _, bal := range info.Balances {
		if bal.Asset == "USDT" || bal.Asset == "BUSD" {
			free, _ := strconv.ParseFloat(bal.Free, 64)
			lock, _ := strconv.ParseFloat(bal.Locked, 64)
			total += free + lock
			available += free
			locked += lock
		}
	}

	return balance.Balance{
		Total:     total,
		Available: available,
		Locked:    locked,
	}, nil
}

// GetPositions implements reconciliation.ExchangeClient interface
// Note: Spot trading doesn't have positions like futures, this returns empty for compatibility
func (c *Client) GetPositions(ctx context.Context) (map[string]reconciliation.Position, error) {
	// Spot doesn't have positions, return empty map
	// If you want to track "positions" in spot, you'd need to implement based on balances
	return make(map[string]reconciliation.Position), nil
}
