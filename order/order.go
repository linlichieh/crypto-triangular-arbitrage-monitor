package order

import (
	"context"
	"fmt"

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

func PlaceOrder() {
	/*
			buy btc
			"orderId": "1551475535790740992",
		"orderLinkId": "1551475535790740993"

			sell btc
			orderId": "1551477564315538944",
						"orderLinkId": "1551477564315538945"
	*/
	// TODO FIXME dev: bybit.TESTNET  prod: ???
	client := bybit.NewBybitHttpClient(viper.GetString("BYBIT_API_KEY"), viper.GetString("BYBIT_API_SECRET"), bybit.WithBaseURL(bybit.TESTNET))
	params := map[string]interface{}{
		"category":  CATEGORY_SPOT,
		"symbol":    "BTCUSDT",
		"orderType": ORDER_TYPE_MARKET,
		// For Spot Market Buy order, please note that qty should be quote curreny amount, and make sure it satisfies quotePrecision in Spot instrument spec
		// https://bybit-exchange.github.io/docs/v5/market/instrument#response-parameters
		// for example:
		// "symbol": "BTCUSDT",
		// "baseCoin": "BTC",
		// "quoteCoin": "USDT",
		// "basePrecision": "0.000001", for sell btc - 0.003478 is valid; 0.00347851 is invalid
		// "quotePrecision": "0.00000001", for buy USDT
		"side": SIDE_SELL,
		"qty":  "0.003478",
		// "side":      SIDE_BUY,
		// "qty": "100",
	}
	orderResult, err := client.NewTradeService(params).PlaceOrder(context.Background())
	if err != nil {
		fmt.Println(err)
		return
	}
	// RetMsg
	// Result
	// RetExtInfo
	// Time
	// response:
	//		{
	//				"retCode": 0,
	//				"retMsg": "OK",
	//				"result": {
	//						"orderId": "1551741421621614080",
	//						"orderLinkId": "1551741421621614081"
	//				},
	//				"retExtInfo": {},
	//				"time": 1699717992439
	//		}
	fmt.Println(bybit.PrettyPrint(orderResult))
	if orderResult.RetCode == 0 {
		fmt.Println("success")
	} else {
		fmt.Println("fail")
	}
}
