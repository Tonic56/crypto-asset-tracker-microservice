package models

import "github.com/shopspring/decimal"

type KafkaMsg struct {
	MessageID     string          `json:"message_id"`
	EventType     string          `json:"e"`
	EventTime     int64           `json:"E"`
	RecvTime      int64           `json:"receive_time"`
	Symbol        string          `json:"s"`
	ClosePrice    decimal.Decimal `json:"c"`
	OpenPrice     decimal.Decimal `json:"o"`
	HighPrice     decimal.Decimal `json:"h"`
	LowPrice      decimal.Decimal `json:"l"`
	ChangePrice   decimal.Decimal `json:"change_price"`
	ChangePercent decimal.Decimal `json:"change_percent"`
}
