package main

// TODO
// type Kline struct {
// Start      int64  `json:"start"`
// Timestamp  int64  `json:"timestamp"`
// Symbol     string `json:"symbol"`
// Interval   string `json:"interval"`
// OpenPrice  string `json:"open"`
// ClosePrice string `json:"close"`
// HighPrice  string `json:"high"`
// LowPrice   string `json:"low"`
// Volume     string `json:"volume"`
// Turnover   string `json:"turnover"`
// }

// TODO
// type KlineMessage struct {
// Topic string  `json:"topic"`
// Type  string  `json:"type"`
// Data  []Kline `json:"data"`
// }

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

type Messenger struct {
	OrderbookMessageChan chan *OrderbookMessage
	Tri                  *Tri
}

func initMessenger() *Messenger {
	return &Messenger{
		OrderbookMessageChan: make(chan *OrderbookMessage),
	}
}

func (m *Messenger) setTri(tri *Tri) {
	m.Tri = tri
}

func (m *Messenger) process() {
	for {
		select {
		case orderbookMsg := <-m.OrderbookMessageChan:
			t := msToTime(orderbookMsg.Ts)
			if len(orderbookMsg.Data.Bids) > 0 {
				m.Tri.SetPrice(BID, t, orderbookMsg.Data.Symbol, &orderbookMsg.Data.Bids[0])
			}
			if len(orderbookMsg.Data.Asks) > 0 {
				m.Tri.SetPrice(ASK, t, orderbookMsg.Data.Symbol, &orderbookMsg.Data.Asks[0])
			}
		}

		// TODO
		// var klineMsg KlineMessage
		// if err := json.Unmarshal(message, &klineMsg); err != nil {
		// log.Printf("Error parsing JSON: %v", err)
		// continue
		// }
		// if len(klineMsg.Data) > 0 {
		// t := time.Unix(0, klineMsg.Data[0].Timestamp*int64(time.Millisecond))
		// fmt.Printf("[%s] %s %s\n", t.Format("2006-01-02 15:04:05"), klineMsg.Topic, klineMsg.Data[0].ClosePrice)
		// }
	}
}
