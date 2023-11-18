package trade

import "github.com/shopspring/decimal"

const (
	BID               = "bid"
	ASK               = "ask"
	CATEGORY_SPOT     = "spot"
	SIDE_BUY          = "Buy"
	SIDE_SELL         = "Sell"
	ORDER_TYPE_MARKET = "Market"
)

type Trade struct {
	BeforeTradeBalance decimal.Decimal
	AfterTradeBalance  decimal.Decimal
	ActualQty          decimal.Decimal
}
