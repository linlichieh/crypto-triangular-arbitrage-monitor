package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
)

type SubscribeMessage struct {
	Op   string   `json:"op"`
	Args []string `json:"args"`
}

func connectToBybit(messenger *Messenger) {
	// Connect to Bybit Testnet
	conn, _, err := websocket.DefaultDialer.Dial(viper.GetString("MAINNET_PUBLIC_WS_SPOT"), nil)
	if err != nil {
		log.Fatalf("Error connecting: %v", err)
	}
	defer conn.Close()

	// Subscribe to a spot channel, for example: klineV2.1.BTCUSDT.1
	subscribePayload := SubscribeMessage{
		Op: "subscribe",
		Args: []string{
			// "kline.1.BTCUSDT",
			"orderbook.1.BTCUSDT",
			"orderbook.1.ETHUSDT",
			"orderbook.1.ETHBTC",
		},
	}

	err = conn.WriteJSON(subscribePayload)
	if err != nil {
		log.Fatalf("Error subscribing: %v", err)
	}

	// Handle incoming messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading message: %v", err)
			break
		}
		var parsedMsg map[string]interface{}
		if err := json.Unmarshal(message, &parsedMsg); err != nil {
			log.Printf("Error parsing JSON: %v", err)
			continue
		}

		var orderbookMsg OrderbookMessage
		if err := json.Unmarshal(message, &orderbookMsg); err != nil {
			log.Printf("Error parsing JSON: %v", err)
			continue
		}
		messenger.OrderbookMessageChan <- &orderbookMsg
	}
}

func msToTime(ms int64) time.Time {
	return time.Unix(0, ms*int64(time.Millisecond))
}
