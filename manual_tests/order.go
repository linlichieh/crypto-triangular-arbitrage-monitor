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

	// Parse the flags.
	flag.Parse()

	switch *action {
	case trade.SIDE_BUY, trade.SIDE_SELL:
		loadEnvConfig("")
		placeOrder(*action, *qty)
	case "trii":
		loadEnvConfig("")
		trii()
	case "instrument":
		loadEnvConfig("prod-config")
		instrument(*sym)
	case "generate_instruments":
		loadEnvConfig("prod-config")
		generateInstruments()
	case "all_symbols":
		allSymbols()
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

func placeOrder(side string, qty string) {
	api := bybit.InitApi()
	resp, err := api.PlaceOrder(side, "BTCUSDT", qty)
	if err != nil {
		log.Println("err:", err)
		return
	}
	log.Println(resp)
}

func trii() {
	// slack
	slack := notification.Init()
	go slack.HandleChannelSystemLogs()

	// tri
	tri := tri.Init()
	tri.SetSlack(slack)
	tri.PrintAllSymbols()
	tri.PrintAllCombinations()

	// ordrebookRunner
	orderbookRunner := runner.Init(tri)
	orderbookRunner.CalculateTriArb = false
	orderbookRunner.SetSlack(slack)
	go orderbookRunner.ListenAll()

	// bybit
	bybit := bybit.Init()
	bybit.SetTri(tri)
	bybit.SetOrderbookRunner(orderbookRunner)
	bybit.SetSlack(slack)
	go bybit.HandlePrivateChannel()
	go bybit.HandlePublicChannel() // block

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
	combination := tri.SymbolCombinationsMap[allSymbols[0]][0] // For testing, just get the first combination
	log.Printf("Will use this combination: %s -> %s -> %s\n", combination.SymbolOrders[0].Symbol, combination.SymbolOrders[1].Symbol, combination.SymbolOrders[2].Symbol)

	// Tri trade
	// api := bybit.InitApi()
	// resp, err := api.PlaceOrder(side, "BTCUSDT", qty)
	// if err != nil {
	// log.Println("err:", err)
	// return
	// }

	select {}
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
		log.Printf("symbol: %s  basePre: %s  quotePre: %s\n", sym, resp.Result.List[0].LotSizeFilter.BasePrecision, resp.Result.List[0].LotSizeFilter.QuotePrecision)
	} else {
		log.Printf("symbol: %s  no list", sym)
	}

	// Example quantity
	quantity := 123.123456789

	// Convert float64 to decimal.Decimal
	decimalQuantity := decimal.NewFromFloat(quantity)

	// Define the precision as the number of decimal places
	precision, _ := bybit.PrecisionConverter("0.00001")

	// Format the quantity with the desired precision
	formattedQuantity := decimalQuantity.Round(int32(precision))

	// Print the formatted quantity
	fmt.Println(quantity, formattedQuantity.String())
}

// TESTNET doesn't have MNTBTC, use prod bybit host
func generateInstruments() {
	tri := tri.Init()
	tri.SetConfigPath("prod-symbol_combinations.json")
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
			m := map[string]string{}
			m["base_precision"] = resp.Result.List[0].LotSizeFilter.BasePrecision
			m["quote_precision"] = resp.Result.List[0].LotSizeFilter.QuotePrecision
			result[sym] = m
		} else {
			log.Printf("symbol: %s  no list", sym)
		}
	}

	// Write json into file
	// Create or open the file
	file, err := os.Create("symbol_instruments.json")
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
	fmt.Println("'symbol_instruments.json' has been created")
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
