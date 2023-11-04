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

type WsClient struct {
	Tri             *Tri
	OrderbookRunner *OrderbookRunner
	klines          []string
	Messenger       *Messenger
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

func (ws *WsClient) setMessenger(messenger *Messenger) {
	ws.Messenger = messenger
}

func (ws *WsClient) ConnectToBybit() {
	for {
		if err := ws.connect(); err != nil {
			ws.Messenger.sendToSystemLogs(fmt.Sprintf("Websocket error: %v", err))
		}
		ws.Messenger.sendToSystemLogs("Reconnecting...")
		time.Sleep(3 * time.Second)
	}
}

func (ws *WsClient) connect() error {
	conn, _, err := websocket.DefaultDialer.Dial(viper.GetString("MAINNET_PUBLIC_WS_SPOT"), nil)
	if err != nil {
		return fmt.Errorf("failed to dial, err: %v", err)
	}
	defer conn.Close()

	// Subscribe to a spot channel, for example: klineV2.1.BTCUSDT.1
	subscribePayload := SubscribeMessage{
		Op:   "subscribe",
		Args: ws.klines,
	}

	err = conn.WriteJSON(subscribePayload)
	if err != nil {
		return fmt.Errorf("failed to write json, err: %v", err)
	}
	_, message, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read message after connecting, err: %v", err)
	}
	var parsedMsg map[string]interface{}
	if err := json.Unmarshal(message, &parsedMsg); err != nil {
		return fmt.Errorf("failed to parse message after connecting, err: %v", err)
	}
	if !parsedMsg["success"].(bool) {
		return fmt.Errorf("ws response, err: %v", err)
	}

	// Handle incoming messages
	ws.Messenger.sendToSystemLogs("Running successfully!")
	ws.Messenger.sendToSystemLogs("Listening...")
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("failed to read message during running, err: %v", err)
		}
		var orderbookMsg OrderbookMsg
		if err := json.Unmarshal(message, &orderbookMsg); err != nil {
			return fmt.Errorf("failed to parse message during running, err: %v", err)
		}
		ws.OrderbookRunner.OrderbookListeners[orderbookMsg.Data.Symbol].orderbookMessageCh <- &orderbookMsg
	}
}

func msToTime(ms int64) time.Time {
	return time.Unix(0, ms*int64(time.Millisecond))
}
