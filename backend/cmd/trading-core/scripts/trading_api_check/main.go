package main

import (
	"context"
	"log"
	"os"
	"time"

	"trading-core/pkg/config"
	exchange "trading-core/pkg/exchanges/common"
	exspot "trading-core/pkg/exchanges/binance/spot"
	exfutusdt "trading-core/pkg/exchanges/binance/futures_usdt"
	exfutcoin "trading-core/pkg/exchanges/binance/futures_coin"
)

// trading_api_check/main.go
//
// 小工具：快速測試「程式內封裝的」Binance 交易 API 是否能正常打通。
//
// 用法（實網，建議先用很小的資金或空帳戶）:
//
//   cd backend/cmd/trading-core
//   go run ./scripts/trading_api_check
//
// 相關環境變數（和主程式一致）:
//   BINANCE_API_KEY / BINANCE_API_SECRET
//   BINANCE_USDT_KEY / BINANCE_USDT_SECRET
//   BINANCE_COIN_KEY / BINANCE_COIN_SECRET
//
// 控制測試行為:
//   TRADING_CHECK_PLACE_ORDERS  (default "false")
//        - false: 只做「查詢」與「撤單」類 API，不嘗試下單
//        - true : 會嘗試送出極小的 MARKET 單
//
//   CHECK_SPOT_SYMBOL           (default "BTCUSDT")
//   CHECK_USDT_SYMBOL           (default "BTCUSDT")
//   CHECK_COIN_SYMBOL           (default "BTCUSD_PERP")
//
// 注意：下單測試在你「真的有足夠餘額」時可能會成交；
//       建議一開始先保持 TRADING_CHECK_PLACE_ORDERS=false，確認連線 OK 再打開。

func main() {
	log.Println("=== Trading API check starting ===")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load error: %v", err)
	}

	placeOrders := getenv("TRADING_CHECK_PLACE_ORDERS", "false") == "true"
	spotSymbol := getenv("CHECK_SPOT_SYMBOL", "BTCUSDT")
	usdtSymbol := getenv("CHECK_USDT_SYMBOL", "BTCUSDT")
	coinSymbol := getenv("CHECK_COIN_SYMBOL", "BTCUSD_PERP")

	log.Printf("Config: placeOrders=%v spotSymbol=%s usdtSymbol=%s coinSymbol=%s", placeOrders, spotSymbol, usdtSymbol, coinSymbol)

	// Spot
	if cfg.BinanceAPIKey == "" || cfg.BinanceAPISecret == "" {
		log.Println("[SPOT] BINANCE_API_KEY/SECRET empty, skipping spot checks")
	} else {
		spot := exspot.New(exspot.Config{
			APIKey:    cfg.BinanceAPIKey,
			APISecret: cfg.BinanceAPISecret,
			Testnet:   false,
		})
		checkSpotTrading(spot, spotSymbol, placeOrders)
	}

	// USDT Futures
	if cfg.BinanceUSDTKey == "" || cfg.BinanceUSDTSecret == "" {
		log.Println("[USDT] BINANCE_USDT_KEY/SECRET empty, skipping USDT futures checks")
	} else {
		usdt := exfutusdt.NewClient(exfutusdt.Config{
			APIKey:    cfg.BinanceUSDTKey,
			APISecret: cfg.BinanceUSDTSecret,
			Testnet:   false,
		})
		checkUSDTFutures(usdt, usdtSymbol, placeOrders)
	}

	// COIN Futures
	if cfg.BinanceCoinKey == "" || cfg.BinanceCoinSecret == "" {
		log.Println("[COIN] BINANCE_COIN_KEY/SECRET empty, skipping COIN futures checks")
	} else {
		coin := exfutcoin.NewClient(exfutcoin.Config{
			APIKey:    cfg.BinanceCoinKey,
			APISecret: cfg.BinanceCoinSecret,
			Testnet:   false,
		})
		checkCoinFutures(coin, coinSymbol, placeOrders)
	}

	log.Println("=== Trading API check finished ===")
}

