package main

import (
	"context"
	"crypto-triangular-arbitrage-watch/notification"
	"crypto-triangular-arbitrage-watch/runner"
	"crypto-triangular-arbitrage-watch/tri"
	"crypto-triangular-arbitrage-watch/ws"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spf13/viper"
	bybit "github.com/wuhewuhe/bybit.go.api"
)

const (
	CATEGORY_SPOT     = "spot"
	SIDE_BUY          = "Buy"
	SIDE_SELL         = "Sell"
	ORDER_TYPE_MARKET = "Market"
)

type Client struct {
	bybitClient *bybit.Client
}

func main() {
	loadEnvConfig()
	// Define a string flag with a default value and a short description.
	action := flag.String("action", "Sell", "Buy or Sell")
	// Define an integer flag.
	qty := flag.String("qty", "0", "Quantity")
	// Parse the flags.
	flag.Parse()

	client := Client{
		bybitClient: bybit.NewBybitHttpClient(viper.GetString("BYBIT_API_KEY"), viper.GetString("BYBIT_API_SECRET"), bybit.WithBaseURL(bybit.TESTNET)),
	}

	switch *action {
	case SIDE_BUY:
		client.buy(*qty)
	case SIDE_SELL:
		client.sell(*qty)
	case "trii":
		client.tri()
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

func (c *Client) buy(qty string) {
	params := map[string]interface{}{
		"category":  CATEGORY_SPOT,
		"symbol":    "BTCUSDT",
		"orderType": ORDER_TYPE_MARKET,
		"side":      SIDE_BUY,
		"qty":       qty,
	}
	orderResult, err := c.bybitClient.NewTradeService(params).PlaceOrder(context.Background())
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

func (c *Client) sell(qty string) {
	params := map[string]interface{}{
		"category":  CATEGORY_SPOT,
		"symbol":    "BTCUSDT",
		"orderType": ORDER_TYPE_MARKET,
		"side":      SIDE_SELL,
		"qty":       qty,
	}
	orderResult, err := c.bybitClient.NewTradeService(params).PlaceOrder(context.Background())
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

func (c *Client) tri() {
	slack := notification.Init()
	go slack.HandleChannelSystemLogs()
	tri := tri.Init()
	tri.SetSlack(slack)
	tri.PrintAllSymbols()
	tri.PrintAllCombinations()
	orderbookRunner := runner.Init(tri)
	orderbookRunner.CalculateTriArb = false
	orderbookRunner.SetSlack(slack)
	go orderbookRunner.ListenAll()
	wsClient := ws.Init()
	wsClient.SetTri(tri)
	wsClient.SetOrderbookRunner(orderbookRunner)
	wsClient.SetSlack(slack)
	go wsClient.HandlePrivateChannel()
	go wsClient.HandlePublicChannel() // block

	var allSymbols []string
	for symbol, _ := range tri.SymbolOrdersMap {
		allSymbols = append(allSymbols, symbol)
	}
	fmt.Println("all symbols:", allSymbols)
	for {
		if tri.SymbolOrdersMap[allSymbols[0]].Ready() && tri.SymbolOrdersMap[allSymbols[1]].Ready() && tri.SymbolOrdersMap[allSymbols[2]].Ready() {
			break
		}
		log.Printf("not ready: %v %v %v\n", tri.SymbolOrdersMap[allSymbols[0]], tri.SymbolOrdersMap[allSymbols[1]], tri.SymbolOrdersMap[allSymbols[2]])
		time.Sleep(100 * time.Millisecond)
	}
	log.Printf("ready: %v %v %v\n", tri.SymbolOrdersMap[allSymbols[0]], tri.SymbolOrdersMap[allSymbols[1]], tri.SymbolOrdersMap[allSymbols[2]])

	log.Printf("%s: %v\n", allSymbols[0], tri.SymbolCombinationsMap)
	combination := tri.SymbolCombinationsMap[allSymbols[0]][0] // For testing, just get the first combination
	log.Printf("%+v\n", combination)

	// TODO trade tri

	// TODO
	// Conduct tri trade

	select {}
}
