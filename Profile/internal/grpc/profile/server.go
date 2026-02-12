package profile

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/service"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/lib/errs"
	grpc_profile "github.com/Tonic56/proto-crypto-asset-tracker/proto/gen/go/profile"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	grpc_profile.UnimplementedProfileServer
	usersService service.UsersService
	coinsService service.CoinsService
	log          *slog.Logger
}

func NewServer(usersService service.UsersService, coinsService service.CoinsService, log *slog.Logger) *server {
	return &server{
		usersService: usersService,
		coinsService: coinsService,
		log:          log,
	}
}

func (s *server) GetUserProfile(ctx context.Context, req *grpc_profile.GetUserProfileRequest) (*grpc_profile.GetUserProfileResponse, error) {
	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	user, err := s.usersService.GetUserProfile(ctx, userID)
	if err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "failed to get user profile")
	}

	cryptoCoins := make([]*grpc_profile.Coin, 0, len(user.Coins))
	for _, coin := range user.Coins {
		cryptoCoins = append(cryptoCoins, &grpc_profile.Coin{
			Symbol:   coin.Symbol,
			Quantity: coin.Quantity.String(),
		})
	}

	return &grpc_profile.GetUserProfileResponse{
		UserId: user.ID.String(),
		Name:   user.Name,
		Coins:  cryptoCoins,
	}, nil
}

func (s *server) UpdateCoinQuantity(ctx context.Context, req *grpc_profile.UpdateCoinQuantityRequest) (*grpc_profile.UpdateCoinQuantityResponse, error) {
	userID, err := uuid.Parse(req.GetUserId())

	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	symbol := req.GetSymbol()
	if symbol == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid symbol")
	}

	quantityDecimal, err := decimal.NewFromString(req.GetQuantity())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid quantity format: %v", err))
	}

	if quantityDecimal.IsZero() {
		return nil, status.Error(codes.InvalidArgument, "quantity cannot be zero")
	}

	_, err = s.coinsService.UpdateCoinQuantity(ctx, userID, symbol, quantityDecimal)
	
	if err != nil {
		if errors.Is(err, errs.ErrInsufficientFunds) {
			return nil, status.Error(codes.FailedPrecondition, "insufficient funds")
		}
		
        s.log.Error("failed to update coin quantity", slog.Any("error", err))
        return nil, status.Error(codes.Internal, "failed to process request")
	}

	s.log.Info("Addcoin called", "userID", userID, "symbol", symbol, "quantity", quantityDecimal.String())

	return &grpc_profile.UpdateCoinQuantityResponse{Success: true}, nil
}

func (s *server) DeleteCoin(ctx context.Context, req *grpc_profile.DeleteCoinRequest) (*grpc_profile.DeleteCoinResponse, error) {
	userID, err := uuid.Parse(req.GetUserId()); 
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	symbol := req.GetSymbol() 
	if symbol == "" {
		return nil, status.Error(codes.InvalidArgument, "symbol is required")
	}

	if err := s.coinsService.DeleteCoin(ctx, userID, symbol); err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "coin not found in portfolio")
		}

		s.log.Error("failed to delete coin", slog.Any("error", err))
		return nil, status.Error(codes.Internal, "failed to process request")
	}

	return &grpc_profile.DeleteCoinResponse{Success: true}, nil
}