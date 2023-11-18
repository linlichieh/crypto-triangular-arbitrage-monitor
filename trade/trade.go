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
	Balance decimal.Decimal      // USDT
	Qty     chan decimal.Decimal // When ws private channel receives updates, will send a notification to here
}

func Init() *Trade {
	return &Trade{
		Qty: make(chan decimal.Decimal),
	}
}
