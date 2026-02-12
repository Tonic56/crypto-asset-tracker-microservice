
package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/internal/models"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/lib/errs"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TokenRepository interface {
	StoreRefreshToken(session *models.Session) error
	GetByRefreshTokenHash(tokenHash string) (*models.Session, error)
	DeleteByRefreshTokenHash(tokenHash string) error
	DeleteExpiredToken() error
	DeleteAllUserSessions(userID uuid.UUID) error
}

type tokenRepository struct {
	db *gorm.DB
}

func NewTokenRepository(db *gorm.DB) TokenRepository {
	return &tokenRepository{
		db: db,
	}
}

func (db *tokenRepository) StoreRefreshToken(session *models.Session) error {

	result := db.db.Create(session)

	if result.Error != nil {
		return fmt.Errorf("%w: %s", errs.ErrDB, result.Error.Error())
	}

	return nil
}

func (db *tokenRepository) GetByRefreshTokenHash(tokenHash string) (*models.Session, error) {
	var session models.Session

	if err := db.db.Where("refresh_token = ?", tokenHash).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrRecordingWNF
		}
		return nil, fmt.Errorf("%w: %s", errs.ErrDB, err.Error())
	}

	return &session, nil
}

func (db *tokenRepository) DeleteByRefreshTokenHash(tokenHash string) error {
	result := db.db.Where("refresh_token = ?", tokenHash).Delete(&models.Session{})

	if err := result.Error; err != nil {
		return fmt.Errorf("%w: %s", errs.ErrDB, err.Error())
	}

	if result.RowsAffected == 0 {
		return errs.ErrRecordingWND
	}

	return nil
}

func (db *tokenRepository) DeleteExpiredToken() error {
	result := db.db.Where("expires_at < ?", time.Now()).Delete(&models.Session{})
	if result.Error != nil {
		return fmt.Errorf("%w: %s", errs.ErrDB, result.Error.Error())
	}
	return nil
}

func (db *tokenRepository) DeleteAllUserSessions(userID uuid.UUID) error {
	result := db.db.Where("user_id = ?", userID).Delete(&models.Session{})
	if result.Error != nil {
		return fmt.Errorf("%w: %s", errs.ErrDB, result.Error.Error())
	}
	return nil
}
