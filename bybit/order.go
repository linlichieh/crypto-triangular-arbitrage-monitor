package bybit

import (
	"crypto-triangular-arbitrage-watch/trade"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

const (
	TIMEOUT_SECOND = 3
	ORDER_ENDPOINT = "/v5/order/create"
)

type Api struct {
	Client *http.Client
}

func InitApi() *Api {
	return &Api{
		Client: &http.Client{Timeout: time.Duration(TIMEOUT_SECOND) * time.Second},
	}
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
func (api *Api) PlaceOrder(side string, symbol string, qty string) (resp map[string]any, err error) {
	if side != trade.SIDE_BUY && side != trade.SIDE_SELL {
		err = errors.New(side + " not supported")
		return
	}
	params := map[string]any{
		"category":  trade.CATEGORY_SPOT,
		"symbol":    symbol,
		"orderType": trade.ORDER_TYPE_MARKET,
		"side":      side,
		"qty":       qty,
	}
	body, err := api.post(ORDER_ENDPOINT, params)
	if err != nil {
		return
	}
	err = json.Unmarshal(body, &resp)
	return
}
