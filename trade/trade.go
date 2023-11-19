package trade

import "github.com/shopspring/decimal"

const (
	BID               = "bid"
	ASK               = "ask"
	CATEGORY_SPOT     = "spot"
	SIDE_BUY          = "Buy"
	SIDE_SELL         = "Sell"
	ORDER_TYPE_MARKET = "Market"

	RETRY_INTERVAL_SECOND = 1
)

type Trade struct {
	Balance decimal.Decimal      // USDT
	Qty     chan decimal.Decimal // When ws private channel receives updates, will send a notification to here
	Retry   chan int
}

func Init() *Trade {
	return &Trade{
		Qty:   make(chan decimal.Decimal),
		Retry: make(chan int),
	}
}
