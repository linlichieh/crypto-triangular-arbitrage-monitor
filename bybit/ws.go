package bybit

import (
	"crypto-triangular-arbitrage-watch/notification"
	"crypto-triangular-arbitrage-watch/runner"
	"crypto-triangular-arbitrage-watch/trade"
	"crypto-triangular-arbitrage-watch/tri"
	"encoding/json"
	"fmt"
	"log"
	"regexp"

	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

type Ws struct {
	Tri               *tri.Tri
	Trade             *trade.Trade
	OrderbookRunner   *runner.OrderbookRunner
	Slack             *notification.Slack
	OrderbookTopicReg *regexp.Regexp
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

func InitWs() *Ws {
	reg, err := regexp.Compile(`^orderbook\..+`)
	if err != nil {
		log.Println("Error compiling regex:", err)
	}

	return &Ws{
		DebugPrintMessage: viper.GetBool("DEBUG_PRINT_MESSAGE"),
		OrderbookTopicReg: reg,
	}
}

func (ws *Ws) SetTri(tri *tri.Tri) {
	ws.Tri = tri
}

func (ws *Ws) SetTrade(trade *trade.Trade) {
	ws.Trade = trade
}

func (ws *Ws) SetOrderbookRunner(orderbookRunner *runner.OrderbookRunner) {
	ws.OrderbookRunner = orderbookRunner
}

func (ws *Ws) SetSlack(slack *notification.Slack) {
	ws.Slack = slack
}

func (ws *Ws) handleResponse(message []byte) error {
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
func (ws *Ws) handleOpResp(message []byte) (bool, error) {
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

func (ws *Ws) handleTopicResp(message []byte) error {
	var topicResp TopicResp
	err := json.Unmarshal(message, &topicResp)
	if err != nil {
		return fmt.Errorf("failed to parse topic message, err: %v", err)
	}

	// To prevent panic, it shouldn't happen, but just in case if Bybit returns unexpected data back
	if topicResp.Topic != "" {
		switch {
		case ws.OrderbookTopicReg.MatchString(topicResp.Topic):
			var data runner.OrderbookData
			err := json.Unmarshal(topicResp.Data, &data)
			if err != nil {
				return fmt.Errorf("failed to parse topic data, err: %v", err)
			}
			// To prevent panic, it shouldn't happen, but just in case if Bybit returns unexpected data back
			if data.Symbol != "" {
				ws.OrderbookRunner.OrderbookListeners[data.Symbol].OrderbookDataCh <- &data
			}
		case topicResp.Topic == "order.spot":
			var list []OrderSpotData
			err := json.Unmarshal(topicResp.Data, &list)
			if err != nil {
				return fmt.Errorf("failed to parse topic 'order.spot' data, err: %v", err)
			}
			for _, data := range list {
				ws.Slack.SystemLogs(fmt.Sprintf("order.spot: %+v", data))
				switch data.Status {
				case "PartiallyFilledCanceled", "Filled":
					var actualQty decimal.Decimal
					switch data.Side {
					case trade.SIDE_BUY:
						cumQty, err := decimal.NewFromString(data.CumQty)
						if err != nil {
							return fmt.Errorf("failed to new decimal 'cumQty' data, err: %v", err)
						}
						cumFee, err := decimal.NewFromString(data.CumFee)
						if err != nil {
							return fmt.Errorf("failed to new decimal 'cumFee' data, err: %v", err)
						}
						actualQty = cumQty.Sub(cumFee)
					case trade.SIDE_SELL:
						cumValue, err := decimal.NewFromString(data.CumValue)
						if err != nil {
							return fmt.Errorf("failed to new decimal 'cumQty' data, err: %v", err)
						}
						cumFee, err := decimal.NewFromString(data.CumFee)
						if err != nil {
							return fmt.Errorf("failed to new decimal 'cumFee' data, err: %v", err)
						}
						actualQty = cumValue.Sub(cumFee)
					}
					ws.Slack.SystemLogs(fmt.Sprintf("actualQty: %s", actualQty.String()))
					ws.Trade.Qty <- actualQty
				case "Cancelled":
					log.Println("Cancelled", data)
					// TODO
					// ws.Trade.Retry <- 1
				}
			}
		case topicResp.Topic == "wallet":
			var list []WalletDataData
			err := json.Unmarshal(topicResp.Data, &list)
			if err != nil {
				return fmt.Errorf("failed to parse topic 'wallet' data, err: %v", err)
			}
			for _, data := range list {
				for _, coin := range data.Coins {
					if coin.Coin == "USDT" {
						bal, err := decimal.NewFromString(coin.Balance)
						if err != nil {
							return err
						}
						ws.Trade.Balance = bal
					}
				}
				ws.Slack.SystemLogs(fmt.Sprintf("wallet coins: %+v", data.Coins))
			}
		}
	}
	return nil
}
