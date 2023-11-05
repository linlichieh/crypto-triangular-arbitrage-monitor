package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
)

type WsClient struct {
	Tri             *Tri
	OrderbookRunner *OrderbookRunner
	klines          []string
	Messenger       *Messenger
	Conn            *websocket.Conn
}

type MessageReq struct {
	Op   string   `json:"op"`
	Args []string `json:"args"`
}

// Json
type MessageResp struct {
	// Operation response
	// e.g. {"success":true,"ret_msg":"subscribe","conn_id":"6fc74853-7406-4f7a-8129-7c1267b1d5ac","op":"subscribe"}
	Success bool   `json:"success"`
	Op      string `json:"op"` // ping or subscribe

	// Orderbook message
	// e.g. {"topic":"orderbook.1.BTCUSDC","ts":1699152639644,"type":"delta","data":{"s":"BTCUSDC","b":[["35050.72","0.02853"],["35050.71","0"]],"a":[],"u":16468297,"seq":14234996104}}
	Topic string        `json:"topic"`
	Ts    int64         `json:"ts"`   // ms
	Type  string        `json:"type"` // Data type e.g. snapshot, delta
	Data  OrderbookData `json:"data"`
}

// Json
type OrderbookData struct {
	Symbol   string  `json:"s"`
	Bids     []Price `json:"b"`
	Asks     []Price `json:"a"`
	UpdateId int64   `json:"u"`   // Update ID. It's a sequence. Occasionally, you'll receive "u"=1, which is a snapshot data due to the restart of the service. So please overwrite your local orderbook
	Seq      int64   `json:"seq"` // You can use this field to compare different levels orderbook data, and for the smaller seq, then it means the data is generated earlier.
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
	var err error
	ws.Conn, _, err = websocket.DefaultDialer.Dial(viper.GetString("MAINNET_PUBLIC_WS_SPOT"), nil)
	if err != nil {
		return fmt.Errorf("failed to dial, err: %v", err)
	}
	defer ws.Conn.Close()

	if err = ws.Conn.WriteJSON(MessageReq{Op: "subscribe", Args: ws.klines}); err != nil {
		return fmt.Errorf("failed to send op, err: %v", err)
	}

	// Handle incoming messages
	ws.Messenger.sendToSystemLogs("Listening...")
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// Bybit recommends client to send the ping heartbeat packet every 20 seconds to maintain the WebSocket connection.
			// Otherwise, established connection will close after 5 minutes.
			if err = ws.Conn.WriteJSON(MessageReq{Op: "ping"}); err != nil {
				return fmt.Errorf("failed to send op, err: %v", err)
			}
		default:
			_, message, err := ws.Conn.ReadMessage()
			if err != nil {
				return fmt.Errorf("failed to read message during running, err: %v", err)
			}
			err = ws.handleResponse(message)
			if err != nil {
				return fmt.Errorf("failed to parse message during running, err: %v", err)
			}
		}
	}
}

func (ws *WsClient) handleResponse(message []byte) error {
	var response MessageResp
	err := json.Unmarshal(message, &response)
	if err != nil {
		return fmt.Errorf("failed to parse message after connecting, err: %v", err)
	}
	// If Op is not empty, it means that it's either the response of 'subscribe' or 'ping'
	// In this case, there is no orderbook data in the response and we need to check 'success'
	if response.Op != "" {
		if !response.Success {
			return fmt.Errorf("success: false, response: %+v", response)
		}
		return nil
	}
	// To prevent panic, it shouldn't happen, but just in case if Bybit returns unexpected data back
	if response.Data.Symbol != "" {
		ws.OrderbookRunner.OrderbookListeners[response.Data.Symbol].orderbookDataCh <- &response.Data
	}
	return nil
}
