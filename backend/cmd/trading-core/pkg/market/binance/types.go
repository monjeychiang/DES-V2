package market

// Kline represents a single candlestick with all official Binance fields.
type Kline struct {
	Symbol              string  // trading pair symbol
	OpenTime            int64   // 0: Open time (ms)
	Open                float64 // 1: Open price
	High                float64 // 2: High price
	Low                 float64 // 3: Low price
	Close               float64 // 4: Close price
	Volume              float64 // 5: Base asset volume
	CloseTime           int64   // 6: Close time (ms)
	QuoteVolume         float64 // 7: Quote asset volume
	NumberOfTrades      int     // 8: Number of trades
	TakerBuyBaseVolume  float64 // 9: Taker buy base asset volume
	TakerBuyQuoteVolume float64 // 10: Taker buy quote asset volume
	// Field 11 is unused/ignore
}

// Ticker holds lightweight price info for streaming.
type Ticker struct {
	Symbol string
	Price  float64
	Time   int64
}

// BookTicker holds best bid/ask.
type BookTicker struct {
	Symbol   string
	BidPrice float64
	AskPrice float64
	Time     int64
}

// Trade represents a simple trade update.
type Trade struct {
	Symbol       string
	Price        float64
	Qty          float64
	Time         int64
	IsBuyerMaker bool
}

// DepthUpdate represents a diff depth update snapshot.
type DepthUpdate struct {
	Symbol string
	Bids   [][2]float64 // [price, qty]
	Asks   [][2]float64 // [price, qty]
	Time   int64
}
