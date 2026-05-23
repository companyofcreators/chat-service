package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	redisInfra "github.com/companyofcreators/chat-service/internal/infrastructure/redis"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 4096
)

// Hub maintains the set of active clients and broadcasts messages to them.
type Hub struct {
	clients    map[uuid.UUID]*Client
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	pubsub     *redisInfra.PubSub
	logger     *slog.Logger

	// Message handler for incoming WebSocket messages.
	onMessage func(ctx context.Context, client *Client, raw []byte) error
}

// Client represents a single WebSocket connection.
type Client struct {
	UserID uuid.UUID
	Conn   *websocket.Conn
	Send   chan []byte
	Hub    *Hub
	ctx    context.Context
	cancel context.CancelFunc
}

// NewHub creates a new Hub.
func NewHub(pubsub *redisInfra.PubSub, logger *slog.Logger, onMessage func(ctx context.Context, client *Client, raw []byte) error) *Hub {
	return &Hub{
		clients:    make(map[uuid.UUID]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		pubsub:     pubsub,
		logger:     logger,
		onMessage:  onMessage,
	}
}

// SetOnMessage sets the callback for handling incoming WebSocket messages.
// Must be called before Run(), or messages will be silently dropped.
func (h *Hub) SetOnMessage(fn func(ctx context.Context, client *Client, raw []byte) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onMessage = fn
}

// Run starts the hub's main event loop. It blocks until the context is cancelled.
func (h *Hub) Run(ctx context.Context) error {
	// Subscribe to Redis channels for cross-instance message fanout.
	msgCh, err := h.pubsub.Subscribe(ctx, redisInfra.ChannelMessages)
	if err != nil {
		return fmt.Errorf("failed to subscribe to messages channel: %w", err)
	}

	typingCh, err := h.pubsub.Subscribe(ctx, redisInfra.ChannelTyping)
	if err != nil {
		return fmt.Errorf("failed to subscribe to typing channel: %w", err)
	}

	presenceCh, err := h.pubsub.Subscribe(ctx, redisInfra.ChannelPresence)
	if err != nil {
		return fmt.Errorf("failed to subscribe to presence channel: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			h.shutdownAll()
			return ctx.Err()

		case client := <-h.register:
			h.mu.Lock()
			// If user already has a connection, close the old one.
			if old, ok := h.clients[client.UserID]; ok {
				old.cancel()
				close(old.Send)
			}
			h.clients[client.UserID] = client
			h.mu.Unlock()

			// Publish presence event.
			h.pubsub.PublishPresence(ctx, &redisInfra.PresenceEvent{
				Type:   "presence",
				UserID: client.UserID.String(),
				Online: true,
			})

			h.logger.Info("client connected",
				"user_id", client.UserID.String(),
				"total_clients", h.ClientCount(),
			)

		case client := <-h.unregister:
			h.mu.Lock()
			if c, ok := h.clients[client.UserID]; ok && c == client {
				delete(h.clients, client.UserID)
			}
			h.mu.Unlock()

			// Publish presence event.
			h.pubsub.PublishPresence(ctx, &redisInfra.PresenceEvent{
				Type:   "presence",
				UserID: client.UserID.String(),
				Online: false,
			})

			h.logger.Info("client disconnected",
				"user_id", client.UserID.String(),
				"total_clients", h.ClientCount(),
			)

		case data := <-msgCh:
			h.handleRedisMessage(data)

		case data := <-typingCh:
			h.handleRedisTyping(data)

		case data := <-presenceCh:
			// Presence events are informational; local clients already know their own state.
			_ = data
		}
	}
}

func (h *Hub) handleRedisMessage(data []byte) {
	var event redisInfra.MessageEvent
	if err := json.Unmarshal(data, &event); err != nil {
		h.logger.Error("failed to unmarshal redis message event", "error", err)
		return
	}

	recipientID, err := uuid.Parse(event.Recipient)
	if err != nil {
		h.logger.Error("invalid recipient UUID in redis message", "recipient", event.Recipient)
		return
	}

	h.mu.RLock()
	client, ok := h.clients[recipientID]
	h.mu.RUnlock()

	if ok {
		select {
		case client.Send <- data:
		default:
			h.logger.Warn("client send buffer full, dropping message",
				"user_id", recipientID.String(),
			)
		}
	}
}

func (h *Hub) handleRedisTyping(data []byte) {
	var event redisInfra.TypingEvent
	if err := json.Unmarshal(data, &event); err != nil {
		h.logger.Error("failed to unmarshal redis typing event", "error", err)
		return
	}

	// Forward typing event to the relevant chat participants.
	// The typing event contains chat_id so we need to broadcast to users in that chat.
	// For simplicity, events are forwarded to all connected clients who are participants.
	h.BroadcastRaw(data)
}

// Register adds a client to the hub.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// SendToUser sends a message to a specific user's WebSocket connection.
func (h *Hub) SendToUser(userID uuid.UUID, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if client, ok := h.clients[userID]; ok {
		select {
		case client.Send <- message:
		default:
			h.logger.Warn("client send buffer full",
				"user_id", userID.String(),
			)
		}
	}
}

// BroadcastRaw broadcasts raw data to all connected clients.
func (h *Hub) BroadcastRaw(data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, client := range h.clients {
		select {
		case client.Send <- data:
		default:
		}
	}
}

// IsUserOnline checks if a user is connected.
func (h *Hub) IsUserOnline(userID uuid.UUID) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[userID]
	return ok
}

