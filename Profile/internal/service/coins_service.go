package service

import (
	"context"
	"fmt"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/models"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/repository"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/lib/errs"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type CoinsService interface {
	UpdateCoinQuantity(ctx context.Context, userID uuid.UUID, symbol string, quantityChange decimal.Decimal) (*models.Coin, error)
	DeleteCoin(ctx context.Context, userID uuid.UUID, symbol string) error
}

type coinsService struct {
	coinsRepo repository.CoinsRepository
	db *gorm.DB
}

func NewCoinsService(coinsRepo repository.CoinsRepository, db *gorm.DB) CoinsService {
	return &coinsService{
		coinsRepo: coinsRepo,
		db: db,
	}
}

func (s *coinsService) UpdateCoinQuantity(ctx context.Context, userID uuid.UUID, symbol string, quantityChange decimal.Decimal) (*models.Coin, error) {
	var resultingCoin *models.Coin

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := repository.NewCoinsRepository(tx)

		existingCoin, err := txRepo.GetCoin(userID, symbol)

		if err != nil {
			if quantityChange.IsNegative() {
				return errs.ErrInsufficientFunds
			}

			if err == errs.ErrNotFound {
				newCoin := &models.Coin{
					Symbol: symbol,
					Quantity: quantityChange,
					UserID: userID,
				}
				if err := txRepo.AddCoin(newCoin); err != nil {
					return err
				}
				resultingCoin = newCoin
				return nil
			}

			return err
		}
		newQuantity := existingCoin.Quantity.Add(quantityChange)

		if newQuantity.IsNegative() {
			return errs.ErrInsufficientFunds
		}

		existingCoin.Quantity = newQuantity

		if err := txRepo.UpdateCoin(existingCoin); err != nil {
			return err
		}
		
		resultingCoin = existingCoin
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to upsert coin: %w", err)
	}

	return resultingCoin, nil
}

func (s *coinsService) DeleteCoin(_ context.Context, userID uuid.UUID, symbol string) error {
	return s.coinsRepo.DeleteCoin(userID, symbol)
}
