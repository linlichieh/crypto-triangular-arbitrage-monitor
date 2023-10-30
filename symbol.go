package main

import (
	"fmt"
	"time"
)

/*
For example, consider the following hypothetical rates:

ETH/BTC: 0.03 (1 ETH is 0.03 BTC)
BTC/USD: 50,000 (1 BTC is 50,000 USD)
ETH/USD: 1,600 (1 ETH is 1,600 USD)
Let's analyze two routes:

Route ETH -> BTC -> USD -> ETH:

1 ETH to 0.03 BTC
0.03 BTC to 1,500 USD
1,500 USD back to 0.9375 ETH (since 1,600 USD gives 1 ETH)
Route ETH -> USD -> BTC -> ETH:

1 ETH to 1,600 USD
1,600 USD to 0.032 BTC (since 50,000 USD gives 1 BTC)
0.032 BTC back to 1.06 ETH (since 0.03 BTC gives 1 ETH)
In this example, starting with 1 ETH, the first route returns 0.9375 ETH, while the second route returns 1.06 ETH, indicating a potential arbitrage opportunity in the second route.
*/

const (
	BID = "bid"
	ASK = "ask"
)

type SymbolPrice struct {
	Ts     time.Time // timestamp
	Symbol string
	Bid    *Price // latest bid
	Ask    *Price // latest ask
}

// symbol -> potential combination
type Tri struct {
	SymbolPriceMap map[string]*SymbolPrice   // to store bid and ask price for each symbol
	PairsMap       map[string][]*SymbolPrice // to store symbol pairs
}

func initTri() *Tri {
	tri := &Tri{
		SymbolPriceMap: make(map[string]*SymbolPrice),
		PairsMap:       make(map[string][]*SymbolPrice),
	}

	combinations := [][]string{
		{"BTCUSDT", "ETHBTC", "ETHUSDT"},
		{"ETHUSDT", "ETHBTC", "BTCUSDT"},
	}
	for _, symbols := range combinations {
		firstSym := symbols[0]
		for _, symbol := range symbols {
			if _, ok := tri.SymbolPriceMap[symbol]; !ok {
				tri.SymbolPriceMap[symbol] = &SymbolPrice{Symbol: symbol}
			}
			tri.PairsMap[firstSym] = append(tri.PairsMap[firstSym], tri.SymbolPriceMap[symbol])
		}
	}
	return tri
}

func (tri *Tri) SetPrice(action string, ts time.Time, sym string, price *Price) {
	tri.SymbolPriceMap[sym].Ts = ts
	switch action {
	case BID:
		tri.SymbolPriceMap[sym].Bid = price
	case ASK:
		tri.SymbolPriceMap[sym].Ask = price
	}
	// TODO DEBUG
	tri.printSymbol(sym)
}

func (tri *Tri) printAll() {
	fmt.Println("\nCombinations:")
	for _, pairs := range tri.PairsMap {
		fmt.Printf("%s -> %s -> %s\n", pairs[0].Symbol, pairs[1].Symbol, pairs[2].Symbol)
	}
	fmt.Println()
}

func (tri *Tri) printSymbol(sym string) {
	fmt.Printf("[%s] %s, Bid: %s, Ask: %s\n", tri.SymbolPriceMap[sym].Ts.Format("2006-01-02 15:04:05"), sym, tri.SymbolPriceMap[sym].Bid, tri.SymbolPriceMap[sym].Ask)
}
