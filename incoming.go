package main

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type Price []string

type Orderbook struct {
	Symbol   string  `json:"s"`
	Bids     []Price `json:"b"`
	Asks     []Price `json:"a"`
	UpdateId int64   `json:"u"`   // Update ID. Is a sequence. Occasionally, you'll receive "u"=1, which is a snapshot data due to the restart of the service. So please overwrite your local orderbook
	Seq      int64   `json:"seq"` // You can use this field to compare different levels orderbook data, and for the smaller seq, then it means the data is generated earlier.
}

type OrderbookMessage struct {
	Topic string    `json:"topic"`
	Ts    int64     `json:"ts"`   // ms
	Type  string    `json:"type"` // Data type. snapshot,delta
	Data  Orderbook `json:"data"`
}

type Runner struct {
	Fee                  decimal.Decimal // 0.01 = 1%
	NetPercent           decimal.Decimal
	OrderbookMessageChan chan *OrderbookMessage
	Tri                  *Tri
	ignoreIncomingMark   bool
}

func initRunner() *Runner {
	fee := decimal.NewFromFloat(0.001)
	return &Runner{
		Fee:                  fee,
		NetPercent:           decimal.NewFromInt(1).Sub(fee),
		OrderbookMessageChan: make(chan *OrderbookMessage),
	}
}

func (r *Runner) setTri(tri *Tri) {
	r.Tri = tri
}

func (r *Runner) feed() {
	for {
		select {
		case orderbookMsg := <-r.OrderbookMessageChan:
			if r.ignoreIncomingMark {
				continue
			}
			r.ignoreIncomingMark = true
			go r.setOrder(orderbookMsg)
		}
	}
}

func (r *Runner) setOrder(orderbookMsg *OrderbookMessage) {
	defer func() { r.ignoreIncomingMark = false }()
	ts := msToTime(orderbookMsg.Ts)
	if len(orderbookMsg.Data.Bids) > 0 {
		r.Tri.SetOrder(BID, ts, orderbookMsg.Data.Symbol, orderbookMsg.Data.Bids[0])
	}
	if len(orderbookMsg.Data.Asks) > 0 {
		r.Tri.SetOrder(ASK, ts, orderbookMsg.Data.Symbol, orderbookMsg.Data.Asks[0])
	}
	// TODO DEBUG
	// fmt.Printf("%s %s\n", ts.Format("2006-01-02 15:04:05"), orderbookMsg.Data.Symbol)
	r.calculateAllCombinations(ts, orderbookMsg.Data.Symbol)
}

func (m *Runner) calculateAllCombinations(ts time.Time, symbol string) {
	combinations := m.Tri.SymbolCombinationsMap[symbol]
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
			fmt.Println("Warning: at least one of bids or prices is nil, please wait for all prices are set")
			return
		}
		var result, secondTrade decimal.Decimal
		capital := decimal.NewFromInt(1000)
		firstTrade := capital.Div(combination.SymbolOrders[0].Bid.Price).Mul(m.NetPercent)
		if combination.BaseQuote {
			secondTrade = firstTrade.Mul(combination.SymbolOrders[1].Bid.Price).Mul(m.NetPercent)
		} else {
			secondTrade = firstTrade.Div(combination.SymbolOrders[1].Bid.Price).Mul(m.NetPercent)
		}
		thirdTrade := secondTrade.Mul(combination.SymbolOrders[2].Bid.Price).Mul(m.NetPercent)
		result = thirdTrade.Truncate(4)
		fmt.Printf(
			"%s %s %s->%s   %s (bid: %s) -> %s (ask: %s) -> %s (ask: %s)\n",
			ts.Format("2006-01-02 15:04:05"),
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
	}
}