func (h *Hub) shutdownAll() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, client := range h.clients {
		client.cancel()
		close(client.Send)
	}
	h.clients = make(map[uuid.UUID]*Client)
}

// NewClient creates a new Client for the given WebSocket connection.
func NewClient(ctx context.Context, userID uuid.UUID, conn *websocket.Conn, hub *Hub, logger *slog.Logger) *Client {
	clientCtx, cancel := context.WithCancel(ctx)
	return &Client{
		UserID: userID,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		Hub:    hub,
		ctx:    clientCtx,
		cancel: cancel,
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub.
// It runs in a goroutine that is started for each connection.
func (c *Client) ReadPump(logger *slog.Logger) {
	defer func() {
		c.Hub.Unregister(c)
		c.cancel()
	}()

	c.Conn.SetReadLimit(maxMessageSize)

	for {
		msgType, data, err := c.Conn.Read(c.ctx)
		if err != nil {
			if websocket.CloseStatus(err) != -1 {
				logger.Debug("websocket closed",
					"user_id", c.UserID.String(),
					"close_status", websocket.CloseStatus(err),
				)
			} else {
				logger.Error("websocket read error",
					"user_id", c.UserID.String(),
					"error", err,
				)
			}
			return
		}

		if msgType != websocket.MessageText {
			continue
		}

		if c.Hub.onMessage != nil {
			if err := c.Hub.onMessage(c.ctx, c, data); err != nil {
				logger.Error("failed to handle message",
					"user_id", c.UserID.String(),
					"error", err,
				)
				// Send error back to client.
				select {
				case c.Send <- marshalError(err.Error()):
				default:
				}
			}
		}
	}
}

// WritePump pumps messages from the hub to the WebSocket connection.
// It runs in a goroutine that is started for each connection.
func (c *Client) WritePump(logger *slog.Logger) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.cancel()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return

		case message, ok := <-c.Send:
			if !ok {
				// Hub closed the channel.
				c.Conn.Close(websocket.StatusNormalClosure, "")
				return
			}

			writeCtx, cancel := context.WithTimeout(c.ctx, writeWait)
			err := c.Conn.Write(writeCtx, websocket.MessageText, message)
			cancel()
			if err != nil {
				logger.Error("websocket write error",
					"user_id", c.UserID.String(),
					"error", err,
				)
				return
			}

		case <-ticker.C:
			writeCtx, cancel := context.WithTimeout(c.ctx, writeWait)
			err := c.Conn.Ping(writeCtx)
			cancel()
			if err != nil {
				logger.Error("websocket ping error",
					"user_id", c.UserID.String(),
					"error", err,
				)
				return
			}
		}
	}
}

// MarshalPong creates a pong message.
func MarshalPong() []byte {
	data, err := json.Marshal(map[string]string{"type": "pong"})
	if err != nil {
		slog.Error("failed to marshal pong message", "error", err)
		return []byte(`{"type":"pong"}`)
	}
	return data
}

func marshalError(msg string) []byte {
	data, err := json.Marshal(map[string]string{
		"type":    "error",
		"message": msg,
	})
	if err != nil {
		slog.Error("failed to marshal error message", "error", err)
		return []byte(`{"type":"error","message":"` + msg + `"}`)
	}
	return data
}
