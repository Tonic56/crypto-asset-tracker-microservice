
package repository

import (
	"errors"
	"fmt"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/internal/models"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/lib/errs"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UsersDB interface {
	CreateUser(user *models.User) error
	GetUserByName(name string) (*models.User, error)
	GetUserByID(userID uuid.UUID) (*models.User, error)
	DeleteUserByID(userID uuid.UUID) error
}

type usersDB struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UsersDB {
	return &usersDB{
		db: db,
	}
}

func (db *usersDB) CreateUser(user *models.User) error {
	if err := db.db.Create(user).Error; err != nil {
		return fmt.Errorf("%w: %s", errs.ErrDB, err.Error())
	}
	return nil
}

func (db *usersDB) DeleteUserByID(userID uuid.UUID) error {
	result := db.db.Where("id = ?", userID).Delete(&models.User{})
	if result.RowsAffected == 0 {
		return errs.ErrRecordingWND
	}

	if result.Error != nil {
		return fmt.Errorf("%w: %s", errs.ErrDB, result.Error.Error())
	}

	return nil
}

func (db *usersDB) GetUserByName(name string) (*models.User, error) {
	var user models.User

	if err := db.db.Where("name = ?", name).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrRecordingWNF
		}
		return nil, fmt.Errorf("%w: %s", errs.ErrDB, err.Error())
	}

	return &user, nil
}

func (db *usersDB) GetUserByID(userID uuid.UUID) (*models.User, error) {
	var user models.User
	if err := db.db.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrRecordingWNF
		}
		return nil, fmt.Errorf("%w: %s", errs.ErrDB, err.Error())
	}
	return &user, nil
}
