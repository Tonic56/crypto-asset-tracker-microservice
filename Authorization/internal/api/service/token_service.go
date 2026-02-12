package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/internal/api/repository"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/internal/config"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/internal/models"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/lib/errs"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/lib/hashcrypto"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TokenService interface {
	GenerateTokens(userID uuid.UUID, userName string) (string, string, error)
	RefreshToken(refreshToken string) (newaccessToken string, newRefreshToken string, err error)
	Logout(refreshTokenString string) error
	StoreRefreshToken(session *models.Session) error
	GetSessionByToken(token string) (*models.Session, error)
	DeleteSessionByToken(token string) error
	DeleteExpiredToken() error
	DeleteAllUserSessions(userID uuid.UUID) error
}

type tokenService struct {
	tokenRepo repository.TokenRepository
	userRepo  repository.UsersDB
	db        *gorm.DB
	cfg       config.TokenConfig
}

func NewTokenService(tokenRepo repository.TokenRepository,
	userRepo repository.UsersDB,
	db *gorm.DB,
	cfg config.TokenConfig,
) TokenService {
	return &tokenService{
		tokenRepo: tokenRepo,
		userRepo:  userRepo,
		db:        db,
		cfg:       cfg,
	}
}

func (s *tokenService) GenerateTokens(userID uuid.UUID, userName string) (string, string, error) {
	return s.generateTokenInTx(userID, userName, s.tokenRepo)
}

func (s *tokenService) RefreshToken(currentRefreshToken string) (string, string, error) {
	var newAccessToken, newRefreshToken string
	var err error
	err = s.db.Transaction(func(tx *gorm.DB) error {

		txTokenRepo := repository.NewTokenRepository(tx)
		txUserRepo := repository.NewUserRepository(tx)

		hashedToken := hashcrypto.HashToken(currentRefreshToken)
		session, err := txTokenRepo.GetByRefreshTokenHash(hashedToken)
		if err != nil {
			return errs.ErrInvalidToken
		}

		if time.Now().After(session.ExpiresAt) {
			return errs.ErrInvalidToken
		}

		user, err := txUserRepo.GetUserByID(session.UserID)
		if err != nil {
			return fmt.Errorf("inconsistent state: session not found but user not: %w", err)
		}

		if err := txTokenRepo.DeleteByRefreshTokenHash(hashedToken); err != nil {
			return fmt.Errorf("failed to delete old session: %w", err)
		}

		newAccessToken, newRefreshToken, err = s.generateTokenInTx(user.ID, user.Name, txTokenRepo)
		if err != nil {
			return fmt.Errorf("failed to generate new tokens: %w", err)
		}

		return nil 
	})

	if err != nil {
		return "", "", err
	}

	return newAccessToken, newRefreshToken, nil
}

func (s *tokenService) generateTokenInTx(userID uuid.UUID, userName string, repo repository.TokenRepository) (string, string, error) {
	claims := jwt.MapClaims{
		"sub":  userID.String(),
		"name": userName,
		"exp":  time.Now().Add(s.cfg.AccessToken).Unix(),
		"iat":  time.Now().Unix(),
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedAccessToken, err := accessToken.SignedString([]byte(s.cfg.Secret))
	if err != nil {
		return "", "", fmt.Errorf("failed to sign access token: %w", err)
	}

	refreshToken, err := hashcrypto.GenerateRandomString(32)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	session := &models.Session{
		UserID:       userID,
		RefreshToken: hashcrypto.HashToken(refreshToken),
		ExpiresAt:    time.Now().Add(s.cfg.RefreshToken),
	}

	if err := repo.StoreRefreshToken(session); err != nil {
		return "", "", fmt.Errorf("failed to store refresh token session: %w", err)
	}

	return signedAccessToken, refreshToken, nil
}

func (s *tokenService) Logout(refreshToken string) error {
	hashedToken := hashcrypto.HashToken(refreshToken)

	if err := s.tokenRepo.DeleteByRefreshTokenHash(hashedToken); err != nil && !errors.Is(err, errs.ErrRecordingWND) {
		return err
	}

	return nil
}

func (s *tokenService) StoreRefreshToken(session *models.Session) error {
	return s.tokenRepo.StoreRefreshToken(session)
}

func (s *tokenService) GetSessionByToken(token string) (*models.Session, error) {
	return s.tokenRepo.GetByRefreshTokenHash(token)
}

func (s *tokenService) DeleteSessionByToken(token string) error {
	return s.tokenRepo.DeleteByRefreshTokenHash(token)
}

func (s *tokenService) DeleteExpiredToken() error {
	return s.tokenRepo.DeleteExpiredToken()
}

func (s *tokenService) DeleteAllUserSessions(userID uuid.UUID) error {
	return s.tokenRepo.DeleteAllUserSessions(userID)
}
