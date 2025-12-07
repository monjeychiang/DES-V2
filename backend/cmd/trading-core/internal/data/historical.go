package data

import (
	"context"
	"strconv"

	"trading-core/pkg/binance"
)

// Kline represents a single candlestick.
type Kline struct {
	OpenTime int64
	Open     float64
	High     float64
	Low      float64
	Close    float64
	Volume   float64
}

// HistoricalDataService fetches historical market data.
type HistoricalDataService struct {
	client *binance.MarketDataClient
}

// NewHistoricalDataService creates a new service instance.
func NewHistoricalDataService(testnet bool) *HistoricalDataService {
	return &HistoricalDataService{
		client: binance.NewMarketDataClient(testnet),
	}
}

// GetKlines fetches klines for a symbol and interval.
func (s *HistoricalDataService) GetKlines(ctx context.Context, symbol, interval string, limit int) ([]Kline, error) {
	rawKlines, err := s.client.Klines(ctx, symbol, interval, limit)
	if err != nil {
		return nil, err
	}

	klines := make([]Kline, 0, len(rawKlines))
	for _, raw := range rawKlines {
		k, ok := raw.([]interface{})
		if !ok || len(k) < 6 {
			continue
		}

		openTime := int64(k[0].(float64))
		open, _ := strconv.ParseFloat(k[1].(string), 64)
		high, _ := strconv.ParseFloat(k[2].(string), 64)
		low, _ := strconv.ParseFloat(k[3].(string), 64)
		closePrice, _ := strconv.ParseFloat(k[4].(string), 64)
		volume, _ := strconv.ParseFloat(k[5].(string), 64)

		klines = append(klines, Kline{
			OpenTime: openTime,
			Open:     open,
			High:     high,
			Low:      low,
			Close:    closePrice,
			Volume:   volume,
		})
	}

	return klines, nil
}
