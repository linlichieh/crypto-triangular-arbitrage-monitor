package main

import (
	"crypto-triangular-arbitrage-watch/bybit"
	"crypto-triangular-arbitrage-watch/notification"
	"crypto-triangular-arbitrage-watch/runner"
	"crypto-triangular-arbitrage-watch/trade"
	"crypto-triangular-arbitrage-watch/tri"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

func main() {
	// Define a string flag with a default value and a short description.
	action := flag.String("action", "", "")
	// Define an integer flag.
	qty := flag.String("qty", "0", "Quantity")

	sym := flag.String("sym", "BTCUSDT", "")

	limit := flag.Int("limit", 1, "")

	// Parse the flags.
	flag.Parse()

	switch *action {
	case trade.SIDE_BUY, trade.SIDE_SELL:
		loadEnvConfig("")
		placeOrder(*action, *sym, *qty)
	case "trii":
		loadEnvConfig("")
		trii(*qty)
	case "instrument":
		loadEnvConfig("prod-config")
		instrument(*sym)
	case "generate_instruments":
		generateInstruments("dev")
		generateInstruments("prod")
	case "all_symbols":
		allSymbols()
	case "order_history":
		loadEnvConfig("")
		orderHistory(*limit)
	default:
		log.Fatalf("action '%s' not supported", *action)
	}
}

func loadEnvConfig(config string) {
	if config == "" {
		config = "config"
	}
	viper.SetConfigName(config)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %w \n", err))
	}
	if strings.TrimSpace(viper.GetString("ENV")) == "" {
		log.Fatal("ENV isn't set in the config")
	}
	fmt.Printf("ENV: %s\n", viper.GetString("ENV"))
}

func placeOrder(side string, sym string, qty string) {
	tri := tri.Init()
	tri.Build()
	api := bybit.InitApi()
	api.SetTri(tri)
	decimalQty, err := decimal.NewFromString(qty)
	if err != nil {
		log.Fatal(err)
	}
	resp, err := api.PlaceOrder(side, sym, decimalQty)
	if err != nil {
		log.Println("err:", err)
		return
	}
	log.Println(resp)
}

func trii(qty string) {
	// slack
	slack := notification.Init()
	go slack.HandleChannelSystemLogs()

	// tri
	tri := tri.Init()
	tri.Build()
	tri.SetSlack(slack)
	tri.PrintAllSymbols()
	tri.PrintAllCombinations()

	// ordrebookRunner
	orderbookRunner := runner.Init(tri)
	orderbookRunner.CalculateTriArb = false
	orderbookRunner.SetSlack(slack)
	go orderbookRunner.ListenAll()

	triTrade := trade.Init()

	// bybit
	ws := bybit.InitWs()
	ws.SetTri(tri)
	ws.SetTrade(triTrade)
	ws.SetOrderbookRunner(orderbookRunner)
	ws.SetSlack(slack)
	go ws.HandlePrivateChannel()
	go ws.HandlePublicChannel() // block

	// Check if symbols are ready
	var allSymbols []string
	for symbol, _ := range tri.SymbolOrdersMap {
		allSymbols = append(allSymbols, symbol)
	}
	log.Println("all symbols:", allSymbols)
	for {
		if tri.SymbolOrdersMap[allSymbols[0]].Ready() && tri.SymbolOrdersMap[allSymbols[1]].Ready() && tri.SymbolOrdersMap[allSymbols[2]].Ready() {
			break
		}
		log.Printf("Not ready, waiting for new prices: %v %v %v\n", tri.SymbolOrdersMap[allSymbols[0]], tri.SymbolOrdersMap[allSymbols[1]], tri.SymbolOrdersMap[allSymbols[2]])
		time.Sleep(100 * time.Millisecond)
	}
	log.Printf("Ready! new prices received: %v %v %v\n", tri.SymbolOrdersMap[allSymbols[0]], tri.SymbolOrdersMap[allSymbols[1]], tri.SymbolOrdersMap[allSymbols[2]])
	combination := tri.SymbolCombinationsMap[allSymbols[0]][1] // For testing, just get the first combination
	log.Printf("Will use this combination: %s -> %s -> %s\n", combination.SymbolOrders[0].Symbol, combination.SymbolOrders[1].Symbol, combination.SymbolOrders[2].Symbol)

	// Tri trade
	api := bybit.InitApi()
	api.SetTri(tri)

	decimalQty, err := decimal.NewFromString(qty)
	if err != nil {
		log.Fatal(err)
	}

	// 1st trade
	resp, err := api.PlaceOrder(trade.SIDE_BUY, combination.SymbolOrders[0].Symbol, decimalQty)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("1st %s resp %+v\n", combination.SymbolOrders[0].Symbol, resp)
	tradeQty := <-triTrade.Qty
	log.Println("1st qty:", tradeQty)

	// 2nd trade
	if combination.BaseQuote {
		fmt.Println("2nd sell")
		resp, err = api.PlaceOrder(trade.SIDE_SELL, combination.SymbolOrders[1].Symbol, tradeQty)
	} else {
		fmt.Println("2nd buy")
		resp, err = api.PlaceOrder(trade.SIDE_BUY, combination.SymbolOrders[1].Symbol, tradeQty)
	}
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("2nd %s resp %+v\n", combination.SymbolOrders[1].Symbol, resp)
	tradeQty = <-triTrade.Qty
	log.Println("2nd qty:", tradeQty)

	// 3rd trade
	resp, err = api.PlaceOrder(trade.SIDE_SELL, combination.SymbolOrders[2].Symbol, tradeQty)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("3rd %s resp %+v\n", combination.SymbolOrders[2].Symbol, resp)
	tradeQty = <-triTrade.Qty
	log.Println("3rd qty:", tradeQty)
	log.Printf("Done! %s -> %s", decimalQty.String(), tradeQty.String())

	// TODO some issues with ETHUSDT -> ETHBTC -> BTCUSDT
	// TODO order.spot might miss to notfiy order status, need to check by myself via order history api
	// TODO retry logic for cancelled
}

