package common

// Side denotes order side.
type Side string

const (
	SideBuy  Side = "BUY"
	SideSell Side = "SELL"
)

// OrderType denotes basic order types.
type OrderType string

const (
	OrderTypeMarket          OrderType = "MARKET"
	OrderTypeLimit           OrderType = "LIMIT"
	OrderTypeStopLoss        OrderType = "STOP_LOSS"
	OrderTypeStopLossLimit   OrderType = "STOP_LOSS_LIMIT"
	OrderTypeTakeProfit      OrderType = "TAKE_PROFIT"
	OrderTypeTakeProfitLimit OrderType = "TAKE_PROFIT_LIMIT"
	OrderTypeLimitMaker      OrderType = "LIMIT_MAKER"
	OrderTypeTrailingStop    OrderType = "TRAILING_STOP_MARKET" // Futures only
)

// TimeInForce captures TIF semantics.
type TimeInForce string

const (
	TIFGTC TimeInForce = "GTC" // Good Till Cancelled
	TIFIOC TimeInForce = "IOC" // Immediate Or Cancel
	TIFFOK TimeInForce = "FOK" // Fill Or Kill
	TIFGTX TimeInForce = "GTX" // Post Only / Maker Only
)

// OrderStatus normalizes exchange status into a small set.
type OrderStatus string

const (
	StatusNew      OrderStatus = "NEW"
	StatusPartial  OrderStatus = "PARTIAL"
	StatusFilled   OrderStatus = "FILLED"
	StatusCanceled OrderStatus = "CANCELED"
	StatusRejected OrderStatus = "REJECTED"
	StatusExpired  OrderStatus = "EXPIRED"
	StatusUnknown  OrderStatus = "UNKNOWN"
)

// MarketType distinguishes spot vs futures venues.
type MarketType string

const (
	MarketSpot    MarketType = "SPOT"
	MarketUSDTFut MarketType = "USDT_FUTURES"
	MarketCoinFut MarketType = "COIN_FUTURES"
)

// OrderRequest captures an order intent to be sent to an exchange.
type OrderRequest struct {
	Symbol       string
	Side         Side
	Type         OrderType
	Qty          float64
	Price        float64 // required for LIMIT
	StopPrice    float64 // required for STOP_LOSS/TAKE_PROFIT orders
	TimeInForce  TimeInForce
	IcebergQty   float64 // for iceberg orders (visible quantity)
	ClientID     string  // optional client order id
	ReduceOnly   bool
	PositionSide string // LONG/SHORT for hedge mode futures
	Market       MarketType
	Leverage     int // futures leverage (optional)

	// Futures-specific
	WorkingType     string  // MARK_PRICE or CONTRACT_PRICE
	PriceProtect    bool    // price protection
	ActivationPrice float64 // for TRAILING_STOP
	CallbackRate    float64 // for TRAILING_STOP (percentage)
}

// OrderResult returns the exchange ack.
type OrderResult struct {
	ExchangeOrderID string
	Status          OrderStatus
	ClientID        string
}

// Fill represents a trade fill update.
type Fill struct {
	ExchangeOrderID string
	TradeID         string
	Symbol          string
	Side            Side
	Qty             float64
	Price           float64
}
