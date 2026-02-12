package repository_test

import (
	"errors"
	"testing"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/repository"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/models"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/lib/errs"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	if err := db.AutoMigrate(&models.User{}, &models.Coin{}); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	return db
}

func TestCreateUserProfile(t *testing.T) {
	testDB := setupTestDB(t)
	userRepo := repository.NewUsersRepository(testDB)

	t.Run("success_create_user", func(t *testing.T) {
		user := &models.User{
			ID:   uuid.New(),
			Name: "test_user",
		}

		if err := userRepo.CreateUserProfile(user); err != nil {
			t.Errorf("CreateUserProfile failed: unexpected error: %v", err)
		}

		foundUser, err := userRepo.GetUserByID(user.ID)
		if err != nil {
			t.Errorf("GetUserByID failed after create: %v", err)
		}

		if foundUser.Name != "test_user" {
			t.Errorf("Expected user name %s, got %s", "test_user", foundUser.Name)
		}
	})

	t.Run("duplicate_user_creation", func(t *testing.T) {
		user := &models.User{
			ID:   uuid.New(),
			Name: "duplicate_user",
		}

		_ = userRepo.CreateUserProfile(user)

		err := userRepo.CreateUserProfile(user)

		if err == nil {
			t.Fatalf("Expected an error for duplicated user creation, but got nil")
		}

		if !errors.Is(err, errs.ErrAlreadyExists) {
			t.Errorf("Expected ErrAlreadyExists, but got %v", err)

		}
	})
}
