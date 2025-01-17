package bybit

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
)

func (ws *Ws) HandlePublicChannel() {
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

func (ws *Ws) listenOrderbooksWithRetry(connNum int, topics []string) {
	for {
		if err := ws.listenOrderbooks(connNum, topics); err != nil {
			ws.Slack.SystemLogs(fmt.Sprintf("Orderbooks connection(%d) error: %v", connNum, err))
		}
		ws.Slack.SystemLogs(fmt.Sprintf("Orderbooks connection(%d) reconnecting...", connNum))
		time.Sleep(3 * time.Second)
	}
}

func (ws *Ws) listenOrderbooks(connNum int, topics []string) error {
	var err error
	conn, _, err := websocket.DefaultDialer.Dial(viper.GetString("BYBIT_PUBLIC_WS_SPOT"), nil)
	if err != nil {
		return fmt.Errorf("failed to dial, err: %v", err)
	}
	defer conn.Close()

	if err = conn.WriteJSON(MessageReq{Op: "subscribe", Args: topics}); err != nil {
		return fmt.Errorf("failed to send op, err: %v", err)
	}

	// In order to prevent `conn.ReadMessage()` from blocking if there is no update pushed from bybit and ping won't be
	// executed due to this reason, it needed to be run in another goroutine
	msgChan := make(chan []byte)
	errChan := make(chan error)
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				errChan <- fmt.Errorf("failed to read message during running, err: %v", err)
				return
			}
			msgChan <- message
		}
	}()

	// Handle incoming messages
	ws.Slack.SystemLogs(fmt.Sprintf("Orderbooks connection(%d) listening...", connNum))
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
		case message := <-msgChan:
			if ws.DebugPrintMessage {
				log.Println("orderbook:", string(message))
			}
			err = ws.handleResponse(message)
			if err != nil {
				return fmt.Errorf("failed to parse orderbook message during running, err: %v", err)
			}
		case err := <-errChan:
			return err
		}
	}
}

func (ws *Ws) getOrderbookTopics() []string {
	var topics []string
	for symbol, _ := range ws.Tri.SymbolCombinationsMap {
		if _, ok := ws.Tri.OrderbookTopics[symbol]; !ok {
			log.Fatalf("Please confirm that orderbook topic of '%s' exists in the config", symbol)
		}
		topics = append(topics, ws.Tri.OrderbookTopics[symbol])
	}
	return topics
}
