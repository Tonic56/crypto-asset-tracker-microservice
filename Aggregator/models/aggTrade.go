package models

import (
	"log/slog"
	"strconv"
)

type AggTrade struct {
	EventType        string `json:"e"` 
	EventTime        int64  `json:"E"` 
	Symbol           string `json:"s"` 
	AggregateTradeID int64  `json:"a"` 
	Price            string `json:"p"` 
	Quantity         string `json:"q"` 
	FirstTradeID     int64  `json:"f"` 
	LastTradeID      int64  `json:"l"` 
	TradeTime        int64  `json:"T"` 
	IsBuyer          bool   `json:"m"` 
	Ignore           bool   `json:"M"` 
}

func (at *AggTrade) PriceFloat() float64 {
	pf, err := strconv.ParseFloat(at.Price, 64)
	if err != nil {
		slog.Error("Could not parse Price into float64", "error", err)
		return -1
	}
	return pf
}
