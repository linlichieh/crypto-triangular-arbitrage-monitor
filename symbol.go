package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

const (
	BID = "bid"
	ASK = "ask"
)

var klines []string

var klinesMap = map[string]string{
	"BTCUSDT": "orderbook.1.BTCUSDT",
	"ETHUSDT": "orderbook.1.ETHUSDT",
	"ETHBTC":  "orderbook.1.ETHBTC",
	"BTCUSDC": "orderbook.1.BTCUSDC",
	"ETHUSDC": "orderbook.1.ETHUSDC",
	"SOLUSDT": "orderbook.1.SOLUSDT",
	"SOLBTC":  "orderbook.1.SOLBTC",
}

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
	BaseQuote    bool `json:"baseQuote"` // e.g. for "ETHBTC", true will be ETH->BTC, false will be BTC->ETH
	SymbolOrders []*SymbolOrder
	Symbols      []string `json:"symbols"`
}

// symbol -> potential combination
type Tri struct {
	SymbolOrdersMap       map[string]*SymbolOrder // to store bid and ask price for each symbol
	SymbolCombinationsMap map[string][]*Combination
	config                *viper.Viper
}

func initTri() *Tri {
	tri := &Tri{
		SymbolOrdersMap:       make(map[string]*SymbolOrder),
		SymbolCombinationsMap: make(map[string][]*Combination),
	}

	tri.loadSymbolCombinations()
	for _, combinations := range tri.SymbolCombinationsMap {
		for _, combination := range combinations {
			for _, symbol := range combination.Symbols {
				if tri.SymbolOrdersMap[symbol] == nil {
					tri.SymbolOrdersMap[symbol] = &SymbolOrder{Symbol: symbol}
				}
				combination.SymbolOrders = append(combination.SymbolOrders, tri.SymbolOrdersMap[symbol])
			}
		}
	}

	// For ws connect
	tri.setKlines()

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

func (tri *Tri) printAllCombinations() {
	fmt.Println("\nCombinations:")
	for baseSymbol, combinations := range tri.SymbolCombinationsMap {
		fmt.Printf("%s\n", baseSymbol)
		for _, combination := range combinations {
			for _, order := range combination.SymbolOrders {
				fmt.Printf("  %s", order.Symbol)
			}
			fmt.Println()
		}
	}
}

func (tri *Tri) printSymbol(sym string) {
	fmt.Printf("[%s] %s, Bid: %s, Ask: %s\n", tri.SymbolOrdersMap[sym].Ts.Format("2006-01-02 15:04:05"), sym, tri.SymbolOrdersMap[sym].Bid, tri.SymbolOrdersMap[sym].Ask)
}

func (tri *Tri) loadSymbolCombinations() {
	data, err := os.ReadFile("symbol_combinations.json")
	if err != nil {
		log.Fatalf("Error reading JSON file: %v", err)
	}
	err = json.Unmarshal(data, &tri.SymbolCombinationsMap)
	if err != nil {
		log.Fatalf("Error unmarshaling JSON: %v", err)
	}
}

func (tri *Tri) setKlines() {
	for baseSymbol, _ := range tri.SymbolCombinationsMap {
		if klinesMap[baseSymbol] == "" {
			log.Fatalf("klinesMap misses %s", baseSymbol)
		}
		klines = append(klines, klinesMap[baseSymbol])
	}
}
