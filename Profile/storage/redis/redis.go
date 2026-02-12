package redis

import (
	"context"
	"log/slog"
	"os"
	"sync"

	"github.com/redis/go-redis/v9"
)

type Message struct {
	Channel string
	Payload string
}

type Subscriber struct {
	client        *redis.Client
	Messages      chan Message
	subscriptions map[string]*redis.PubSub
	mu            sync.RWMutex
	log           *slog.Logger
}

func NewSubscriber(log *slog.Logger) *Subscriber {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379" 
	}

	return &Subscriber{
		client: redis.NewClient(&redis.Options{
			Addr:     redisAddr,
			Password: "", 
			DB:       0,  
		}),
		Messages:      make(chan Message, 1000), 
		subscriptions: make(map[string]*redis.PubSub),
		log:           log,
	}
}

func (s *Subscriber) Subscribe(ctx context.Context, symbol string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	
	if _, exists := s.subscriptions[symbol]; exists {
		return nil
	}

	pubsub := s.client.Subscribe(ctx, symbol)

	
	_, err := pubsub.Receive(ctx)
	if err != nil {
		s.log.Error("failed to subscribe to redis channel", "channel", symbol, "error", err)
		return err
	}

	s.subscriptions[symbol] = pubsub
	s.log.Info("subscribed to new redis channel", "channel", symbol)

	
	go s.listener(ctx, pubsub)

	return nil
}

func (s *Subscriber) Unsubscribe(ctx context.Context, symbol string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pubsub, exists := s.subscriptions[symbol]
	if !exists {
		return nil 
	}

	delete(s.subscriptions, symbol)

	
	if err := pubsub.Unsubscribe(ctx, symbol); err != nil {
		s.log.Error("failed to unsubscribe from channel", "channel", symbol, "error", err)
		
	}

	
	if err := pubsub.Close(); err != nil {
		s.log.Warn("error closing pubsub", "channel", symbol, "error", err)
	}

	s.log.Info("unsubscribed from redis channel", "channel", symbol)
	return nil
}

func (s *Subscriber) listener(ctx context.Context, pubsub *redis.PubSub) {
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			s.log.Info("listener stopped due to context cancellation")
			return
		case msg, ok := <-ch:
			if !ok {
				s.log.Warn("redis pubsub channel closed")
				return
			}

			
			select {
			case s.Messages <- Message{Channel: msg.Channel, Payload: msg.Payload}:
			default:
				s.log.Warn("messages channel full, dropping message")
			}
		}
	}
}

func (s *Subscriber) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.log.Info("closing redis subscriber...")
	for _, pubsub := range s.subscriptions {
		pubsub.Close()
	}

	if s.client != nil {
		s.client.Close()
	}

	
	if s.Messages != nil {
		close(s.Messages)
	}
	s.log.Info("redis subscriber closed")
}