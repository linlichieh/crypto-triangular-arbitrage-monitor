package main

import (
	"crypto-triangular-arbitrage-watch/bybit"
	"crypto-triangular-arbitrage-watch/notification"
	"crypto-triangular-arbitrage-watch/runner"
	"crypto-triangular-arbitrage-watch/trade"
	"crypto-triangular-arbitrage-watch/tri"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spf13/viper"
)

func main() {
	loadEnvConfig()
	// Define a string flag with a default value and a short description.
	action := flag.String("action", "Sell", "Buy or Sell")
	// Define an integer flag.
	qty := flag.String("qty", "0", "Quantity")
	// Parse the flags.
	flag.Parse()

	switch *action {
	case trade.SIDE_BUY, trade.SIDE_SELL:
		placeOrder(*action, *qty)
	case "trii":
		trii()
	default:
		log.Fatalf("action '%s' not supported", *action)
	}
}

func loadEnvConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %w \n", err))
	}
	if strings.TrimSpace(viper.GetString("ENV")) == "" {
		log.Fatal("ENV isn't set in the config")
	}
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

	select {}
}