// TESTNET doesn't have MNTBTC, use prod bybit host
func instrument(sym string) {
	api := bybit.InitApi()
	resp, err := api.GetInstrumentsInfo(sym)
	if err != nil {
		log.Println("err:", err)
		return
	}
	if len(resp.Result.List) > 0 {
		log.Printf("symbol: %s  InstrumentResp: %+v\n", sym, resp)
	} else {
		log.Printf("symbol: %s  no list", sym)
	}
}

// TESTNET doesn't have MNTBTC, use prod bybit host
func generateInstruments(env string) {
	var configFileName string
	tri := tri.Init()
	switch env {
	case "dev":
		loadEnvConfig("")
		configFileName = "symbol_instruments.json"
	case "prod":
		loadEnvConfig("prod-config")
		tri.SetSymCombPath("prod-symbol_combinations.json")
		configFileName = "prod-symbol_instruments.json"
	}
	tri.BuildSymbolCombinations()
	var allSymbols []string
	for symbol, _ := range tri.SymbolOrdersMap {
		allSymbols = append(allSymbols, symbol)
	}
	api := bybit.InitApi()
	result := map[string]map[string]string{}
	for _, sym := range allSymbols {
		resp, err := api.GetInstrumentsInfo(sym)
		if err != nil {
			log.Println("err:", err)
			return
		}
		if len(resp.Result.List) > 0 {
			result[sym] = map[string]string{
				"base_precision":  resp.Result.List[0].LotSizeFilter.BasePrecision,
				"quote_precision": resp.Result.List[0].LotSizeFilter.QuotePrecision,
				"min_order_qty":   resp.Result.List[0].LotSizeFilter.MinOrderQty,
				"max_order_qty":   resp.Result.List[0].LotSizeFilter.MaxOrderQty,
				"min_order_amt":   resp.Result.List[0].LotSizeFilter.MinOrderAmt,
				"max_order_amt":   resp.Result.List[0].LotSizeFilter.MaxOrderAmt,
			}
		} else {
			log.Printf("symbol: %s  no list", sym)
		}
	}

	// Write json into file
	// Create or open the file
	file, err := os.Create(configFileName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	jsonData, err := json.Marshal(result)
	if err != nil {
		log.Fatal(err)
	}

	// Write the JSON data to the file
	_, err = file.Write(jsonData)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result)
	fmt.Printf("'%s' has been created\n", configFileName)
}

// resp:
//
//	{
//		"retCode":0,
//		"retMsg":"OK",
//		"result":{
//			"category":"spot",
//			"list":[
//				{
//					"symbol":"BTCUSDT",
//					"baseCoin":"BTC",
//					"quoteCoin":"USDT",
//					"innovation":"0",
//					"status":"Trading",
//					"marginTrading":"both",
//					"lotSizeFilter":{
//						"basePrecision":"0.000001",
//						"quotePrecision":"0.00000001",
//						"minOrderQty":"0.000048",
//						"maxOrderQty":"200",
//						"minOrderAmt":"1",
//						"maxOrderAmt":"2000000"
//					},
//					"priceFilter":{
//						"tickSize":"0.01"
//					}
//				},
//				{
//					..
//				}
//			]
//		}
//	}
func allSymbols() {
	url := "https://api.bybit.com/v5/market/instruments-info?category=spot" // Replace with the actual endpoint
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	type Resp struct {
		RetCode int    `json:"ret_code"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			Category string `json:"category"`
			List     []struct {
				Symbol string `json:"symbol"`
			} `json:"list"`
		} `json:"result"`
	}
	res := Resp{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("category:", res.Result.Category)
	var syms []string
	for _, sym := range res.Result.List {
		syms = append(syms, sym.Symbol)
	}
	fmt.Println(syms)
}

func orderHistory(limit int) {
	api := bybit.InitApi()
	resp, err := api.GetOrderHistory(limit)
	if err != nil {
		log.Println("err:", err)
		return
	}
	log.Println(string(resp))
}
