package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
)

type SubscribeMessage struct {
	Op   string   `json:"op"`
	Args []string `json:"args"`
}

type KlineData struct {
	Start      int64  `json:"start"`
	Timestamp  int64  `json:"timestamp"`
	Symbol     string `json:"symbol"`
	Interval   string `json:"interval"`
	OpenPrice  string `json:"open"`
	ClosePrice string `json:"close"`
	HighPrice  string `json:"high"`
	LowPrice   string `json:"low"`
	Volume     string `json:"volume"`
	Turnover   string `json:"turnover"`
}

type KlineMessage struct {
	Topic string      `json:"topic"`
	Type  string      `json:"type"`
	Data  []KlineData `json:"data"`
}

func connect() {
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
			"kline.1.BTCUSDT",
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

		var klineMsg KlineMessage
		if err := json.Unmarshal(message, &klineMsg); err != nil {
			log.Printf("Error parsing JSON: %v", err)
			continue
		}
		if len(klineMsg.Data) > 0 {
			t := time.Unix(0, klineMsg.Data[0].Timestamp*int64(time.Millisecond))
			fmt.Printf("[%s] %s %s\n", t.Format("2006-01-02 15:04:05"), klineMsg.Topic, klineMsg.Data[0].ClosePrice)
		}
	}
}
