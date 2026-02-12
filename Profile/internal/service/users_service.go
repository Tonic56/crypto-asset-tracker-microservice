package service

import (
	"context"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/repository"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/models"
	"github.com/google/uuid"
)

type UsersService interface {
	CreateUserProfile(ctx context.Context, userID uuid.UUID, name string) (*models.User, error)
	GetUserProfile(ctx context.Context, userID uuid.UUID) (*models.User, error)
	DeleteUserProfile(ctx context.Context, userID uuid.UUID) error
}


type usersService struct {
	repo repository.UsersRepository
}

func NewUsersService(repo repository.UsersRepository) UsersService {
	return &usersService{
		repo: repo,
	}
}

func (s *usersService) CreateUserProfile(_ context.Context, userID uuid.UUID, name string) (*models.User, error) {
	user := models.User{
		ID: userID,
		Name: name,
	}

	if err := s.repo.CreateUserProfile(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *usersService) GetUserProfile(_ context.Context, userID uuid.UUID) (*models.User, error) {
	return s.repo.GetUserByID(userID)
}

func (s *usersService) DeleteUserProfile(_ context.Context, userID uuid.UUID) error {
	return s.repo.DeleteUserByID(userID)
}