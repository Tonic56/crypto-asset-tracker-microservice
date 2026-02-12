package service

import (
	"errors"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/internal/api/repository"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/internal/models"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/lib/errs"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/lib/hashcrypto"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserService interface {
	RegisterUser(name string, password string) (uuid.UUID, error)
	LoginUser(name string, password string) (*models.User, error)
	DeleteUserByID(userID uuid.UUID) error
}

type userService struct {
	userRepo repository.UsersDB
}

func NewUserService(userRepo repository.UsersDB) UserService {
	return &userService{
		userRepo: userRepo,
	}
}

func (s *userService) RegisterUser(name string, password string) (uuid.UUID, error) {
	_, err := s.userRepo.GetUserByName(name)

	if err == nil {
		return uuid.Nil, errs.ErrUserExists
	}
	if !errors.Is(err, errs.ErrRecordingWNF) {
		return uuid.Nil, err
	}

	hashedPassword, err := hashcrypto.HashPwd([]byte(password))
	if err != nil {
		return uuid.Nil, err
	}

	user := &models.User{
		Name:     name,
		Password: string(hashedPassword),
	}
	err = s.userRepo.CreateUser(user)
	if err != nil {
		return uuid.Nil, err
	}

	return user.ID, nil
}

func (s *userService) LoginUser(name string, password string) (*models.User, error) {
	user, err := s.userRepo.GetUserByName(name)
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, errs.ErrRecordingWNF
	}
	return user, nil
}

func (s *userService) DeleteUserByID(userID uuid.UUID) error {
	return s.userRepo.DeleteUserByID(userID)
}
