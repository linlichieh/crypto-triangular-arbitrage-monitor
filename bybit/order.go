package bybit

import (
	"crypto-triangular-arbitrage-watch/trade"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

const (
	TIMEOUT_SECOND      = 3
	ORDER_ENDPOINT      = "/v5/order/create"
	INSTRUMENT_ENDPOINT = "/v5/market/instruments-info"
)

type Api struct {
	Client *http.Client
}

// resp:
//
//	{
//	  "retCode": 0,
//	  "retMsg": "OK",
//	  "result": {
//		"category": "spot",
//		"list": [
//		  {
//			"symbol": "BTCUSDT",
//			"baseCoin": "BTC",
//			"quoteCoin": "USDT",
//			"innovation": "0",
//			"status": "Trading",
//			"marginTrading": "both",
//			"lotSizeFilter": {
//			  "basePrecision": "0.000001",
//			  "quotePrecision": "0.00000001",
//			  "minOrderQty": "0.000048",
//			  "maxOrderQty": "200",
//			  "minOrderAmt": "1",
//			  "maxOrderAmt": "2000000"
//			},
//			"priceFilter": {
//			  "tickSize": "0.01"
//			}
//		  }
//		]
//	  },
//	  "retExtInfo": {},
//	  "time": 1700285758649
//	}
type InstrumentResp struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		Category string `json:"category"`
		List     []struct {
			Symbol        string `json:"symbol"`
			BaseCoin      string `json:"baseCoin"`
			QuoteCoin     string `json:"quoteCoin"`
			Status        string `json:"status"`
			LotSizeFilter struct {
				BasePrecision  string `json:"basePrecision"`
				QuotePrecision string `json:"quotePrecision"`
			} `json:"lotSizeFilter"`
		} `json:"list"`
	} `json:"result"`
	RetExtInfo map[string]any `json:"retExtInfo"`
	Time       int64          `json:"time"`
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
	// resp:
	//	- map[result:map[] retCode:10001 retExtInfo:map[] retMsg:The order remains unchanged as the parameters entered match the existing ones. time:1.700282830415e+12]
	//	- map[result:map[orderId:1556479670277641728 orderLinkId:1556479670277641729] retCode:0 retExtInfo:map[] retMsg:OK time:1.700282835694e+12]
	err = json.Unmarshal(body, &resp)
	return
}

func (api *Api) GetInstrumentsInfo(symbol string) (resp *InstrumentResp, err error) {
	params := map[string]string{
		"category": trade.CATEGORY_SPOT,
		"symbol":   symbol,
	}
	body, err := api.get(INSTRUMENT_ENDPOINT, params)
	if err != nil {
		return
	}
	err = json.Unmarshal(body, &resp)
	return
}

func PrecisionConverter(p string) (int, error) {
	// Find the index of the decimal point
	pointIndex := strings.Index(p, ".")

	// Count the number of decimal places
	if pointIndex != -1 {
		precision := len(p) - pointIndex - 1
		return precision, nil
	}
	return 0, errors.New("No decimal point found.")
}
