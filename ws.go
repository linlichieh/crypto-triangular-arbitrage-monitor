package main

import (
	"encoding/json"
	"fmt"

	"github.com/shopspring/decimal"
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

type TopicResp struct {
	Topic string          `json:"topic"`
	Data  json.RawMessage `json:"data"`
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

func (ws *WsClient) handleResponse(message []byte) error {
	proceed, err := ws.handleOpResp(message)
	if err != nil {
		return err
	}
	if !proceed {
		return nil
	}
	return ws.handleTopicResp(message)
}

// There are 2 different formats of response
// operation response e.g. subscribe, ping, auth
// content response e.g. orderbook, wallet
// bool in response means should it continue to parse the message?
func (ws *WsClient) handleOpResp(message []byte) (bool, error) {
	var opResp OpResp
	err := json.Unmarshal(message, &opResp)
	if err != nil {
		return false, fmt.Errorf("failed to parse op message, err: %v", err)
	}
	// If op isn't empty, it means that it's the response of operation e.g. subscribe or ping
	switch opResp.Op {
	case "subscribe":
		if !opResp.Success {
			return false, fmt.Errorf("success: false, response: %s", string(message))
		}
		return false, nil
	case "ping":
		if !opResp.Success {
			return false, fmt.Errorf("success: false, response: %s", string(message))
		}
		return false, nil
	case "pong":
		// when sending ping to private channel, the response if different from public channel. It doesn't content success.
		return false, nil
	default:
		return true, nil
	}
}

func (ws *WsClient) handleTopicResp(message []byte) error {
	var topicResp TopicResp
	err := json.Unmarshal(message, &topicResp)
	if err != nil {
		return fmt.Errorf("failed to parse topic message, err: %v", err)
	}

	// To prevent panic, it shouldn't happen, but just in case if Bybit returns unexpected data back
	if topicResp.Topic != "" {
		switch topicResp.Topic {
		case "orderbook.1.BTCUSDT":
			var data OrderbookData
			err := json.Unmarshal(topicResp.Data, &data)
			if err != nil {
				return fmt.Errorf("failed to parse topic data, err: %v", err)
			}
			// To prevent panic, it shouldn't happen, but just in case if Bybit returns unexpected data back
			if data.Symbol != "" {
				ws.OrderbookRunner.OrderbookListeners[data.Symbol].orderbookDataCh <- &data
			}
		case "order.spot":
			var list []OrderSpotData
			err := json.Unmarshal(topicResp.Data, &list)
			if err != nil {
				return fmt.Errorf("failed to parse topic 'order.spot' data, err: %v", err)
			}
			for _, data := range list {
				ws.Messenger.SystemLogs(fmt.Sprintf("order.spot: %+v", data))
				if data.Status == "PartiallyFilledCanceled" || data.Status == "Filled" {
					switch data.Side {
					case SIDE_BUY:
						cumQty, err := decimal.NewFromString(data.CumQty)
						if err != nil {
							return fmt.Errorf("failed to new decimal 'cumQty' data, err: %v", err)
						}
						cumFee, err := decimal.NewFromString(data.CumFee)
						if err != nil {
							return fmt.Errorf("failed to new decimal 'cumFee' data, err: %v", err)
						}
						actualQty := cumQty.Sub(cumFee)
						ws.Messenger.SystemLogs(fmt.Sprintf("actualQty: %s", actualQty.String()))
					case SIDE_SELL:
						cumValue, err := decimal.NewFromString(data.CumValue)
						if err != nil {
							return fmt.Errorf("failed to new decimal 'cumQty' data, err: %v", err)
						}
						cumFee, err := decimal.NewFromString(data.CumFee)
						if err != nil {
							return fmt.Errorf("failed to new decimal 'cumFee' data, err: %v", err)
						}
						actualQty := cumValue.Sub(cumFee)
						ws.Messenger.SystemLogs(fmt.Sprintf("actualQty: %s", actualQty.String()))
					}
				}
			}
		case "wallet":
			var list []WalletDataData
			err := json.Unmarshal(topicResp.Data, &list)
			if err != nil {
				return fmt.Errorf("failed to parse topic 'wallet' data, err: %v", err)
			}
			for _, data := range list {
				ws.Messenger.SystemLogs(fmt.Sprintf("wallet coins: %+v", data.Coins))
			}
		}
		// case "execution.spot":
		// DEBUG always send private channel message into systemlogs

		// var data []ExecutionSpotData
		// err := json.Unmarshal(topicResp.Data, &data)
		// if err != nil {
		// return fmt.Errorf("failed to parse topic data, err: %v", err)
		// }
	}
	return nil
}
