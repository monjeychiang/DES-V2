package binance

import market "trading-core/pkg/market/binance"

// Aliases to keep compatibility with previous imports.
type Client = market.Client
type StreamClient = market.StreamClient
type MarketDataClient = market.MarketDataClient

func NewClient(apiKey, apiSecret string, testnet bool) *Client {
	return market.NewClient(apiKey, apiSecret, testnet)
}

func NewStreamClient(testnet bool) *StreamClient {
	return market.NewStreamClient(testnet)
}

func NewMarketDataClient(testnet bool) *MarketDataClient {
	return market.NewMarketDataClient(testnet)
}
