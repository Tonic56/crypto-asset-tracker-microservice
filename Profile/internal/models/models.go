package models

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type User struct {
	ID    uuid.UUID `gorm:"type:uuid;primaryKey;"`
	Name  string    `gorm:"unique;not null"`
	Coins []Coin    `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;"`
}

type Coin struct {
	gorm.Model

	Symbol   string          `gorm:"not null"`
	Quantity decimal.Decimal `gorm:"type:decimal(20,8);not null"`
	UserID   uuid.UUID       `gorm:"not null"`
}
