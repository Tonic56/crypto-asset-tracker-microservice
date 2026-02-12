package models

import (
	"fmt"

	"github.com/shopspring/decimal"
)

type SecondStat struct {
	Symbol string  `json:"s"` 
	Price  float64 `json:"p"` 
}

type DailyStat struct {
	EventType  string  `json:"e"`
	EventTime  int64   `json:"E"`
	RecvTime   int64   `json:"receive_rime"`
	Symbol     string  `json:"s"`
	ClosePrice float64 `json:"c"`
	OpenPrice  float64 `json:"o"`
	HighPrice  float64 `json:"h"`
	LowPrice   float64 `json:"l"`
}

func (ds *DailyStat) ChangeInPrice() decimal.Decimal {
	op := decimal.NewFromFloat(ds.OpenPrice)
	cp := decimal.NewFromFloat(ds.ClosePrice)

	return cp.Sub(op)
}

func (ds *DailyStat) ChangeInPercent() decimal.Decimal {
	op := decimal.NewFromFloat(ds.OpenPrice)
	cp := decimal.NewFromFloat(ds.ClosePrice)

	return cp.Sub(op).Div(cp).Mul(decimal.NewFromFloat(100))
}

func (ds *DailyStat) ShowStatistic() string {
	difference := ds.ChangeInPercent().InexactFloat64()

	if difference >= 0 {
		return fmt.Sprintf("📈 +%.2f%%", difference)
	}
	return fmt.Sprintf("📉 %.2f%%", difference)
}
