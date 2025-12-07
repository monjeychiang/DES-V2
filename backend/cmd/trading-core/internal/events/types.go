package events

// Event enumerates high-level topics inside the trading core.
type Event string

const (
	EventPriceTick            Event = "price_tick"
	EventOrderUpdate          Event = "order_update"
	EventStrategySignal       Event = "strategy_signal"
	EventRiskAlert            Event = "risk_alert"
	EventPositionChange       Event = "position_change"
	EventOrderSubmitted       Event = "order.submitted"
	EventOrderAccepted        Event = "order.accepted"
	EventOrderRejected        Event = "order.rejected"
	EventOrderFilled          Event = "order.filled"
	EventOrderPartiallyFilled Event = "order.partially_filled"
)
