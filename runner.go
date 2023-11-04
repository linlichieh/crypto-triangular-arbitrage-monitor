package main

import (
	"fmt"
	"log"
	"time"

	"github.com/shopspring/decimal"
)

// Rate limit for triangular arbitrage happens
const TRI_ARB_FOUND_INTERVAL_MILLISECOND = 500

// Slack
const SEND_TO_WATCH_INTERVAL_MILLISECOND = 1100
const SEND_TO_SYSTEM_LOGS_INTERVAL_SECOND = 30

const CAPITAL = 1000

type Price []string

type OrderbookRunner struct {
	Tri                *Tri
	Fee                decimal.Decimal // 0.01 = 1%
	NetPercent         decimal.Decimal
	OrderbookListeners map[string]*OrderbookListener
	Messenger          *Messenger
	ChannelWatch       chan string // Message queue
	ChannelSystemLogs  chan string
}

type OrderbookListener struct {
	lastTimeOfTriArbFound time.Time
	ignoreIncomingOrder   bool
	orderbookMessageCh    chan *OrderbookMsg
}

// Json
type OrderbookMsg struct {
	Topic string        `json:"topic"`
	Ts    int64         `json:"ts"`   // ms
	Type  string        `json:"type"` // Data type. snapshot,delta
	Data  OrderbookData `json:"data"`
}

// Json
type OrderbookData struct {
	Symbol   string  `json:"s"`
	Bids     []Price `json:"b"`
	Asks     []Price `json:"a"`
	UpdateId int64   `json:"u"`   // Update ID. Is a sequence. Occasionally, you'll receive "u"=1, which is a snapshot data due to the restart of the service. So please overwrite your local orderbook
	Seq      int64   `json:"seq"` // You can use this field to compare different levels orderbook data, and for the smaller seq, then it means the data is generated earlier.
}

func initOrderbookRunner(tri *Tri) *OrderbookRunner {
	fee := decimal.NewFromFloat(0.001)
	orderbookRunner := &OrderbookRunner{
		Fee:                fee,
		NetPercent:         decimal.NewFromInt(1).Sub(fee),
		Tri:                tri,
		OrderbookListeners: make(map[string]*OrderbookListener),
		ChannelWatch:       make(chan string),
		ChannelSystemLogs:  make(chan string),
	}
	orderbookRunner.initOrderbookListeners()
	return orderbookRunner
}

func (or *OrderbookRunner) setMessenger(messenger *Messenger) {
	or.Messenger = messenger
}

func (or *OrderbookRunner) initOrderbookListeners() {
	for symbol, _ := range or.Tri.SymbolOrdersMap {
		or.OrderbookListeners[symbol] = &OrderbookListener{
			orderbookMessageCh: make(chan *OrderbookMsg),
		}
	}
}

func (or *OrderbookRunner) ListenAll() {
	for symbol, _ := range or.OrderbookListeners {
		go or.listenOrderbook(symbol)
	}

	// Send messages to slack
	go or.sendToWatch()
	go or.sendToSystemLogs()
}

func (or *OrderbookRunner) listenOrderbook(symbol string) {
	listener := or.OrderbookListeners[symbol]
	for {
		select {
		case orderbookMsg := <-listener.orderbookMessageCh:
			if listener.ignoreIncomingOrder {
				continue
			}

			// Skip if it's less than interval
			if time.Since(listener.lastTimeOfTriArbFound) <= time.Duration(TRI_ARB_FOUND_INTERVAL_MILLISECOND)*time.Millisecond {
				continue
			}

			listener.ignoreIncomingOrder = true
			or.setOrder(symbol, listener, orderbookMsg)
		}
	}
}

func (or *OrderbookRunner) setOrder(symbol string, listener *OrderbookListener, orderbookMsg *OrderbookMsg) {
	defer func() { listener.ignoreIncomingOrder = false }()

	ts := msToTime(orderbookMsg.Ts)
	if len(orderbookMsg.Data.Bids) > 0 {
		or.Tri.SetOrder(BID, ts, orderbookMsg.Data.Symbol, orderbookMsg.Data.Bids[0])
	}
	if len(orderbookMsg.Data.Asks) > 0 {
		or.Tri.SetOrder(ASK, ts, orderbookMsg.Data.Symbol, orderbookMsg.Data.Asks[0])
	}

	or.calculateTriangularArbitrage(symbol, listener)
}

