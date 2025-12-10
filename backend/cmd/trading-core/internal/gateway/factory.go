package gateway

import (
	"fmt"

	"trading-core/pkg/db"
	exfutcoin "trading-core/pkg/exchanges/binance/futures_coin"
	exfutusdt "trading-core/pkg/exchanges/binance/futures_usdt"
	exspot "trading-core/pkg/exchanges/binance/spot"
	exchange "trading-core/pkg/exchanges/common"
)

// DefaultFactory creates Gateway instances based on exchange type.
func DefaultFactory(conn db.Connection, apiKey, apiSecret string) (exchange.Gateway, error) {
	switch conn.ExchangeType {
	case "binance-spot":
		return exspot.New(exspot.Config{
			APIKey:    apiKey,
			APISecret: apiSecret,
			Testnet:   false,
		}), nil

	case "binance-usdtfut":
		return exfutusdt.NewClient(exfutusdt.Config{
			APIKey:    apiKey,
			APISecret: apiSecret,
			Testnet:   false,
		}), nil

	case "binance-coinfut":
		return exfutcoin.NewClient(exfutcoin.Config{
			APIKey:    apiKey,
			APISecret: apiSecret,
			Testnet:   false,
		}), nil

	default:
		return nil, fmt.Errorf("unsupported exchange type: %s", conn.ExchangeType)
	}
}

// TestnetFactory creates Gateway instances for testnet.
func TestnetFactory(conn db.Connection, apiKey, apiSecret string) (exchange.Gateway, error) {
	switch conn.ExchangeType {
	case "binance-spot":
		return exspot.New(exspot.Config{
			APIKey:    apiKey,
			APISecret: apiSecret,
			Testnet:   true,
		}), nil

	case "binance-usdtfut":
		return exfutusdt.NewClient(exfutusdt.Config{
			APIKey:    apiKey,
			APISecret: apiSecret,
			Testnet:   true,
		}), nil

	case "binance-coinfut":
		return exfutcoin.NewClient(exfutcoin.Config{
			APIKey:    apiKey,
			APISecret: apiSecret,
			Testnet:   true,
		}), nil

	default:
		return nil, fmt.Errorf("unsupported exchange type: %s", conn.ExchangeType)
	}
}
