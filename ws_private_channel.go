package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
)

func (ws *WsClient) HandlePrivateChannel() {
	for {
		if err := ws.listenOrderStatus([]string{"order.spot", "execution.spot", "wallet"}); err != nil {
			ws.Messenger.SystemLogs(fmt.Sprintf("Order status connection, error: %v", err))
		}
		ws.Messenger.SystemLogs("Order status connection reconnecting...")
		time.Sleep(3 * time.Second)
	}
}

func (ws *WsClient) listenOrderStatus(topics []string) error {
	header := make(http.Header)
	header.Set("api_key", viper.GetString("BYBIT_API_KEY"))
	header.Set("api_secret", viper.GetString("BYBIT_API_SECRET"))

	var err error
	conn, _, err := websocket.DefaultDialer.Dial(viper.GetString("BYBIT_PRIVATE_WS"), nil)
	if err != nil {
		return fmt.Errorf("failed to dial, err: %v", err)
	}
	defer conn.Close()

	// Auth message
	expires := strconv.FormatInt(time.Now().Unix()*1000+1000, 10)
	signature := ws.generateSignature(expires)
	req := MessageReq{
		Op:   "auth",
		Args: []string{viper.GetString("BYBIT_API_KEY"), expires, signature},
	}
	if err = conn.WriteJSON(req); err != nil {
		return fmt.Errorf("failed to send op, err: %v", err)
	}

	// Check auth message
	_, message, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read auth message, err: %v", err)
	}
	var opResp OpResp
	err = json.Unmarshal(message, &opResp)
	if err != nil {
		return fmt.Errorf("failed to parse auth response, err: %v", err)
	}
	if opResp.Op == "auth" {
		if !opResp.Success {
			return fmt.Errorf("failed to auth, err: %v", err)
		}
	}
	ws.Messenger.SystemLogs("auth succeed!")

	// Subscribe order status, wallet, etc.
	if err = conn.WriteJSON(MessageReq{Op: "subscribe", Args: topics}); err != nil {
		return fmt.Errorf("failed to send op, args: %v, err: %v", topics, err)
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

	// Listen to response
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err = conn.WriteJSON(MessageReq{Op: "ping"}); err != nil {
				return fmt.Errorf("failed to send op, err: %v", err)
			}
		case message := <-msgChan:
			if ws.DebugPrintMessage {
				log.Println("private:", string(message))
			}
			err = ws.handlePrivateResponse(message)
			if err != nil {
				return fmt.Errorf("failed to parse private message during running, err: %v", err)
			}
		case err := <-errChan:
			return err
		}
	}
}

func (ws *WsClient) handlePrivateResponse(message []byte) error {
	proceed, err := ws.handleOpResp(message)
	if err != nil {
		return err
	}
	if !proceed {
		return nil
	}

	// TODO
	return nil
}

func (ws *WsClient) generateSignature(expires string) string {
	apiSecret := viper.GetString("BYBIT_API_SECRET")

	// Create a new HMAC by defining the hash type and the key (as byte array)
	h := hmac.New(sha256.New, []byte(apiSecret))

	// Write Data to it
	h.Write([]byte(fmt.Sprintf("GET/realtime%s", expires)))

	// Get result and encode as hexadecimal string
	signature := hex.EncodeToString(h.Sum(nil))

	return signature
}
