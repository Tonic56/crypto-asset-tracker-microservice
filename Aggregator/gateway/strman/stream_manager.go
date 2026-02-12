package strman

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"github.com/Tonic56/crypto-asset-tracker-microservice/Aggregator/gateway/converting"
)

type StreamManager struct {
	streams   map[string]context.CancelFunc
	Followers map[string]map[string]struct{} 
	mu        sync.RWMutex
}

func NewStreamManager() *StreamManager {
	return &StreamManager{
		streams:   make(map[string]context.CancelFunc),
		Followers: make(map[string]map[string]struct{}),
	}
}

func (sm *StreamManager) AddCoin(
	ctxParent context.Context,
	wg *sync.WaitGroup,
	symbol string,
	userID string, 
	outChan chan []byte,
) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	symbol = strings.ToLower(symbol)

	
	if _, ok := sm.Followers[symbol]; !ok {
		sm.Followers[symbol] = make(map[string]struct{})
	}

	
	if _, exists := sm.Followers[symbol][userID]; exists {
		slog.Info(
			"User already subscribed to coin, re-subscription is not required", "symbol", symbol, "userID", userID,
		)
		return false
	}

	
	sm.Followers[symbol][userID] = struct{}{}

	
	if _, streamExists := sm.streams[symbol]; !streamExists {
		slog.Info("First subscriber, starting new stream", "symbol", symbol, "userID", userID)
		sm.start(ctxParent, wg, symbol, outChan)
	} else {
		slog.Info("Adding subscriber to existing stream", "symbol", symbol, "userID", userID, "total_followers", len(sm.Followers[symbol]))
	}
	return true
}

func (sm *StreamManager) start(
	ctxParent context.Context,
	wg *sync.WaitGroup,
	symbol string,
	outChan chan []byte,
) {
	ctx, cancel := context.WithCancel(ctxParent)
	sm.streams[symbol] = cancel

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			sm.mu.Lock()
			delete(sm.streams, symbol) 
			slog.Info(
				"Stream processing stopped and cleaned up", "symbol", symbol,
			)
			sm.mu.Unlock()
		}()
		converting.ReceiveAggTradeMessage(ctx, symbol, outChan)
	}()
}

func (sm *StreamManager) DeleteCoin(symbol string, userID string) { 
	sm.mu.Lock()
	defer sm.mu.Unlock()

	symbol = strings.ToLower(symbol)

	
	if users, ok := sm.Followers[symbol]; ok {
		
		if _, exists := users[userID]; exists {
			delete(sm.Followers[symbol], userID)
			slog.Info("User removed from coin", "symbol", symbol, "userID", userID)
		}

		
		if len(sm.Followers[symbol]) == 0 {
			if cancel, exists := sm.streams[symbol]; exists {
				cancel() 
				
				slog.Info("Last user unsubscribed, stream cancellation signal sent", "symbol", symbol)
			}
		}
	}
}