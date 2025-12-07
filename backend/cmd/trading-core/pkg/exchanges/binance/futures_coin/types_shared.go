package futures_coin

import (
	"strings"

	"trading-core/pkg/exchanges/common"
)

type OpenOrder struct {
	Symbol        string `json:"symbol"`
	OrderID       int64  `json:"orderId"`
	ClientOrderID string `json:"clientOrderId"`
	Side          string `json:"side"`
	Type          string `json:"type"`
	Price         string `json:"price"`
	OrigQty       string `json:"origQty"`
	ExecQty       string `json:"executedQty"`
	Status        string `json:"status"`
	PositionSide  string `json:"positionSide"`
	ReduceOnly    bool   `json:"reduceOnly"`
}

type FuturesBalance struct {
	Asset              string `json:"asset"`
	Balance            string `json:"balance"`
	CrossWalletBalance string `json:"crossWalletBalance"`
	CrossUnPnl         string `json:"crossUnPnl"`
	AvailableBalance   string `json:"availableBalance"`
	AccountAlias       string `json:"accountAlias,omitempty"`
}

type UserTrade struct {
	Symbol          string `json:"symbol"`
	Id              int64  `json:"id"`
	OrderID         int64  `json:"orderId"`
	Price           string `json:"price"`
	Qty             string `json:"qty"`
	QuoteQty        string `json:"quoteQty"`
	RealizedPnl     string `json:"realizedPnl"`
	MarginAsset     string `json:"marginAsset"`
	Commission      string `json:"commission"`
	CommissionAsset string `json:"commissionAsset"`
	Time            int64  `json:"time"`
	Buyer           bool   `json:"buyer"`
	Maker           bool   `json:"maker"`
}

type Income struct {
	Symbol     string `json:"symbol"`
	IncomeType string `json:"incomeType"`
	Income     string `json:"income"`
	Asset      string `json:"asset"`
	Time       int64  `json:"time"`
	Info       string `json:"info"`
	TranID     int64  `json:"tranId"`
	TradeID    string `json:"tradeId"`
}

func mapStatus(s string) common.OrderStatus {
	switch strings.ToUpper(s) {
	case "NEW":
		return common.StatusNew
	case "PARTIALLY_FILLED":
		return common.StatusPartial
	case "FILLED":
		return common.StatusFilled
	case "CANCELED":
		return common.StatusCanceled
	case "REJECTED":
		return common.StatusRejected
	case "EXPIRED":
		return common.StatusExpired
	default:
		return common.StatusUnknown
	}
}

