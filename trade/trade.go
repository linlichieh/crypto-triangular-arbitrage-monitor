package trade

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
	bybit "github.com/wuhewuhe/bybit.go.api"
)

const (
	BID               = "bid"
	ASK               = "ask"
	CATEGORY_SPOT     = "spot"
	SIDE_BUY          = "Buy"
	SIDE_SELL         = "Sell"
	ORDER_TYPE_MARKET = "Market"
)

type Trade struct {
	USDT      decimal.Decimal // USDT balance
	ActualQty decimal.Decimal
}

func Init() *Trade {
	return &Trade{}
}

// For Spot Market Buy order, please note that qty should be quote curreny amount, and make sure it satisfies quotePrecision in Spot instrument spec
// https://bybit-exchange.github.io/docs/v5/market/instrument#response-parameters
// for example:
//
//	"symbol": "BTCUSDT",
//	"baseCoin": "BTC",
//	"quoteCoin": "USDT",
//	"basePrecision": "0.000001", for sell btc - 0.003478 is valid; 0.00347851 is invalid
//	"quotePrecision": "0.00000001", for buy USDT
//
// response:
//
//	{
//			"retCode": 0,
//			"retMsg": "OK",
//			"result": {
//					"orderId": "1551741421621614080",
//					"orderLinkId": "1551741421621614081"
//			},
//			"retExtInfo": {},
//			"time": 1699717992439
//	}
func (t *Trade) Buy(symbol string, qty string) {
	// TODO FIXME dev: bybit.TESTNET  prod: ???
	client := bybit.NewBybitHttpClient(viper.GetString("BYBIT_API_KEY"), viper.GetString("BYBIT_API_SECRET"), bybit.WithBaseURL(bybit.TESTNET))
	params := map[string]interface{}{
		"category":  CATEGORY_SPOT,
		"symbol":    symbol,
		"orderType": ORDER_TYPE_MARKET,
		"side":      SIDE_BUY,
		"qty":       qty,
	}
	orderResult, err := client.NewTradeService(params).PlaceOrder(context.Background())
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(bybit.PrettyPrint(orderResult))
	if orderResult.RetCode == 0 {
		fmt.Println("success")
	} else {
		fmt.Println("fail")
	}
}

func (t *Trade) Sell(symbol string, qty string) {
	client := bybit.NewBybitHttpClient(viper.GetString("BYBIT_API_KEY"), viper.GetString("BYBIT_API_SECRET"), bybit.WithBaseURL(bybit.TESTNET))
	params := map[string]interface{}{
		"category":  CATEGORY_SPOT,
		"symbol":    symbol,
		"orderType": ORDER_TYPE_MARKET,
		"side":      SIDE_SELL,
		"qty":       qty,
	}
	orderResult, err := client.NewTradeService(params).PlaceOrder(context.Background())
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(bybit.PrettyPrint(orderResult))
	if orderResult.RetCode == 0 {
		fmt.Println("success")
	} else {
		fmt.Println("fail")
	}
}
