package ws

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	appChat "github.com/companyofcreators/chat-service/internal/application/chat"
	domain "github.com/companyofcreators/chat-service/internal/domain/chat"
	redisInfra "github.com/companyofcreators/chat-service/internal/infrastructure/redis"
	wsinfra "github.com/companyofcreators/chat-service/internal/infrastructure/websocket"
)

// Handler manages WebSocket connections and routes incoming messages.
type Handler struct {
	hub          *wsinfra.Hub
	pubsub       *redisInfra.PubSub
	sendMessage  *appChat.SendMessageUseCase
	markRead     *appChat.MarkReadUseCase
	jwtPublicKey *rsa.PublicKey
	logger       *slog.Logger
}

func NewHandler(
	hub *wsinfra.Hub,
	pubsub *redisInfra.PubSub,
	sendMessage *appChat.SendMessageUseCase,
	markRead *appChat.MarkReadUseCase,
	jwtPublicKey *rsa.PublicKey,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		hub:          hub,
		pubsub:       pubsub,
		sendMessage:  sendMessage,
		markRead:     markRead,
		jwtPublicKey: jwtPublicKey,
		logger:       logger,
	}
}

// Upgrade handles HTTP upgrade to WebSocket. It validates the JWT token from
// the query parameter, extracts the user ID, and establishes a WebSocket connection.
func (h *Handler) Upgrade(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "отсутствует параметр token", http.StatusUnauthorized)
		return
	}

	userID, err := h.validateToken(tokenStr)
	if err != nil {
		h.logger.Warn("invalid JWT token", "error", err)
		http.Error(w, "недействительный токен", http.StatusUnauthorized)
		return
	}

	acceptOptions := &websocket.AcceptOptions{
		InsecureSkipVerify: true, // In production, use proper origin checking.
	}

	conn, err := websocket.Accept(w, r, acceptOptions)
	if err != nil {
		h.logger.Error("failed to accept websocket connection", "error", err)
		return
	}

	client := wsinfra.NewClient(r.Context(), userID, conn, h.hub, h.logger)
	h.hub.Register(client)

	go client.WritePump(h.logger)
	go client.ReadPump(h.logger)
}

// OnMessage is the callback used by the Hub to handle incoming WebSocket messages.
func (h *Handler) OnMessage(ctx context.Context, client *wsinfra.Client, raw []byte) error {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return errors.New("недопустимый формат сообщения")
	}

	switch envelope.Type {
	case "message.send":
		return h.handleMessageSend(ctx, client, raw)
	case "typing.start":
		return h.handleTyping(ctx, client, raw, true)
	case "typing.stop":
		return h.handleTyping(ctx, client, raw, false)
	case "messages.read":
		return h.handleMessagesRead(ctx, client, raw)
	case "ping":
		client.Send <- wsinfra.MarshalPong()
		return nil
	default:
		return errors.New("неизвестный тип сообщения: " + envelope.Type)
	}
}

func (h *Handler) handleMessageSend(ctx context.Context, client *wsinfra.Client, raw []byte) error {
	var req struct {
		Type             string  `json:"type"`
		ChatID           string  `json:"chat_id"`
		Message          string  `json:"message"`
		AttachmentFileID *string `json:"attachment_file_id,omitempty"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return errors.New("недопустимые данные message.send")
	}

	chatID, err := uuid.Parse(req.ChatID)
	if err != nil {
		return errors.New("недействительный chat_id")
	}

	var attachmentFileID *uuid.UUID
	if req.AttachmentFileID != nil {
		id, err := uuid.Parse(*req.AttachmentFileID)
		if err == nil {
			attachmentFileID = &id
		}
	}

	input := appChat.SendMessageInput{
		ChatID:           chatID,
		SenderID:         client.UserID,
		Message:          req.Message,
		AttachmentFileID: attachmentFileID,
	}

	msg, chat, err := h.sendMessage.Execute(ctx, input)
	if err != nil {
		if errors.Is(err, domain.ErrEmptyMessage) {
			return err
		}
		if errors.Is(err, domain.ErrNotParticipant) {
			return err
		}
		h.logger.Error("не удалось отправить сообщение", "error", err)
		return errors.New("не удалось отправить сообщение")
	}

	// Send confirmation back to sender.
	event := redisInfra.MessageEvent{
		Type:   "message.new",
		ChatID: msg.ChatID.String(),
		Message: redisInfra.MessageData{
			ID:               msg.ID.String(),
			ChatID:           msg.ChatID.String(),
			SenderID:         msg.SenderID.String(),
			Message:          msg.Message,
			CreatedAt:        msg.CreatedAt.Format(time.RFC3339),
		},
	}

	if msg.AttachmentFileID != nil {
		s := msg.AttachmentFileID.String()
		event.Message.AttachmentFileID = &s
	}

	// Send to sender.
	confirmData, _ := json.Marshal(event)
	client.Send <- confirmData

	// Publish to Redis for multi-instance fanout to the recipient.
	event.Recipient = chat.OtherParticipant(client.UserID).String()
	if err := h.pubsub.PublishMessage(ctx, &event); err != nil {
		h.logger.Error("failed to publish message to redis", "error", err)
	}

	return nil
}

func (h *Handler) handleTyping(ctx context.Context, client *wsinfra.Client, raw []byte, isTyping bool) error {
	var req struct {
		Type   string `json:"type"`
		ChatID string `json:"chat_id"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return errors.New("недопустимые данные typing")
	}

	chatID, err := uuid.Parse(req.ChatID)
	if err != nil {
		return errors.New("недействительный chat_id")
	}

	event := redisInfra.TypingEvent{
		Type:     "typing",
		ChatID:   chatID.String(),
		UserID:   client.UserID.String(),
		IsTyping: isTyping,
	}

	if err := h.pubsub.PublishTyping(ctx, &event); err != nil {
		h.logger.Error("failed to publish typing event", "error", err)
	}

	return nil
}

func (h *Handler) handleMessagesRead(ctx context.Context, client *wsinfra.Client, raw []byte) error {
	var req struct {
		Type   string `json:"type"`
		ChatID string `json:"chat_id"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return errors.New("недопустимые данные messages.read")
	}

	chatID, err := uuid.Parse(req.ChatID)
	if err != nil {
		return errors.New("недействительный chat_id")
	}

	input := appChat.MarkReadInput{
		ChatID: chatID,
		UserID: client.UserID,
	}

	if err := h.markRead.Execute(ctx, input); err != nil {
		if errors.Is(err, domain.ErrNotParticipant) {
			return err
		}
		h.logger.Error("не удалось отметить сообщения как прочитанные", "error", err)
		return errors.New("не удалось отметить сообщения как прочитанные")
	}

	// Notify the other participant that messages have been read.
	readEvent := map[string]interface{}{
		"type":    "messages.read",
		"chat_id": chatID.String(),
		"user_id": client.UserID.String(),
	}

	// Publish to Redis for multi-instance fanout.
	_ = h.pubsub.Publish(ctx, redisInfra.ChannelMessages, readEvent)

	return nil
}

// validateToken validates a JWT token and extracts the user ID.
func (h *Handler) validateToken(tokenStr string) (uuid.UUID, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New("неожиданный метод подписи")
		}
		return h.jwtPublicKey, nil
	})
	if err != nil {
		return uuid.Nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return uuid.Nil, errors.New("недействительные claims токена")
	}

	userIDStr, ok := claims["sub"].(string)
	if !ok {
		return uuid.Nil, errors.New("отсутствует sub claim")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, errors.New("недействительный ID пользователя в токене")
	}

	return userID, nil
}
