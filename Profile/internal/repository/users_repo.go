package repository

import (
	"errors"
	"strings"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/models"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/lib/errs"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UsersRepository interface {
	CreateUserProfile(user *models.User) error
	GetUserByID(userID uuid.UUID) (*models.User, error)
	DeleteUserByID(userID uuid.UUID) error
}

type usersRepository struct {
	db *gorm.DB
}

func NewUsersRepository(db *gorm.DB) UsersRepository {
	return &usersRepository{db: db}
}

func (db *usersRepository) CreateUserProfile(user *models.User) error {
	if err := db.db.Create(user).Error; err != nil {
		errorString := err.Error()
		if strings.Contains(errorString, "UNIQUE constraint failed") || strings.Contains(errorString, "duplicate key valuе violates unique constraint") {
			return errs.ErrAlreadyExists
		}

		return errs.ErrInternal
	}

	return nil
}

func (db *usersRepository) GetUserByID(userID uuid.UUID) (*models.User, error) {
	var user models.User
	if err := db.db.Preload("Coins").First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}

		return nil, err
	}
	return &user, nil
}

func (db *usersRepository) DeleteUserByID(userID uuid.UUID) error {

	result := db.db.Delete(&models.User{}, userID)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errs.ErrNotFound
	}

	return nil
}
