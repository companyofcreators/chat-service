package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/redis/go-redis/v9"
)

const (
	ChannelMessages = "chat:messages"
	ChannelTyping   = "chat:typing"
	ChannelPresence = "chat:presence"
)

// PubSub wraps Redis Pub/Sub for multi-instance WebSocket message fanout.
type PubSub struct {
	client *redis.Client
	logger *slog.Logger
	mu     sync.RWMutex
	subs   map[string]*redis.PubSub
}

func NewPubSub(addr, password string, logger *slog.Logger) *PubSub {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
	})

	return &PubSub{
		client: client,
		logger: logger,
		subs:   make(map[string]*redis.PubSub),
	}
}

// Ping checks the Redis connection.
func (ps *PubSub) Ping(ctx context.Context) error {
	return ps.client.Ping(ctx).Err()
}

// Publish sends a message to a Redis channel.
func (ps *PubSub) Publish(ctx context.Context, channel string, message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if err := ps.client.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("failed to publish to channel %s: %w", channel, err)
	}

	return nil
}

// Subscribe subscribes to a Redis channel and returns a Go channel of messages.
// The returned channel receives JSON-encoded byte slices.
func (ps *PubSub) Subscribe(ctx context.Context, channel string) (<-chan []byte, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	sub := ps.client.Subscribe(ctx, channel)
	ps.subs[channel] = sub

	// Wait for subscription confirmation.
	if _, err := sub.Receive(ctx); err != nil {
		return nil, fmt.Errorf("failed to subscribe to channel %s: %w", channel, err)
	}

	msgCh := make(chan []byte, 256)

	go func() {
		defer close(msgCh)
		defer sub.Close()

		ch := sub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				select {
				case msgCh <- []byte(msg.Payload):
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return msgCh, nil
}

// Close shuts down the Redis Pub/Sub client and all subscriptions.
func (ps *PubSub) Close() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	for _, sub := range ps.subs {
		sub.Close()
	}

	return ps.client.Close()
}

// PublishMessage publishes a new message event to the chat:messages channel.
func (ps *PubSub) PublishMessage(ctx context.Context, msg *MessageEvent) error {
	return ps.Publish(ctx, ChannelMessages, msg)
}

// PublishTyping publishes a typing indicator to the chat:typing channel.
func (ps *PubSub) PublishTyping(ctx context.Context, event *TypingEvent) error {
	return ps.Publish(ctx, ChannelTyping, event)
}

// PublishPresence publishes a presence event to the chat:presence channel.
func (ps *PubSub) PublishPresence(ctx context.Context, event *PresenceEvent) error {
	return ps.Publish(ctx, ChannelPresence, event)
}

// MessageEvent represents a chat message for Redis Pub/Sub fanout.
type MessageEvent struct {
	Type      string      `json:"type"`
	ChatID    string      `json:"chat_id"`
	Message   MessageData `json:"message"`
	Recipient string      `json:"recipient"`
}

type MessageData struct {
	ID               string  `json:"id"`
	ChatID           string  `json:"chat_id"`
	SenderID         string  `json:"sender_id"`
	Message          string  `json:"message"`
	AttachmentFileID *string `json:"attachment_file_id,omitempty"`
	CreatedAt        string  `json:"created_at"`
}

// TypingEvent represents a typing indicator for Redis Pub/Sub fanout.
type TypingEvent struct {
	Type     string `json:"type"`
	ChatID   string `json:"chat_id"`
	UserID   string `json:"user_id"`
	IsTyping bool   `json:"is_typing"`
}

// PresenceEvent represents a presence event for Redis Pub/Sub fanout.
type PresenceEvent struct {
	Type   string `json:"type"`
	UserID string `json:"user_id"`
	Online bool   `json:"online"`
	ChatID string `json:"chat_id,omitempty"`
}
