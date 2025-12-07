package market

import marketpkg "trading-core/pkg/market/binance"

// LatestClose extracts the closing price from a kline.
func LatestClose(k marketpkg.Kline) float64 {
	return k.Close
}
