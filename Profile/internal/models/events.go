package models

import "github.com/shopspring/decimal"

type PriceUpdate struct {
	Symbol string  `json:"s"`
	Price  float64 `json:"p"`
}

type CoinView struct {
	Symbol   string          `json:"symbol"`
	Quantity decimal.Decimal `json:"quantity"`
	Price    decimal.Decimal `json:"price"`
	Total    decimal.Decimal `json:"total"`
}

type PortfolioView struct {
	UserID     string     `json:"userID"`
	UserName   string     `json:"userName"`
	TotalValue decimal.Decimal `json:"totalValue"`
	Coins      []CoinView `json:"coins"`
}