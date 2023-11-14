package main

import (
	"crypto-triangular-arbitrage-watch/notification"
	"crypto-triangular-arbitrage-watch/runner"
	"crypto-triangular-arbitrage-watch/tri"
	"crypto-triangular-arbitrage-watch/ws"
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
	tri.SetSlack(slack)
	tri.PrintAllSymbols()
	// tri.printAllCombinations()

	orderbookRunner := runner.Init(tri)
	orderbookRunner.SetSlack(slack)
	go orderbookRunner.ListenAll()

	// Have to be after initTri as it will set klines
	wsClient := ws.Init()
	wsClient.SetTri(tri)
	wsClient.SetOrderbookRunner(orderbookRunner)
	wsClient.SetSlack(slack)
	go wsClient.HandlePrivateChannel() // block
	wsClient.HandlePublicChannel()     // block
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
