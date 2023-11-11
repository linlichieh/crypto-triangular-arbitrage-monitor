package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/spf13/viper"
	bybit "github.com/wuhewuhe/bybit.go.api"
)

const (
	CATEGORY_SPOT     = "spot"
	SIDE_BUY          = "Buy"
	SIDE_SELL         = "Sell"
	ORDER_TYPE_MARKET = "Market"
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
	case SIDE_BUY:
		buy(*qty)
	case SIDE_SELL:
		sell(*qty)
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

func buy(qty string) {
	client := bybit.NewBybitHttpClient(viper.GetString("TESTNET_API_KEY"), viper.GetString("TESTNET_API_SECRET"), bybit.WithBaseURL(bybit.TESTNET))
	params := map[string]interface{}{
		"category":  CATEGORY_SPOT,
		"symbol":    "BTCUSDT",
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

func sell(qty string) {
	client := bybit.NewBybitHttpClient(viper.GetString("TESTNET_API_KEY"), viper.GetString("TESTNET_API_SECRET"), bybit.WithBaseURL(bybit.TESTNET))
	params := map[string]interface{}{
		"category":  CATEGORY_SPOT,
		"symbol":    "BTCUSDT",
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
