package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
)

type WsClient struct {
	Tri               *Tri
	OrderbookRunner   *OrderbookRunner
	Messenger         *Messenger
	DebugPrintMessage bool
	ListeningTopics   []string
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
	return &WsClient{
		Tri:               tri,
		OrderbookRunner:   orderbookRunner,
		DebugPrintMessage: viper.GetBool("DEBUG_PRINT_MESSAGE"),
	}
}

func (ws *WsClient) setMessenger(messenger *Messenger) {
	ws.Messenger = messenger
}

func (ws *WsClient) HandleConnections() {
	var wg sync.WaitGroup
	topics := ws.getListeningTopics()
	chunkSize := 10 // bybit only accepts up to 10 symbols per connection
	connNum := 1
	for i := 0; i < len(topics); i += chunkSize {
		end := i + chunkSize
		if end > len(topics) {
			end = len(topics)
		}
		wg.Add(1)
		go ws.connectWithRetry(connNum, topics[i:end])
		connNum++
	}
	wg.Wait()
}

func (ws *WsClient) connectWithRetry(connNum int, topics []string) {
	for {
		if err := ws.connect(connNum, topics); err != nil {
			ws.Messenger.sendToChannel(ws.Messenger.Channel.SystemLogs, fmt.Sprintf("Connection(%d) error: %v", connNum, err))
		}
		ws.Messenger.sendToChannel(ws.Messenger.Channel.SystemLogs, fmt.Sprintf("Connection(%d) reconnecting...", connNum))
		time.Sleep(3 * time.Second)
	}
}

func (ws *WsClient) connect(connNum int, topics []string) error {
	var err error
	conn, _, err := websocket.DefaultDialer.Dial(viper.GetString("MAINNET_PUBLIC_WS_SPOT"), nil)
	if err != nil {
		return fmt.Errorf("failed to dial, err: %v", err)
	}
	defer conn.Close()

	if err = conn.WriteJSON(MessageReq{Op: "subscribe", Args: topics}); err != nil {
		return fmt.Errorf("failed to send op, err: %v", err)
	}

	// Handle incoming messages
	ws.Messenger.sendToChannel(ws.Messenger.Channel.SystemLogs, fmt.Sprintf("Connection(%d) listening...", connNum))
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// Bybit recommends client to send the ping heartbeat packet every 20 seconds to maintain the WebSocket connection.
			// Otherwise, established connection will close after 5 minutes.
			if err = conn.WriteJSON(MessageReq{Op: "ping"}); err != nil {
				return fmt.Errorf("failed to send op, err: %v", err)
			}
		default:
			_, message, err := conn.ReadMessage()
			if err != nil {
				return fmt.Errorf("failed to read message during running, err: %v", err)
			}
			if ws.DebugPrintMessage {
				log.Println(string(message))
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

func (ws *WsClient) getListeningTopics() []string {
	var topics []string
	for symbol, _ := range ws.Tri.SymbolCombinationsMap {
		if _, ok := ws.Tri.OrderbookTopics[symbol]; !ok {
			log.Fatalf("Please confirm that orderbook topic of '%s' exists in the config", symbol)
		}
		topics = append(topics, ws.Tri.OrderbookTopics[symbol])
	}
	return topics
}