func checkSpotTrading(c *exspot.Client, symbol string, placeOrders bool) {
	log.Println("---- [SPOT] Checking trading API ----")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	info, err := c.GetAccountInfo(ctx)
	if err != nil {
		log.Printf("[SPOT] GetAccountInfo error: %v", err)
	} else {
		log.Printf("[SPOT] CanTrade=%v, balances=%d", info.CanTrade, len(info.Balances))
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()
	openOrders, err := c.GetOpenOrders(ctx2, "")
	if err != nil {
		log.Printf("[SPOT] GetOpenOrders error: %v", err)
	} else {
		log.Printf("[SPOT] Open orders count=%d", len(openOrders))
	}

	ctx3, cancel3 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel3()
	allOrders, err := c.GetAllOrders(ctx3, symbol, 10)
	if err != nil {
		log.Printf("[SPOT] GetAllOrders error: %v", err)
	} else {
		log.Printf("[SPOT] Recent orders for %s: %d", symbol, len(allOrders))
	}

	ctx4, cancel4 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel4()
	trades, err := c.GetMyTrades(ctx4, symbol, 10, "")
	if err != nil {
		log.Printf("[SPOT] GetMyTrades error: %v", err)
	} else {
		log.Printf("[SPOT] Recent trades for %s: %d", symbol, len(trades))
	}

	if !placeOrders {
		log.Println("[SPOT] Skip placing/canceling orders (TRADING_CHECK_PLACE_ORDERS=false)")
		return
	}

	ctx5, cancel5 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel5()
	req := exchange.OrderRequest{
		Symbol: symbol,
		Side:   exchange.SideBuy,
		Type:   exchange.OrderTypeMarket,
		Qty:    0.00001, // 極小數量，避免實際影響太大
		Market: exchange.MarketSpot,
	}
	log.Printf("[SPOT] Submitting test MARKET BUY order %s qty=%f", symbol, req.Qty)
	res, err := c.SubmitOrder(ctx5, req)
	if err != nil {
		log.Printf("[SPOT] SubmitOrder returned error (acceptable for test, e.g. insufficient balance): %v", err)
		return
	}
	log.Printf("[SPOT] SubmitOrder OK exch_id=%s status=%s", res.ExchangeOrderID, res.Status)

	ctx6, cancel6 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel6()
	if cancelErr := c.CancelOrder(ctx6, symbol, res.ExchangeOrderID); cancelErr != nil {
		log.Printf("[SPOT] CancelOrder error (may be filled already): %v", cancelErr)
	} else {
		log.Println("[SPOT] CancelOrder OK")
	}
}

func checkUSDTFutures(c *exfutusdt.Client, symbol string, placeOrders bool) {
	log.Println("---- [USDT] Checking futures trading API ----")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	info, err := c.GetAccountInfo(ctx)
	if err != nil {
		log.Printf("[USDT] GetAccountInfo error: %v", err)
	} else {
		log.Printf("[USDT] CanTrade=%v assets=%d positions=%d", info.CanTrade, len(info.Assets), len(info.Positions))
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()
	pos, err := c.GetPositions(ctx2, "")
	if err != nil {
		log.Printf("[USDT] GetPositions error: %v", err)
	} else {
		log.Printf("[USDT] PositionRisk entries=%d", len(pos))
	}

	ctx3, cancel3 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel3()
	ords, err := c.GetOpenOrders(ctx3, symbol)
	if err != nil {
		log.Printf("[USDT] GetOpenOrders error: %v", err)
	} else {
		log.Printf("[USDT] Open orders for %s: %d", symbol, len(ords))
	}

	ctx4, cancel4 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel4()
	trades, err := c.GetUserTrades(ctx4, symbol, 10, "")
	if err != nil {
		log.Printf("[USDT] GetUserTrades error: %v", err)
	} else {
		log.Printf("[USDT] Recent user trades for %s: %d", symbol, len(trades))
	}

	ctx5, cancel5 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel5()
	income, err := c.GetIncome(ctx5, symbol, "", 10)
	if err != nil {
		log.Printf("[USDT] GetIncome error: %v", err)
	} else {
		log.Printf("[USDT] Income records for %s: %d", symbol, len(income))
	}

	if !placeOrders {
		log.Println("[USDT] Skip placing/canceling orders (TRADING_CHECK_PLACE_ORDERS=false)")
		return
	}

	ctx6, cancel6 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel6()
	req := exchange.OrderRequest{
		Symbol:       symbol,
		Side:         exchange.SideBuy,
		Type:         exchange.OrderTypeMarket,
		Qty:          0.001, // 小數量
		Market:       exchange.MarketUSDTFut,
		PositionSide: "BOTH",
	}
	log.Printf("[USDT] Submitting test MARKET BUY order %s qty=%f", symbol, req.Qty)
	res, err := c.SubmitOrder(ctx6, req)
	if err != nil {
		log.Printf("[USDT] SubmitOrder returned error (acceptable for test, e.g. insufficient margin): %v", err)
		return
	}
	log.Printf("[USDT] SubmitOrder OK exch_id=%s status=%s", res.ExchangeOrderID, res.Status)

	ctx7, cancel7 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel7()
	if cancelErr := c.CancelOrder(ctx7, symbol, res.ExchangeOrderID); cancelErr != nil {
		log.Printf("[USDT] CancelOrder error (may be filled already): %v", cancelErr)
	} else {
		log.Println("[USDT] CancelOrder OK")
	}
}

func checkCoinFutures(c *exfutcoin.Client, symbol string, placeOrders bool) {
	log.Println("---- [COIN] Checking futures trading API ----")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	info, err := c.GetAccountInfo(ctx)
	if err != nil {
		log.Printf("[COIN] GetAccountInfo error: %v", err)
	} else {
		log.Printf("[COIN] CanTrade=%v assets=%d positions=%d", info.CanTrade, len(info.Assets), len(info.Positions))
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()
	pos, err := c.GetPositions(ctx2, "")
	if err != nil {
		log.Printf("[COIN] GetPositions error: %v", err)
	} else {
		log.Printf("[COIN] PositionRisk entries=%d", len(pos))
	}

	ctx3, cancel3 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel3()
	ords, err := c.GetOpenOrders(ctx3, symbol)
	if err != nil {
		log.Printf("[COIN] GetOpenOrders error: %v", err)
	} else {
		log.Printf("[COIN] Open orders for %s: %d", symbol, len(ords))
	}

	ctx4, cancel4 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel4()
	trades, err := c.GetUserTrades(ctx4, symbol, 10, "")
	if err != nil {
		log.Printf("[COIN] GetUserTrades error: %v", err)
	} else {
		log.Printf("[COIN] Recent user trades for %s: %d", symbol, len(trades))
	}

	ctx5, cancel5 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel5()
	income, err := c.GetIncome(ctx5, symbol, "", 10)
	if err != nil {
		log.Printf("[COIN] GetIncome error: %v", err)
	} else {
		log.Printf("[COIN] Income records for %s: %d", symbol, len(income))
	}

	if !placeOrders {
		log.Println("[COIN] Skip placing/canceling orders (TRADING_CHECK_PLACE_ORDERS=false)")
		return
	}

	ctx6, cancel6 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel6()
	req := exchange.OrderRequest{
		Symbol:       symbol,
		Side:         exchange.SideBuy,
		Type:         exchange.OrderTypeMarket,
		Qty:          1, // 1 合約
		Market:       exchange.MarketCoinFut,
		PositionSide: "BOTH",
	}
	log.Printf("[COIN] Submitting test MARKET BUY order %s qty=%f", symbol, req.Qty)
	res, err := c.SubmitOrder(ctx6, req)
	if err != nil {
		log.Printf("[COIN] SubmitOrder returned error (acceptable for test, e.g. insufficient margin): %v", err)
		return
	}
	log.Printf("[COIN] SubmitOrder OK exch_id=%s status=%s", res.ExchangeOrderID, res.Status)

	ctx7, cancel7 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel7()
	if cancelErr := c.CancelOrder(ctx7, symbol, res.ExchangeOrderID); cancelErr != nil {
		log.Printf("[COIN] CancelOrder error (may be filled already): %v", cancelErr)
	} else {
		log.Println("[COIN] CancelOrder OK")
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
