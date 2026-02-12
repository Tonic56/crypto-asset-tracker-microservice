package postgres

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/config"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Storage struct {
	DB *gorm.DB
}

func New(cfg config.DBConfig) (*Storage, error) {
	const op = "storage/postgres"

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable",
		cfg.Host, cfg.User, cfg.Password, cfg.DBName, cfg.Port)

	var db *gorm.DB
	var err error

	for i := 0; i < 5; i++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err == nil {
			break
		}
		slog.Warn("failed to connect to postgres, retrying...", "attempt", i+1, "error", err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("%s:  failed to connect after multiple retries: %w", op, err)
	}

	slog.Info("Successfully connected to PostgreSQL.")

	if err := db.AutoMigrate(&models.User{}, &models.Coin{}); err != nil {
		return nil, fmt.Errorf("%s: failed to auto-migrate database: %w", op, err)
	}
	slog.Info("Database auto-migration completed.")

	return &Storage{DB: db}, nil
}

func (s *Storage) Stop() error {
	sqlDb, err := s.DB.DB()
	if err != nil {
		return err
	}

	return sqlDb.Close()
}
