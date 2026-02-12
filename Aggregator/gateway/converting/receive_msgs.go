package converting

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Aggregator/lib/getenv"
	socket "github.com/Tonic56/proto-crypto-asset-tracker/proto/gen/go/socket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	Address    = getenv.GetString("SOCKET_SERVICE_ADDR", "socket-service:50051")
	MaxRetries = getenv.GetInt("SOCKET_SERVICE_MAX_RETRIES", 10)
)

type StreamReceiver interface {
	Recv() (*socket.RawResponse, error)
}

func DistributeMessages(
	ctx context.Context,
	wg *sync.WaitGroup,
	rawChan, aggTradeChan, miniTickerChan chan []byte,
) {
	defer wg.Done()
	defer close(aggTradeChan)
	defer close(miniTickerChan)

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-rawChan:
			if !ok {
				return
			}
			if len(msg) == 0 {
				continue
			}

			if msg[0] == '{' {
				select {
				case <-ctx.Done():
					return
				case aggTradeChan <- msg:
				}
			}

			if msg[0] == '[' {
				select {
				case <-ctx.Done():
					return
				case miniTickerChan <- msg:
				}
			}
		}
	}
}

func ReceiveMiniTickerMessage(ctx context.Context, wg *sync.WaitGroup, outChan chan []byte) {
	defer wg.Done()

	conn, err := createClientConn(ctx)
	if err != nil {
		slog.Error("Failed to connect after retries",
			"address", Address,
			"error", err,
		)
	}
	defer conn.Close()

	client := socket.NewSocketServiceClient(conn)

	stream, err := createMiniTickerStream(ctx, client)
	if err != nil {
		slog.Error("Could not set up MiniTicker stream", "error", err)
		return 
	}

	receiveMessages(ctx, stream, outChan)
}

func ReceiveAggTradeMessage(
	ctx context.Context,
	symbol string,
	outChan chan []byte,
) {
	conn, err := createClientConn(ctx)
	if err != nil {
		slog.Error("Failed to connect after retries",
			"address", Address,
			"error", err,
		)
		return
	}
	defer conn.Close()

	client := socket.NewSocketServiceClient(conn)

	stream, err := createAggTradeStream(ctx, client, symbol)
	if err != nil {
		slog.Error("Could not set up AggTrade stream", "error", err)
		return
	}

	receiveMessages(ctx, stream, outChan)
}

func createClientConn(ctx context.Context) (*grpc.ClientConn, error) {
	var conn *grpc.ClientConn
	var err error

	for i := 0; i < MaxRetries; i++ {
		select {
		case <-ctx.Done():
			slog.Info("Connection cancelled by context")
			return nil, ctx.Err()
		default:
			conn, err = grpc.NewClient(
				Address,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err == nil {
				return conn, nil
			}

			delay := time.Duration(i+1) * time.Second
			slog.Warn("Failed to connect, retrying...",
				"attempt", i+1,
				"delay", delay,
				"error", err,
			)

			select {
			case <-ctx.Done():
				slog.Info("Connection cancelled during retry delay")
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return nil, err
}

func createMiniTickerStream(
	ctx context.Context,
	client socket.SocketServiceClient,
) (StreamReceiver, error) {
	var stream socket.SocketService_ReceiveRawMiniTickerClient
	var err error

	for i := 0; i < MaxRetries; i++ {
		select {
		case <-ctx.Done():

			return nil, ctx.Err()
		default:
			stream, err = client.ReceiveRawMiniTicker(ctx, &socket.RawMiniTickerRequest{})
			if err == nil {
				return stream, err
			}

			delay := time.Duration(i+1) * time.Second
			slog.Warn("Failed to connect, retrying...",
				"attempt", i+1,
				"delay", delay,
				"error", err,
			)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return nil, err
}

func createAggTradeStream(
	ctx context.Context,
	client socket.SocketServiceClient,
	symbol string,
) (StreamReceiver, error) {
	var stream socket.SocketService_ReceiveRawAggTradeClient
	var err error

	for i := 0; i < MaxRetries; i++ {
		select {
		case <-ctx.Done():
			slog.Info("Stream creation cancelled by context")
			return nil, ctx.Err()
		default:
			stream, err = client.ReceiveRawAggTrade(ctx, &socket.RawAggTradeRequest{Symbol: symbol})
			if err == nil {
				return stream, err
			}

			delay := time.Duration(i+1) * time.Second
			slog.Warn("Failed to connect, retrying...",
				"attempt", i+1,
				"delay", delay,
				"error", err,
			)

			select {
			case <-ctx.Done():
				slog.Info("Stream creation cancelled during retry delay")
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return nil, err
}

func receiveMessages(ctx context.Context, stream StreamReceiver, outChan chan<- []byte) {
	slog.Info("📞 Starting to receive messages from stream")

	for {
		select {
		case <-ctx.Done():
			slog.Info("Got Interruption signal, stopping to receive messages from stream")
			return
		default:
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					slog.Info("Stream closed by server")
					return
				}
				slog.Error("Got error from stream during receiving", "error", err)
				return
			}
			select {
			case <-ctx.Done():
				return
			case outChan <- resp.Data:
			}
		}
	}
}
