package common

import "context"

// Gateway abstracts a trading venue.
type Gateway interface {
	SubmitOrder(ctx context.Context, req OrderRequest) (OrderResult, error)
	CancelOrder(ctx context.Context, symbol, exchangeOrderID string) error
}
