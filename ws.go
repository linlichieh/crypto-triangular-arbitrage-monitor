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

type WsClient struct {
	Tri             *Tri
	OrderbookRunner *OrderbookRunner
	klines          []string
}

func initWsClient(tri *Tri, orderbookRunner *OrderbookRunner) *WsClient {
	klines := tri.initWsKlines()
	if len(klines) == 0 {
		log.Fatal("There is no klines to listen")
	}
	return &WsClient{
		Tri:             tri,
		OrderbookRunner: orderbookRunner,
		klines:          klines,
	}
}

func (c *WsClient) ConnectToBybit() {
	conn, _, err := websocket.DefaultDialer.Dial(viper.GetString("MAINNET_PUBLIC_WS_SPOT"), nil)
	if err != nil {
		log.Fatalf("Error connecting: %v", err)
	}
	defer conn.Close()

	// Subscribe to a spot channel, for example: klineV2.1.BTCUSDT.1
	subscribePayload := SubscribeMessage{
		Op:   "subscribe",
		Args: c.klines,
	}

	err = conn.WriteJSON(subscribePayload)
	if err != nil {
		log.Fatalf("Error subscribing: %v", err)
	}
	_, message, err := conn.ReadMessage()
	if err != nil {
		log.Fatal(err)
	}
	var parsedMsg map[string]interface{}
	if err := json.Unmarshal(message, &parsedMsg); err != nil {
		log.Fatalf("Error parsing JSON: %v", err)
	}
	if !parsedMsg["success"].(bool) {
		log.Fatal("Failed to connect WS, err:", string(message))
	}

	// Handle incoming messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading message: %v", err)
			break
		}
		var orderbookMsg OrderbookMsg
		if err := json.Unmarshal(message, &orderbookMsg); err != nil {
			log.Printf("Error parsing JSON: %v", err)
			continue
		}
		c.OrderbookRunner.OrderbookListeners[orderbookMsg.Data.Symbol].orderbookMessageCh <- &orderbookMsg
	}
}

func msToTime(ms int64) time.Time {
	return time.Unix(0, ms*int64(time.Millisecond))
}
