package connsock

import (
	"context"
	"log/slog"
	"sync"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Socket/lib/getenv"
)

type ConnectionManager struct {
	connections map[string]*socketProducer
	mu          sync.RWMutex
	mainCtx     context.Context
	mainWg      *sync.WaitGroup
}

func NewConnectionManager(ctx context.Context, wg *sync.WaitGroup) *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*socketProducer),
		mainCtx:     ctx,
		mainWg:      wg,
	}
}

func (cm *ConnectionManager) GetOrCreateConnection(symbol string) <-chan []byte {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if conn, exists := cm.connections[symbol]; exists {
		slog.Info("Reusing existing connection", "symbol", symbol)
		return conn.outputChan
	}

	slog.Info("Creating new connection", "symbol", symbol)

	outputChan := make(chan []byte, 100)
	msgChan := make(chan error, 1)
	url := getenv.GetString("AGGTRADE_URL", "I don't know") + symbol + EvTypeAggTrade

	socketConn := NewSocketProduecer(outputChan, url, msgChan)

	cm.mainWg.Add(1)
	go socketConn.Start(cm.mainCtx, cm.mainWg)

	cm.connections[symbol] = socketConn
	return outputChan
}

func (cm *ConnectionManager) GetMiniTickerConnection() <-chan []byte {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	key := "miniTicker"
	if conn, exists := cm.connections[key]; exists {
		slog.Info("Reusing existing miniTicker connection")
		return conn.outputChan
	}

	slog.Info("Creating new miniTicker connection")

	outputChan := make(chan []byte, 100)
	msgChan := make(chan error, 1)
	url := getenv.GetString("MINITICKER_URL", "I don't care")

	socketConn := NewSocketProduecer(outputChan, url, msgChan)

	cm.mainWg.Add(1)
	go socketConn.Start(cm.mainCtx, cm.mainWg)

	cm.connections[key] = socketConn
	return outputChan
}

func (cm *ConnectionManager) CloseAll() {
	slog.Info("Closing all connections", "count", len(cm.connections))
}
