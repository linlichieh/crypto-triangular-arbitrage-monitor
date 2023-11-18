package main

import (
	"crypto-triangular-arbitrage-watch/bybit"
	"crypto-triangular-arbitrage-watch/notification"
	"crypto-triangular-arbitrage-watch/runner"
	"crypto-triangular-arbitrage-watch/trade"
	"crypto-triangular-arbitrage-watch/tri"
	"fmt"
	"log"
	"strings"

	"github.com/spf13/viper"
)

func main() {
	loadEnvConfig()

	slack := notification.Init()
	// Listen incoming messages and store these messages into buffer, so that it won't reach the rate limits of slack
	go slack.HandleChannelSystemLogs()
	slack.SystemLogs("Config has been loaded successfully.")
	slack.SystemLogs(fmt.Sprintf("ENV: %s", viper.GetString("ENV")))
	log.Println("DEBUG_PRINT_MESSAGE:", viper.GetBool("DEBUG_PRINT_MESSAGE"))
	log.Println("DEBUG_PRINT_MOST_PROFIT:", viper.GetBool("DEBUG_PRINT_MOST_PROFIT"))

	tri := tri.Init()
	tri.Build()
	tri.SetSlack(slack)
	tri.PrintAllSymbols()
	// tri.printAllCombinations()

	orderbookRunner := runner.Init(tri)
	orderbookRunner.SetSlack(slack)
	go orderbookRunner.ListenAll()

	// Trade
	tra := trade.Init()

	// Have to be after initTri as it will set klines
	ws := bybit.InitWs()
	ws.SetTrade(tra)
	ws.SetTri(tri)
	ws.SetOrderbookRunner(orderbookRunner)
	ws.SetSlack(slack)
	go ws.HandlePrivateChannel() // block
	ws.HandlePublicChannel()     // block
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
