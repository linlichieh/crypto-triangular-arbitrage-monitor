package main

import (
	"fmt"
	"log"
	"time"

	"github.com/shopspring/decimal"
)

// Prevent triangular arbitrage happened all of sudden that causes issues
const TRI_ARB_FOUND_LIMIT_INTERVAL_MS = 1200

type Price []string

type OrderbookRunner struct {
	Tri                   *Tri
	Fee                   decimal.Decimal // 0.01 = 1%
	NetPercent            decimal.Decimal
	OrderbookListeners    map[string]*OrderbookListener
	Messenger             *Messenger
	LastTimeOfTriArbFound time.Time
}

type OrderbookListener struct {
	ignoreIncomingOrder bool
	orderbookMessageCh  chan *OrderbookMsg
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
}

func (or *OrderbookRunner) listenOrderbook(symbol string) {
	listener := or.OrderbookListeners[symbol]
	for {
		select {
		case orderbookMsg := <-listener.orderbookMessageCh:
			if listener.ignoreIncomingOrder {
				continue
			}
			listener.ignoreIncomingOrder = true
			go or.setOrder(symbol, orderbookMsg)
		}
	}
}

func (or *OrderbookRunner) setOrder(symbol string, orderbookMsg *OrderbookMsg) {
	listener := or.OrderbookListeners[symbol]
	defer func() { listener.ignoreIncomingOrder = false }()

	ts := msToTime(orderbookMsg.Ts)
	if len(orderbookMsg.Data.Bids) > 0 {
		or.Tri.SetOrder(BID, ts, orderbookMsg.Data.Symbol, orderbookMsg.Data.Bids[0])
	}
	if len(orderbookMsg.Data.Asks) > 0 {
		or.Tri.SetOrder(ASK, ts, orderbookMsg.Data.Symbol, orderbookMsg.Data.Asks[0])
	}

	or.calculateTriangularArbitrage(symbol)

	// Cooldown interval
	time.Sleep(200 * time.Millisecond)
}

func (or *OrderbookRunner) calculateTriangularArbitrage(symbol string) {
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
		var result, secondTrade decimal.Decimal
		capital := decimal.NewFromInt(1000)
		firstTrade := capital.Div(combination.SymbolOrders[0].Bid.Price).Mul(or.NetPercent)
		if combination.BaseQuote {
			secondTrade = firstTrade.Mul(combination.SymbolOrders[1].Bid.Price).Mul(or.NetPercent)
		} else {
			secondTrade = firstTrade.Div(combination.SymbolOrders[1].Bid.Price).Mul(or.NetPercent)
		}
		thirdTrade := secondTrade.Mul(combination.SymbolOrders[2].Bid.Price).Mul(or.NetPercent)
		result = thirdTrade.Truncate(4)
		if result.GreaterThanOrEqual(capital) {
			msg := fmt.Sprintf(
				"%s %s %s->%s   %s (bid: %s) -> %s (ask: %s) -> %s (ask: %s)",
				time.Now().Format("2006-01-02 15:04:05"),
				symbol,
				capital.String(),
				result.String(),
				combination.SymbolOrders[0].Symbol,
				combination.SymbolOrders[0].Bid.Price.String(),
				combination.SymbolOrders[1].Symbol,
				combination.SymbolOrders[1].Bid.Price.String(),
				combination.SymbolOrders[2].Symbol,
				combination.SymbolOrders[2].Bid.Price.String(),
			)
			// Skip it if it's less than limit interval
			if time.Since(or.LastTimeOfTriArbFound) <= time.Duration(TRI_ARB_FOUND_LIMIT_INTERVAL_MS)*time.Millisecond {
				return
			}
			or.LastTimeOfTriArbFound = time.Now()
			or.Messenger.sendToWatch(msg)
		}
	}
}
