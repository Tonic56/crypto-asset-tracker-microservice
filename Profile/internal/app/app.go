package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/config"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/grpc/profile"
	httphandler "github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/handler/http"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/repository"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/service"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/internal/websocket"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/storage/postgres"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Profile/storage/redis"
	"github.com/Tonic56/proto-crypto-asset-tracker/proto/gen/go/auth"
	grpc_profile "github.com/Tonic56/proto-crypto-asset-tracker/proto/gen/go/profile"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type App struct {
	cfg             *config.Config
	log             *slog.Logger
	grpcServer      *grpc.Server
	httpServer      *http.Server
	storage         *postgres.Storage
	redisSubscriber *redis.Subscriber
	wsManager       *websocket.Manager

	
	ctx    context.Context
	cancel context.CancelFunc
}

func New(log *slog.Logger, cfg *config.Config) *App {
	
	ctx, cancel := context.WithCancel(context.Background())

	storage, err := postgres.New(cfg.Database)
	if err != nil {
		panic(fmt.Errorf("failed to init storage: %w", err))
	}

	redisSubscriber := redis.NewSubscriber(log)

	usersRepo := repository.NewUsersRepository(storage.DB)
	usersService := service.NewUsersService(usersRepo)

	coinsRepo := repository.NewCoinsRepository(storage.DB)
	coinsService := service.NewCoinsService(coinsRepo, storage.DB)

	wsManager := websocket.NewManager(log, redisSubscriber, coinsService)

	grpcHandler := profile.NewServer(usersService, coinsService, log)
	grpcServer := grpc.NewServer()
	grpc_profile.RegisterProfileServer(grpcServer, grpcHandler)

	authConn, err := grpc.NewClient(cfg.GRPC.AuthServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(fmt.Errorf("failed to connect to auth service: %w", err))
	}
	authClient := auth.NewAuthClient(authConn)

	ginEngine := gin.New()
	httpHandler := httphandler.NewHandler(usersService, coinsService, wsManager, log, cfg.Security.JWTSecret, authClient)
	httpHandler.RegisterRoutes(ginEngine)

	httpServer := &http.Server{
		Addr:    net.JoinHostPort("", strconv.FormatUint(uint64(cfg.HTTP.Port), 10)),
		Handler: ginEngine,
	}

	return &App{
		log:             log,
		cfg:             cfg,
		grpcServer:      grpcServer,
		httpServer:      httpServer,
		storage:         storage,
		redisSubscriber: redisSubscriber,
		wsManager:       wsManager,
		ctx:             ctx,
		cancel:          cancel,
	}
}

func (a *App) Run() error {
	errChan := make(chan error, 3) 
	slog.Info("starting application components...")

	
	go func() {
		a.log.Info("websocket manager started")
		a.wsManager.Run(a.ctx)
		a.log.Info("websocket manager stopped")
	}()

	
	go func() {
		if err := a.runGRPC(); err != nil {
			errChan <- fmt.Errorf("gRPC server error: %w", err)
		}
	}()

	
	go func() {
		if err := a.runHTTP(); err != nil {
			errChan <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	
	err := <-errChan
	a.log.Warn("shutting down application due to an error", "error", err)

	a.Stop()
	return err
}

func (a *App) Stop() {
	a.log.Info("stopping application components gracefully...")

	
	a.cancel()

	
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), a.cfg.HTTP.Timeout)
	defer shutdownCancel()

	if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
		a.log.Warn("failed to gracefully shutdown HTTP server", "error", err)
	} else {
		a.log.Info("HTTP server stopped")
	}

	
	a.grpcServer.GracefulStop()
	a.log.Info("gRPC server stopped")

	
	a.redisSubscriber.Close()

	
	if err := a.storage.Stop(); err != nil {
		a.log.Error("failed to stop storage", "error", err)
	} else {
		a.log.Info("database connection closed")
	}
}

func (a *App) runGRPC() error {
	const op = "app.runGRPC"

	grpcAddress := net.JoinHostPort("", strconv.FormatUint(uint64(a.cfg.GRPC.Port), 10))
	listener, err := net.Listen("tcp", grpcAddress)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	a.log.Info("gRPC server is running", "addr", listener.Addr().String())

	if err := a.grpcServer.Serve(listener); err != nil {
		
		if !errors.Is(err, net.ErrClosed) {
			return fmt.Errorf("%s: %w", op, err)
		}
	}
	return nil
}

func (a *App) runHTTP() error {
	const op = "app.runHTTP"

	a.log.Info("HTTP server is running", "addr", a.httpServer.Addr)

	if err := a.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
