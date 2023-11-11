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
const TRI_ARB_FOUND_INTERVAL_MILLISECOND = 300

// Slack
const SEND_TO_WATCH_INTERVAL_SECOND = 2
const SEND_TO_SYSTEM_LOGS_INTERVAL_SECOND = 30

const CAPITAL = 1000

// Only place the order when it is over the target profit threshold
const TARGET_PROFIT_FOR_TRADE = 0.001

type Price []string

type OrderbookRunner struct {
	Tri                  *Tri
	Fee                  decimal.Decimal // 0.01 = 1%
	NetPercent           decimal.Decimal
	OrderbookListeners   map[string]*OrderbookListener
	Messenger            *Messenger
	ChannelWatch         chan *MostProfit
	ChannelSystemLogs    chan *MostProfit
	DebugPrintMostProfit bool
}

type OrderbookListener struct {
	lastTimeOfTriArbFound time.Time
	ignoreIncomingOrder   bool
	orderbookDataCh       chan *OrderbookData
}

type MostProfit struct {
	// Which symbol trigger the combination calculation
	Symbol string
	// Store the balance for the most profitable combination
	RemainingBalance decimal.Decimal
	// Store the most profitable combination
	Combination *Combination
}

func initOrderbookRunner(tri *Tri) *OrderbookRunner {
	fee := decimal.NewFromFloat(0.001)
	orderbookRunner := &OrderbookRunner{
		Fee:                  fee,
		NetPercent:           decimal.NewFromInt(1).Sub(fee),
		Tri:                  tri,
		OrderbookListeners:   make(map[string]*OrderbookListener),
		ChannelWatch:         make(chan *MostProfit),
		ChannelSystemLogs:    make(chan *MostProfit),
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
	go or.handleChannelWatch()
	go or.handleChannelSystemLogs()
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
	mostProfit := MostProfit{Symbol: symbol}

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
		if balance.GreaterThan(mostProfit.RemainingBalance) {
			mostProfit.RemainingBalance = balance
			mostProfit.Combination = combination
		}
	}

	capital := decimal.NewFromInt(CAPITAL)
	profitPercent := mostProfit.RemainingBalance.Sub(capital).Div(capital)
	if profitPercent.GreaterThanOrEqual(decimal.NewFromFloat(TARGET_PROFIT_FOR_TRADE)) {
		listener.lastTimeOfTriArbFound = time.Now()
		or.ChannelWatch <- &mostProfit
	}
	or.ChannelSystemLogs <- &mostProfit

	if or.DebugPrintMostProfit {
		log.Println(mostProfit.tradeMsg())
	}
}

// Send to slack every second in case hit the ceiling of rate limits
func (or *OrderbookRunner) handleChannelWatch() {
	ticker := time.NewTicker(time.Duration(SEND_TO_WATCH_INTERVAL_SECOND) * time.Second)
	defer ticker.Stop()

	var combinedMsg string
	mostProfitMap := make(map[*Combination]*MostProfit)
	for {
		select {
		case mostProfit := <-or.ChannelWatch:
			if _, ok := mostProfitMap[mostProfit.Combination]; !ok {
				mostProfitMap[mostProfit.Combination] = mostProfit
			}
		case <-ticker.C:
			if len(mostProfitMap) == 0 {
				continue
			}

			for _, mostProfit := range mostProfitMap {
				combinedMsg += fmt.Sprintf("%s\n", mostProfit.tradeMsg())
			}
			go or.Messenger.sendToChannel(or.Messenger.Channel.Watch, combinedMsg)

			// flush the combined message
			combinedMsg = ""
			mostProfitMap = make(map[*Combination]*MostProfit)
		}
	}
}

func (or *OrderbookRunner) handleChannelSystemLogs() {
	ticker := time.NewTicker(time.Duration(SEND_TO_SYSTEM_LOGS_INTERVAL_SECOND) * time.Second)
	defer ticker.Stop()

	// To show counters for result e.g. `map[997:1762 998:466]` means result 997 gets 1762 times, 998 gets 466 times
	counters := make(map[string]int64)
	for {
		select {
		case mostProfit := <-or.ChannelSystemLogs:
			balance := strconv.FormatInt(mostProfit.RemainingBalance.IntPart(), 10)
			counters[balance]++
		case <-ticker.C:
			if len(counters) == 0 {
				continue
			}
			go or.Messenger.sendToChannel(or.Messenger.Channel.SystemLogs, fmt.Sprintf("%s %+v", time.Now().UTC().Add(8*time.Hour).Format("15:04:05"), counters))

			// flush the combined message
			counters = make(map[string]int64)
		}
	}
}

func (p *MostProfit) tradeMsg() string {
	return fmt.Sprintf(
		"%s %s->%s  [%s]  %s -> %s -> %s",
		time.Now().UTC().Add(8*time.Hour).Format("15:04:05"),
		decimal.NewFromInt(CAPITAL).String(),
		p.RemainingBalance.StringFixed(1),
		p.Symbol,
		p.Combination.SymbolOrders[0].Symbol,
		p.Combination.SymbolOrders[1].Symbol,
		p.Combination.SymbolOrders[2].Symbol,
	)
}