func (or *OrderbookRunner) calculateTriangularArbitrage(symbol string, listener *OrderbookListener) {
	var mostProfitPrice decimal.Decimal
	var mostProfitCombination *Combination

	combinations := or.Tri.SymbolCombinationsMap[symbol]
	for _, combination := range combinations {
		if len(combination.SymbolOrders) < 3 {
			return
		}
		if combination.SymbolOrders[0].Bid == nil ||
			combination.SymbolOrders[0].Ask == nil ||
			combination.SymbolOrders[1].Bid == nil ||
			combination.SymbolOrders[1].Ask == nil ||
			combination.SymbolOrders[2].Bid == nil ||
			combination.SymbolOrders[2].Ask == nil {
			log.Println("Warning: at least one of bids or prices is nil, please wait for all prices are set")
			return
		}

		// Calculate the profit
		var result, secondTrade decimal.Decimal
		capital := decimal.NewFromInt(CAPITAL)
		firstTrade := capital.Div(combination.SymbolOrders[0].Bid.Price).Mul(or.NetPercent)
		if combination.BaseQuote {
			secondTrade = firstTrade.Mul(combination.SymbolOrders[1].Bid.Price).Mul(or.NetPercent)
		} else {
			secondTrade = firstTrade.Div(combination.SymbolOrders[1].Bid.Price).Mul(or.NetPercent)
		}
		thirdTrade := secondTrade.Mul(combination.SymbolOrders[2].Bid.Price).Mul(or.NetPercent)
		result = thirdTrade.Truncate(4)

		// Store most profitable combination
		if result.GreaterThan(mostProfitPrice) {
			mostProfitPrice = result
			mostProfitCombination = combination
		}
	}

	capital := decimal.NewFromInt(CAPITAL)
	if mostProfitPrice.GreaterThan(capital) {
		msg := fmt.Sprintf(
			"%s %s %s->%s   %s (bid: %s) -> %s (ask: %s) -> %s (ask: %s)",
			time.Now().Format("2006-01-02 15:04:05"),
			symbol,
			capital.String(),
			mostProfitPrice.String(),
			mostProfitCombination.SymbolOrders[0].Symbol,
			mostProfitCombination.SymbolOrders[0].Bid.Price.String(),
			mostProfitCombination.SymbolOrders[1].Symbol,
			mostProfitCombination.SymbolOrders[1].Bid.Price.String(),
			mostProfitCombination.SymbolOrders[2].Symbol,
			mostProfitCombination.SymbolOrders[2].Bid.Price.String(),
		)
		listener.lastTimeOfTriArbFound = time.Now()
		or.ChannelWatch <- msg
	}

	// Like health check
	or.ChannelSystemLogs <- "."
}

// Send to slack every second in case hit the ceiling of rate limits
func (or *OrderbookRunner) sendToWatch() {
	ticker := time.NewTicker(time.Duration(SEND_TO_WATCH_INTERVAL_MILLISECOND) * time.Millisecond)
	defer ticker.Stop()

	var combinedMsg string
	for {
		select {
		case msg := <-or.ChannelWatch:
			combinedMsg += fmt.Sprintf("%s\n", msg)
		case <-ticker.C:
			if combinedMsg == "" {
				continue
			}
			go or.Messenger.sendToWatch(combinedMsg)

			// flush the combined message
			combinedMsg = ""
		}
	}
}

func (or *OrderbookRunner) sendToSystemLogs() {
	ticker := time.NewTicker(time.Duration(SEND_TO_SYSTEM_LOGS_INTERVAL_SECOND) * time.Second)
	defer ticker.Stop()

	var combinedMsg string
	for {
		select {
		case msg := <-or.ChannelSystemLogs:
			combinedMsg += msg
		case <-ticker.C:
			if combinedMsg == "" {
				continue
			}
			go or.Messenger.sendToSystemLogs(fmt.Sprintf("Checked %d times", len(combinedMsg)))

			// flush the combined message
			combinedMsg = ""
		}
	}
}
