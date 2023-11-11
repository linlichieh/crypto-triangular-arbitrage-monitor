package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
	bybit "github.com/wuhewuhe/bybit.go.api"
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

func (ws *WsClient) HandleOrderConnection() {
	for {
		if err := ws.listenOrderStatus(); err != nil {
			ws.Messenger.sendToChannel(ws.Messenger.Channel.SystemLogs, fmt.Sprintf("Order status onnection, error: %v", err))
		}
		ws.Messenger.sendToChannel(ws.Messenger.Channel.SystemLogs, "Order status connection reconnecting...")
		time.Sleep(3 * time.Second)
	}
}

func (ws *WsClient) listenOrderStatus() error {
	conn := bybit.NewBybitPrivateWebSocket(
		viper.GetString("TESTNET_PRIVATE_WS"),
		viper.GetString("TESTNET_API_KEY"),
		viper.GetString("TESTNET_API_SECRET"),
		func(message string) error {
			// TODO send to system logs ???
			// https://bybit-exchange.github.io/docs/v5/websocket/private/order
			// Buy response
			// {
			//   "topic": "order.spot",
			//   "id": "100401071-20000-1252827477",
			//   "creationTime": 1699717992444,
			//   "data": [
			//     {
			//       "category": "spot",
			//       "symbol": "BTCUSDT",
			//       "orderId": "1551741421621614080",
			//       "orderLinkId": "1551741421621614081",
			//       "blockTradeId": "",
			//       "side": "Buy",
			//       "positionIdx": 0,
			//       "orderStatus": "PartiallyFilledCanceled",
			//       "cancelType": "UNKNOWN",
			//       "rejectReason": "EC_CancelForNoFullFill",
			//       "timeInForce": "IOC",
			//       "isLeverage": "0",
			//       "price": "0",
			//       "qty": "10.000000",
			//       "avgPrice": "33393.46",
			//       "leavesQty": "0.000000",
			//       "leavesValue": "0.01535546",
			//       "cumExecQty": "0.000299",
			//       "cumExecValue": "9.98464454",
			//       "cumExecFee": "0.000000299",
			//       "orderType": "Market",
			//       "stopOrderType": "",
			//       "orderIv": "",
			//       "triggerPrice": "0.00",
			//       "takeProfit": "0.00",
			//       "stopLoss": "0.00",
			//       "triggerBy": "",
			//       "tpTriggerBy": "",
			//       "slTriggerBy": "",
			//       "triggerDirection": 0,
			//       "placeType": "",
			//       "lastPriceOnCreated": "33274.95",
			//       "closeOnTrigger": false,
			//       "reduceOnly": false,
			//       "smpGroup": 0,
			//       "smpType": "None",
			//       "smpOrderId": "",
			//       "createdTime": "1699717992439",
			//       "updatedTime": "1699717992442",
			//       "feeCurrency": "BTC"
			//     }
			//   ]
			// }
			// Sell response
			// {
			//   "topic": "order.spot",
			//   "id": "100401071-20000-1252832013",
			//   "creationTime": 1699718691217,
			//   "data": [
			//     {
			//       "category": "spot",
			//       "symbol": "BTCUSDT",
			//       "orderId": "1551747283354392064",
			//       "orderLinkId": "1551747283354392065",
			//       "blockTradeId": "",
			//       "side": "Sell",
			//       "positionIdx": 0,
			//       "orderStatus": "Filled",
			//       "cancelType": "UNKNOWN",
			//       "rejectReason": "EC_NoError",
			//       "timeInForce": "IOC",
			//       "isLeverage": "0",
			//       "price": "0",
			//       "qty": "0.000294",
			//       "avgPrice": "33944.81",
			//       "leavesQty": "0.000000",
			//       "leavesValue": "0.00000000",
			//       "cumExecQty": "0.000294",
			//       "cumExecValue": "9.97977414",
			//       "cumExecFee": "0.00997977414",
			//       "orderType": "Market",
			//       "stopOrderType": "",
			//       "orderIv": "",
			//       "triggerPrice": "0.00",
			//       "takeProfit": "0.00",
			//       "stopLoss": "0.00",
			//       "triggerBy": "",
			//       "tpTriggerBy": "",
			//       "slTriggerBy": "",
			//       "triggerDirection": 0,
			//       "placeType": "",
			//       "lastPriceOnCreated": "33944.83",
			//       "closeOnTrigger": false,
			//       "reduceOnly": false,
			//       "smpGroup": 0,
			//       "smpType": "None",
			//       "smpOrderId": "",
			//       "createdTime": "1699718691211",
			//       "updatedTime": "1699718691215",
			//       "feeCurrency": "USDT"
			//     }
			//   ]
			// }

			log.Println("Received:", message)
			return nil
		},
	)
	err := conn.Connect([]string{"order.spot"})
	if err != nil {
		return fmt.Errorf("failed to subscribe 'order.spot', err: %v", err)
	}
	// var err error
	// conn, _, err := websocket.DefaultDialer.Dial(viper.GetString("TESTNET_PRIVATE_WS"), nil)
	// if err != nil {
	// return fmt.Errorf("failed to dial, err: %v", err)
	// }
	// defer conn.Close()

	// if err = conn.WriteJSON(MessageReq{Op: "subscribe", Args: []string{"order.spot"}}); err != nil {
	// return fmt.Errorf("failed to send op, err: %v", err)
	// }
	// _, message, err := conn.ReadMessage()
	// if err != nil {
	// return fmt.Errorf("failed to read message during running, err: %v", err)
	// }
	// fmt.Println(string(message))
	select {}
}

func (ws *WsClient) HandleOrderbookConnections() {
	var wg sync.WaitGroup
	topics := ws.getOrderbookTopics()
	chunkSize := 10 // bybit only accepts up to 10 symbols per connection
	connNum := 1
	for i := 0; i < len(topics); i += chunkSize {
		end := i + chunkSize
		if end > len(topics) {
			end = len(topics)
		}
		wg.Add(1)
		go ws.listenOrderbooksWithRetry(connNum, topics[i:end])
		connNum++
	}
	wg.Wait()
}

func (ws *WsClient) listenOrderbooksWithRetry(connNum int, topics []string) {
	for {
		if err := ws.listenOrderbooks(connNum, topics); err != nil {
			ws.Messenger.sendToChannel(ws.Messenger.Channel.SystemLogs, fmt.Sprintf("Orderbooks connection(%d) error: %v", connNum, err))
		}
		ws.Messenger.sendToChannel(ws.Messenger.Channel.SystemLogs, fmt.Sprintf("Orderbooks connection(%d) reconnecting...", connNum))
		time.Sleep(3 * time.Second)
	}
}

func (ws *WsClient) listenOrderbooks(connNum int, topics []string) error {
	var err error
	conn, _, err := websocket.DefaultDialer.Dial(viper.GetString("TESTNET_PUBLIC_WS_SPOT"), nil)
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

func (ws *WsClient) getOrderbookTopics() []string {
	var topics []string
	for symbol, _ := range ws.Tri.SymbolCombinationsMap {
		if _, ok := ws.Tri.OrderbookTopics[symbol]; !ok {
			log.Fatalf("Please confirm that orderbook topic of '%s' exists in the config", symbol)
		}
		topics = append(topics, ws.Tri.OrderbookTopics[symbol])
	}
	return topics
}
