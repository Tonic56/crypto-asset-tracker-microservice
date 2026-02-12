package auth

import (
	"context"
	"errors"

	"github.com/Tonic56/proto-crypto-asset-tracker/proto/gen/go/auth"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/internal/api/service"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/lib/errs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	auth.UnimplementedAuthServer
	userService  service.UserService
	tokenService service.TokenService
}

func New(userService service.UserService, tokenService service.TokenService) *Server {
	return &Server{
		userService:  userService,
		tokenService: tokenService,
	}
}

func (s *Server) Register(ctx context.Context, req *auth.RegisterRequest) (*auth.RegisterResponse, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}

	userID, err := s.userService.RegisterUser(req.GetName(), req.GetPassword())
	if err != nil {
		if errors.Is(err, errs.ErrUserExists) {
			return nil, status.Error(codes.AlreadyExists, "user with this name already exists")
		}

		return nil, status.Error(codes.Internal, "failed to register user")
	}
	return &auth.RegisterResponse{
		UserId: userID.String(),
	}, nil
}

func (s *Server) Login(ctx context.Context, req *auth.LoginRequest) (*auth.LoginResponse, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}

	user, err := s.userService.LoginUser(req.GetName(), req.GetPassword())
	if err != nil {
		if errors.Is(err, errs.ErrRecordingWNF) {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}
		return nil, status.Error(codes.Internal, "login failed")
	}

	accsessToken, refreshToken, err := s.tokenService.GenerateTokens(user.ID, user.Name)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate tokens")
	}

	return &auth.LoginResponse{
		AccessToken:  accsessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *Server) RefreshToken(ctx context.Context, req *auth.RefreshTokenRequest) (*auth.RefreshTokenResponse, error) {
	if req.GetRefreshToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh token is required")
	}

	newAccessToken, newRefreshToken, err := s.tokenService.RefreshToken(req.GetRefreshToken()) 
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, errs.ErrInvalidToken.Error())
	}

	return &auth.RefreshTokenResponse{
		AccessToken: newAccessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (s *Server) Logout(ctx context.Context, req *auth.LogoutRequest) (*auth.LogoutResponse, error) {
	if req.GetRefreshToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh token is required")
	}

	err := s.tokenService.Logout(req.GetRefreshToken())
	
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to logout")
	}

	return &auth.LogoutResponse{Success: true}, nil
}