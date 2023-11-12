package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/spf13/viper"
)

func main() {
	loadEnvConfig()

	messenger := initMessenger()
	// Listen incoming messages and store these messages into buffer, so that it won't reach the rate limits of slack
	go messenger.handleChannelSystemLogs()
	messenger.SystemLogs("Config has been loaded successfully.")
	messenger.SystemLogs(fmt.Sprintf("ENV: %s", viper.GetString("ENV")))
	log.Println("DEBUG_PRINT_MESSAGE:", viper.GetBool("DEBUG_PRINT_MESSAGE"))
	log.Println("DEBUG_PRINT_MOST_PROFIT:", viper.GetBool("DEBUG_PRINT_MOST_PROFIT"))

	tri := initTri()
	tri.setMessenger(messenger)
	tri.printAllSymbols()
	// tri.printAllCombinations()

	orderbookRunner := initOrderbookRunner(tri)
	orderbookRunner.setMessenger(messenger)
	go orderbookRunner.ListenAll()

	// Have to be after initTri as it will set klines
	wsClient := initWsClient(tri, orderbookRunner)
	wsClient.setMessenger(messenger)
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
