package events

// UserDataStreamEvent represents events from User Data Stream
type UserDataStreamEvent struct {
	EventType string
	EventTime int64
	Data      interface{}
}

// ExecutionReport represents order execution update
type ExecutionReport struct {
	Symbol          string
	Side            string
	OrderType       string
	Price           string
	Qty             string
	Status          string
	OrderID         int64
	ClientOrderID   string
	ExecutedQty     string
	CumulativeQty   string
	LastPrice       string
	LastQty         string
	Commission      string
	CommissionAsset string
	TradeID         int64
	IsMaker         bool
}

// AccountUpdate represents account balance update
type AccountUpdate struct {
	Asset  string
	Free   string
	Locked string
}

// BalanceUpdate represents balance change
type BalanceUpdate struct {
	Asset  string
	Delta  string
	Time   int64
	Reason string
}
