package repository

import (
	"errors"
	"strings"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/models"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/lib/errs"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CoinsRepository interface {
	AddCoin(coin *models.Coin) error
	GetCoin(userID uuid.UUID, symbol string) (*models.Coin, error)
	UpdateCoin(coin *models.Coin) error
	DeleteCoin(userID uuid.UUID, symbol string) error
}

type coinsRepository struct {
	db *gorm.DB
}

func NewCoinsRepository(db *gorm.DB) CoinsRepository {
	return &coinsRepository{
		db: db,
	}
}

func (db *coinsRepository) AddCoin(coin *models.Coin) error {
	if err := db.db.Create(coin).Error; err != nil {
		errorString := err.Error()
		if strings.Contains(errorString, "UNIQUE") || strings.Contains(errorString, "duplicate") {
			return errs.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (db *coinsRepository) GetCoin(userID uuid.UUID, symbol string) (*models.Coin, error) {
	var coin models.Coin

	if err := db.db.Where("user_id = ? AND symbol = ?", userID, symbol).First(&coin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}

	return &coin, nil
}

func (db *coinsRepository) UpdateCoin(coin *models.Coin) error {
	if err := db.db.Save(coin).Error; err != nil {
		return err
	}
	return nil
}

func (db *coinsRepository) DeleteCoin(userID uuid.UUID, symbol string) error {
	result := db.db.Where("user_id = ? AND symbol = ?", userID, symbol).Delete(&models.Coin{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errs.ErrNotFound
	}

	return nil
}