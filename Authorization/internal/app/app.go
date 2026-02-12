package app

import (
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	grpcapp "github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/internal/grpc/auth"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/internal/api/repository"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/internal/api/service"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/internal/config"
	storage "github.com/Tonic56/crypto-asset-tracker-microservice/Authorization/storage/postgres"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	auth "github.com/Tonic56/proto-crypto-asset-tracker/proto/gen/go/auth"
)

type App struct {
	log          *slog.Logger
	cfg          *config.Config
	gRPCServer   *grpc.Server
	storage      *storage.Storage
	tokenService service.TokenService
}

func New(log *slog.Logger, cfg *config.Config) *App {
	st, err := storage.New(cfg.Database)
	if err != nil {
		panic(fmt.Errorf("failed to init storage: %w", err))
	}

	userRepo := repository.NewUserRepository(st.DB)
	tokenRepo := repository.NewTokenRepository(st.DB)

	userService := service.NewUserService(userRepo)
	tokenService := service.NewTokenService(tokenRepo, userRepo, st.DB, cfg.Token)

	grpcServer := grpc.NewServer()

	auth.RegisterAuthServer(grpcServer, grpcapp.New(userService, tokenService))

	if cfg.GRPC.EnableReflection {
		reflection.Register(grpcServer)
		log.Info("gRPC reflection enabled")
	}
	return &App{
		log:        log,
		gRPCServer: grpcServer,
		storage:    st,
		cfg:        cfg,
		tokenService: tokenService,
	}
}

func (a *App) Run() error {
	const op = "app.Run"

	go a.runTokenCleanup()

	grpcAddress := net.JoinHostPort("", strconv.FormatUint(uint64(a.cfg.GRPC.Port), 10))
	listener, err := net.Listen("tcp", grpcAddress)
	if err != nil {
		return fmt.Errorf("%s: gRPC server failed to serve: %w", op, err)
	}

	a.log.Info("gRPC server started", slog.String("address", grpcAddress))

	if err := a.gRPCServer.Serve(listener); err != nil {
		return fmt.Errorf("%s: gRPC server failed to serve: %w", op, err)
	}

	return nil
}

func (a *App) runTokenCleanup() {
	ticker := time.NewTicker(a.cfg.Token.TokenCleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		a.log.Info("running expired tokens cleanup...")
		if err := a.tokenService.DeleteExpiredToken(); err != nil {
			a.log.Error("failed to cleanup expired tokens", slog.Any("error", err))
		} else {
			a.log.Info("expired tokens cleanup finished successfully")
		}
	}
}

func (a *App) Stop() {
	a.log.Info("stopping gRPC server...")
	a.gRPCServer.GracefulStop()
	a.log.Info("gRPC server stoped...")

	a.log.Info("Stoping storage...")
	if err := a.storage.Stop(); err != nil {
		a.log.Error("failed to stop storage", slog.Any("error", err))
	}
}
