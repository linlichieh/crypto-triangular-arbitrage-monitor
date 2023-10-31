package main

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

const (
	BID = "bid"
	ASK = "ask"
)

type Order struct {
	Price decimal.Decimal
	Size  decimal.Decimal
}

type SymbolOrder struct {
	Symbol string
	Bid    *Order    // latest bid
	Ask    *Order    // latest ask
	Ts     time.Time // timestamp
}

type Combination struct {
	BaseQuote    bool // e.g. for "ETHBTC", true will be ETH->BTC, false will be BTC->ETH
	SymbolOrders []*SymbolOrder
}

// symbol -> potential combination
type Tri struct {
	SymbolOrdersMap       map[string]*SymbolOrder // to store bid and ask price for each symbol
	SymbolCombinationsMap map[string][]*Combination
}

func initTri() *Tri {
	tri := &Tri{
		SymbolOrdersMap:       make(map[string]*SymbolOrder),
		SymbolCombinationsMap: make(map[string][]*Combination),
	}

	type combination struct {
		baseQuote bool
		symbols   []string
	}
	list := []struct {
		symbols      []string
		combinations []combination
	}{
		{
			symbols: []string{"BTCUSDT", "ETHBTC", "ETHUSDT"},
			combinations: []combination{
				{baseQuote: false, symbols: []string{"BTCUSDT", "ETHBTC", "ETHUSDT"}},
				{baseQuote: true, symbols: []string{"ETHUSDT", "ETHBTC", "BTCUSDT"}},
			},
		},
		// TODO FIXME
		// {
		// symbols: []string{"BTCUSDC", "ETHBTC", "ETHUSDC"},
		// combinations: []combination{
		// {baseQuote: false, symbols: []string{"BTCUSDC", "ETHBTC", "ETHUSDC"}},
		// {baseQuote: true, symbols: []string{"ETHUSDC", "ETHBTC", "BTCUSDC"}},
		// },
		// },
		// {
		// symbols: []string{"BTCUSDT", "ETHBTC", "ETHUSDT", "BTCUSDC", "ETHUSDC"},
		// combinations: []combination{
		// {baseQuote: false, symbols: []string{"BTCUSDT", "ETHBTC", "ETHUSDC"}},
		// {baseQuote: true, symbols: []string{"ETHUSDT", "ETHBTC", "BTCUSDC"}},
		// {baseQuote: false, symbols: []string{"BTCUSDC", "ETHBTC", "ETHUSDT"}},
		// {baseQuote: true, symbols: []string{"ETHUSDC", "ETHBTC", "BTCUSDT"}},
		// },
		// },
	}
	for _, item := range list {
		var cs []*Combination
		for _, combination := range item.combinations {
			c := &Combination{BaseQuote: combination.baseQuote}
			for _, symbol := range combination.symbols {
				if tri.SymbolOrdersMap[symbol] == nil {
					tri.SymbolOrdersMap[symbol] = &SymbolOrder{Symbol: symbol}
				}
				c.SymbolOrders = append(c.SymbolOrders, tri.SymbolOrdersMap[symbol])
			}
			cs = append(cs, c)
		}
		for _, symbol := range item.symbols {
			if tri.SymbolCombinationsMap[symbol] == nil {
				tri.SymbolCombinationsMap[symbol] = cs
			} else {
				tri.SymbolCombinationsMap[symbol] = append(tri.SymbolCombinationsMap[symbol], cs...)
			}
		}
	}
	return tri
}

func (tri *Tri) SetOrder(action string, ts time.Time, sym string, price Price) error {
	tri.SymbolOrdersMap[sym].Ts = ts
	p, err := decimal.NewFromString(price[0])
	if err != nil {
		return err
	}
	s, err := decimal.NewFromString(price[1])
	if err != nil {
		return err
	}
	switch action {
	case BID:
		tri.SymbolOrdersMap[sym].Bid = &Order{Price: p, Size: s}
	case ASK:
		tri.SymbolOrdersMap[sym].Ask = &Order{Price: p, Size: s}
	}
	// TODO DEBUG
	// tri.printSymbol(sym)
	return nil
}

func (tri *Tri) printAll() {
	fmt.Println("\nCombinations:")
	for symbol, combinations := range tri.SymbolCombinationsMap {
		fmt.Printf("%s:\n", symbol)
		for _, combination := range combinations {
			fmt.Printf("  [%s -> %s -> %s]\n", combination.SymbolOrders[0].Symbol, combination.SymbolOrders[1].Symbol, combination.SymbolOrders[2].Symbol)
		}
	}
	fmt.Println()
}

func (tri *Tri) printSymbol(sym string) {
	fmt.Printf("[%s] %s, Bid: %s, Ask: %s\n", tri.SymbolOrdersMap[sym].Ts.Format("2006-01-02 15:04:05"), sym, tri.SymbolOrdersMap[sym].Bid, tri.SymbolOrdersMap[sym].Ask)
}
