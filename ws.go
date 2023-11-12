package main

import (
	"encoding/json"
	"fmt"

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

type OpResp struct {
	Success bool   `json:"success"`
	Op      string `json:"op"`
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

// There are 2 different formats of response
// operation response e.g. subscribe, ping, auth
// content response e.g. orderbook, wallet
func (ws *WsClient) handleOpResp(message []byte) (bool, error) {
	var opResp OpResp
	err := json.Unmarshal(message, &opResp)
	if err != nil {
		return false, fmt.Errorf("failed to parse op message, err: %v", err)
	}
	// If op isn't empty, it means that it's the response of operation e.g. subscribe or ping
	if opResp.Op != "" {
		if !opResp.Success {
			return false, fmt.Errorf("success: false, response: %s", string(message))
		}

		// Nothing error happens, but it's just the successful operation response. No need to proceed the following step, so return false.
		return false, nil
	}
	return true, nil
}
