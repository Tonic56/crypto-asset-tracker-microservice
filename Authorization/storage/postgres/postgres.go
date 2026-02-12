package postgres

import (
	"fmt"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/internal/config"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Storage struct {
	DB *gorm.DB
}

func New(cfg config.DBConfig) (*Storage, error) {
	const op = "storage.postgres.New"

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable",
		cfg.Host, cfg.User, cfg.Password, cfg.DBName, cfg.Port,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	if err := db.AutoMigrate(&models.User{}, &models.Session{}); err != nil {
		return nil, fmt.Errorf("%s: failed to migrate database: %w", op, err)
	}

	return &Storage{DB: db}, nil
}

func (s *Storage) Stop() error {
	db, err := s.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying db connection: %w", err)
	}
	return db.Close()
}
