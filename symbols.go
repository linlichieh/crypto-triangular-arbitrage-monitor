package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/shopspring/decimal"
)

const (
	BID = "bid"
	ASK = "ask"
)

// symbol -> potential combination
type Tri struct {
	SymbolOrdersMap       map[string]*SymbolOrder // to store bid and ask price for each symbol
	SymbolCombinationsMap map[string][]*Combination
	Messenger             *Messenger
	OrderbookTopics       map[string]string
}

// Combination is a paris of 3 symbols
type Combination struct {
	BaseQuote    bool
	SymbolOrders []*SymbolOrder
}

// orderbook
type SymbolOrder struct {
	Symbol string
	Bid    *Order // latest bid
	Ask    *Order // latest ask
	Seq    int64
}

type Order struct {
	Price decimal.Decimal
	Size  decimal.Decimal
}

func initTri() *Tri {
	tri := &Tri{
		SymbolOrdersMap:       make(map[string]*SymbolOrder),
		SymbolCombinationsMap: make(map[string][]*Combination),
		OrderbookTopics:       make(map[string]string),
	}
	tri.buildSymbolCombinations()
	return tri
}

func (tri *Tri) setMessenger(messenger *Messenger) {
	tri.Messenger = messenger
}

func (tri *Tri) buildSymbolCombinations() {
	data := tri.readSymbolsJson()

	// Load orderbook topics
	for symbol, topic := range data["topics"].(map[string]any) {
		tri.OrderbookTopics[symbol] = topic.(string)
	}

	// Load symbols combinations
	for _, item := range data["list"].([]any) {
		// symbols
		for _, symbol := range item.(map[string]any)["symbols"].([]any) {
			if tri.SymbolOrdersMap[symbol.(string)] == nil {
				tri.SymbolOrdersMap[symbol.(string)] = &SymbolOrder{Symbol: symbol.(string)}
			}
		}

		// combinations
		var cs []*Combination
		for _, combination := range item.(map[string]any)["combinations"].([]any) {
			var c Combination
			c.BaseQuote = combination.(map[string]any)["base_quote"].(bool)
			for _, symbol := range combination.(map[string]any)["symbols"].([]any) {
				c.SymbolOrders = append(c.SymbolOrders, tri.SymbolOrdersMap[symbol.(string)])
			}
			cs = append(cs, &c)
		}

		// Build relationships between symbols and combinations
		for _, symbol := range item.(map[string]any)["symbols"].([]any) {
			tri.SymbolCombinationsMap[symbol.(string)] = append(tri.SymbolCombinationsMap[symbol.(string)], cs...)
		}
	}
}

func (tri *Tri) readSymbolsJson() map[string]interface{} {
	body, err := os.ReadFile("symbol_combinations.json")
	if err != nil {
		log.Fatalf("Error reading JSON file: %v", err)
	}
	data := make(map[string]any)
	err = json.Unmarshal(body, &data)
	if err != nil {
		log.Fatalf("Error unmarshaling JSON: %v", err)
	}
	return data
}

func (tri *Tri) SetOrder(action string, sym string, price Price, seq int64) error {
	tri.SymbolOrdersMap[sym].Seq = seq
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
	return nil
}

func (tri *Tri) printAllSymbols() {
	var symbols []string
	for symbol := range tri.SymbolOrdersMap {
		symbols = append(symbols, symbol)
	}
	tri.Messenger.sendToChannel(tri.Messenger.Channel.SystemLogs, fmt.Sprintf("Symbols: %v", symbols))
}

func (tri *Tri) printAllCombinations() {
	msg := "Symbol combinations:"
	for baseSymbol, combinations := range tri.SymbolCombinationsMap {
		msg += fmt.Sprintf("\n  %s", baseSymbol)
		for _, combination := range combinations {
			msg += "\n   - ["
			for _, order := range combination.SymbolOrders {
				msg += fmt.Sprintf(" %s ", order.Symbol)
			}
			msg += "]"
		}
	}
	log.Println(msg)
}
