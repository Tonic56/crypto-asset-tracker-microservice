package svr

import (
	"context"
	"log/slog"
	"net"
	"sync"

	socket "github.com/Tonic56/proto-crypto-asset-tracker/proto/gen/go/socket"
	"github.com/Tonic56/crypto-asset-tracker-microservice/Socket/lib/getenv"
	"google.golang.org/grpc"
)

type ConnectionManager interface {
	GetOrCreateConnection(symbol string) <-chan []byte
	GetMiniTickerConnection() <-chan []byte
}

type server struct {
	socket.UnimplementedSocketServiceServer
	connManager ConnectionManager
	mainCtx     context.Context
}

func register(gRPC *grpc.Server, connManager ConnectionManager, ctx context.Context) {
	socket.RegisterSocketServiceServer(gRPC, &server{
		connManager: connManager,
		mainCtx:     ctx,
	})
}

func (s *server) ReceiveRawMiniTicker(
	req *socket.RawMiniTickerRequest,
	stream socket.SocketService_ReceiveRawMiniTickerServer,
) error {
	slog.Info("Client connected to ReceiveRawMiniTicker stream")

	outputChan := s.connManager.GetMiniTickerConnection()

	for {
		select {
		case <-stream.Context().Done():
			slog.Warn("Got Interruption signal from streaming server from stream context")
			return stream.Context().Err()
		case <-s.mainCtx.Done():
			slog.Info("Got Interruption signal from streaming server from main context")
			return stream.Context().Err()
		case msg, ok := <-outputChan:
			if !ok {
				slog.Warn("Output chan closed")
				return nil
			}
			if err := stream.Send(&socket.RawResponse{Data: msg}); err != nil {
				slog.Error(
					"Could not send raw message from miniTicker stream to client",
					"error",
					err,
				)
				return err
			}
		}
	}
}

func (s *server) ReceiveRawAggTrade(
	req *socket.RawAggTradeRequest,
	stream socket.SocketService_ReceiveRawAggTradeServer,
) error {
	slog.Info("Client connected to ReceiveRawMiniTicker stream")

	symbol := req.Symbol
	outputChan := s.connManager.GetOrCreateConnection(symbol)

	for {
		select {
		case <-stream.Context().Done():
			slog.Warn("Got Interruption signal from streaming server from stream context")
			return stream.Context().Err()
		case <-s.mainCtx.Done():
			slog.Info("Got Interruption signal from streaming server from main context")
			return stream.Context().Err()
		case msg, ok := <-outputChan:
			if !ok {
				slog.Warn("Output chan closed")
				return nil
			}
			if err := stream.Send(&socket.RawResponse{Data: msg}); err != nil {
				slog.Error(
					"Could not send raw message from aggTrade stream to clietn",
					"error",
					err,
				)
				return err
			}
		}
	}
}

func StartServer(wg *sync.WaitGroup, connManager ConnectionManager, ctx context.Context) {
	defer wg.Done()

	address := getenv.GetString("ADDRESS", "0.0.0.0:12345")

	listen, err := net.Listen("tcp", address)
	if err != nil {
		slog.Error("Could not listening connection", "error", err)
		return
	}

	svr := grpc.NewServer()

	register(svr, connManager, ctx)

	slog.Info("👂 Server listening", "address", address)

	go func() {
		if err := svr.Serve(listen); err != nil {
			slog.Error("Failed to listening server")
		}
	}()

	<-ctx.Done()
	slog.Info("Got interruption signal, stopping gRPC server")
	svr.GracefulStop()
}
