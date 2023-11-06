package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

// Rate limit for triangular arbitrage happens
const TRI_ARB_FOUND_INTERVAL_MILLISECOND = 500

// Slack
const SEND_TO_WATCH_INTERVAL_MILLISECOND = 1100
const SEND_TO_SYSTEM_LOGS_INTERVAL_SECOND = 30

const CAPITAL = 1000

type Price []string

type OrderbookRunner struct {
	Tri                  *Tri
	Fee                  decimal.Decimal // 0.01 = 1%
	NetPercent           decimal.Decimal
	OrderbookListeners   map[string]*OrderbookListener
	Messenger            *Messenger
	ChannelWatch         chan string // Message queue
	ChannelSystemLogs    chan string
	DebugPrintMostProfit bool
}

type OrderbookListener struct {
	lastTimeOfTriArbFound time.Time
	ignoreIncomingOrder   bool
	orderbookDataCh       chan *OrderbookData
}

func initOrderbookRunner(tri *Tri) *OrderbookRunner {
	fee := decimal.NewFromFloat(0.001)
	orderbookRunner := &OrderbookRunner{
		Fee:                  fee,
		NetPercent:           decimal.NewFromInt(1).Sub(fee),
		Tri:                  tri,
		OrderbookListeners:   make(map[string]*OrderbookListener),
		ChannelWatch:         make(chan string),
		ChannelSystemLogs:    make(chan string),
		DebugPrintMostProfit: viper.GetBool("DEBUG_PRINT_MOST_PROFIT"),
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
			orderbookDataCh: make(chan *OrderbookData),
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
		case orderbookData := <-listener.orderbookDataCh:
			if listener.ignoreIncomingOrder {
				continue
			}

			// Skip if it's less than interval
			if time.Since(listener.lastTimeOfTriArbFound) <= time.Duration(TRI_ARB_FOUND_INTERVAL_MILLISECOND)*time.Millisecond {
				continue
			}

			listener.ignoreIncomingOrder = true
			or.setOrder(symbol, listener, orderbookData)
		}
	}
}

func (or *OrderbookRunner) setOrder(symbol string, listener *OrderbookListener, orderbookData *OrderbookData) {
	defer func() { listener.ignoreIncomingOrder = false }()

	if len(orderbookData.Bids) > 0 {
		or.Tri.SetOrder(BID, orderbookData.Symbol, orderbookData.Bids[0], orderbookData.Seq)
	}
	if len(orderbookData.Asks) > 0 {
		or.Tri.SetOrder(ASK, orderbookData.Symbol, orderbookData.Asks[0], orderbookData.Seq)
	}

	or.calculateTriangularArbitrage(symbol, listener)
}

func (or *OrderbookRunner) calculateTriangularArbitrage(symbol string, listener *OrderbookListener) {
	// Store the balance for the most profitable combination
	var mostProfitBalance decimal.Decimal
	// Store the most profitable combination
	var mostProfitCombination *Combination

	combinations := or.Tri.SymbolCombinationsMap[symbol]
	if len(combinations) == 0 {
		log.Fatalf("Please check that '%s' is set in the config", symbol)
	}
	for _, combination := range combinations {
		if len(combination.SymbolOrders) < 3 {
			return
		}
		// Make sure all symbols get latest price
		if combination.SymbolOrders[0].Bid == nil ||
			combination.SymbolOrders[0].Ask == nil ||
			combination.SymbolOrders[1].Bid == nil ||
			combination.SymbolOrders[1].Ask == nil ||
			combination.SymbolOrders[2].Bid == nil ||
			combination.SymbolOrders[2].Ask == nil {
			return
		}

		// Calculate the profit
		var balance, secondTrade decimal.Decimal
		capital := decimal.NewFromInt(CAPITAL)
		firstTrade := capital.Div(combination.SymbolOrders[0].Bid.Price).Mul(or.NetPercent)
		if combination.BaseQuote {
			secondTrade = firstTrade.Mul(combination.SymbolOrders[1].Bid.Price).Mul(or.NetPercent)
		} else {
			secondTrade = firstTrade.Div(combination.SymbolOrders[1].Bid.Price).Mul(or.NetPercent)
		}
		thirdTrade := secondTrade.Mul(combination.SymbolOrders[2].Bid.Price).Mul(or.NetPercent)
		balance = thirdTrade.Truncate(4)

		// Store most profitable combination
		if balance.GreaterThan(mostProfitBalance) {
			mostProfitBalance = balance
			mostProfitCombination = combination
		}
	}

	capital := decimal.NewFromInt(CAPITAL)
	msg := fmt.Sprintf(
		"%s %s %s->%s   %s (bid: %s) -> %s (ask: %s) -> %s (ask: %s)",
		time.Now().Format("2006-01-02 15:04:05"),
		symbol,
		capital.String(),
		mostProfitBalance.String(),
		mostProfitCombination.SymbolOrders[0].Symbol,
		mostProfitCombination.SymbolOrders[0].Bid.Price.String(),
		mostProfitCombination.SymbolOrders[1].Symbol,
		mostProfitCombination.SymbolOrders[1].Bid.Price.String(),
		mostProfitCombination.SymbolOrders[2].Symbol,
		mostProfitCombination.SymbolOrders[2].Bid.Price.String(),
	)
	if mostProfitBalance.GreaterThan(capital) {
		listener.lastTimeOfTriArbFound = time.Now()
		or.ChannelWatch <- msg
	}

	if or.DebugPrintMostProfit {
		log.Println(msg)
	}

	or.ChannelSystemLogs <- strconv.FormatInt(mostProfitBalance.IntPart(), 10)
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

	// To show counters for result e.g. `map[997:1762 998:466]` means result 997 gets 1762 times, 998 gets 466 times
	counters := make(map[string]int64)
	for {
		select {
		case result := <-or.ChannelSystemLogs:
			counters[result]++
		case <-ticker.C:
			if len(counters) == 0 {
				continue
			}
			go or.Messenger.sendToSystemLogs(fmt.Sprintf("%+v", counters))

			// flush the combined message
			counters = make(map[string]int64)
		}
	}
}
