package bybit

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
)

func (b *Bybit) HandlePublicChannel() {
	var wg sync.WaitGroup
	topics := b.getOrderbookTopics()
	chunkSize := 10 // bybit only accepts up to 10 symbols per connection
	connNum := 1
	for i := 0; i < len(topics); i += chunkSize {
		end := i + chunkSize
		if end > len(topics) {
			end = len(topics)
		}
		wg.Add(1)
		go b.listenOrderbooksWithRetry(connNum, topics[i:end])
		connNum++
	}
	wg.Wait()
}

func (b *Bybit) listenOrderbooksWithRetry(connNum int, topics []string) {
	for {
		if err := b.listenOrderbooks(connNum, topics); err != nil {
			b.Slack.SystemLogs(fmt.Sprintf("Orderbooks connection(%d) error: %v", connNum, err))
		}
		b.Slack.SystemLogs(fmt.Sprintf("Orderbooks connection(%d) reconnecting...", connNum))
		time.Sleep(3 * time.Second)
	}
}

func (b *Bybit) listenOrderbooks(connNum int, topics []string) error {
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
	b.Slack.SystemLogs(fmt.Sprintf("Orderbooks connection(%d) listening...", connNum))
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
			if b.DebugPrintMessage {
				log.Println("orderbook:", string(message))
			}
			err = b.handleResponse(message)
			if err != nil {
				return fmt.Errorf("failed to parse orderbook message during running, err: %v", err)
			}
		case err := <-errChan:
			return err
		}
	}
}

func (b *Bybit) getOrderbookTopics() []string {
	var topics []string
	for symbol, _ := range b.Tri.SymbolCombinationsMap {
		if _, ok := b.Tri.OrderbookTopics[symbol]; !ok {
			log.Fatalf("Please confirm that orderbook topic of '%s' exists in the config", symbol)
		}
		topics = append(topics, b.Tri.OrderbookTopics[symbol])
	}
	return topics
}