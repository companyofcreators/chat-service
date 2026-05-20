package app

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"

	"github.com/jmoiron/sqlx"

	appChat "github.com/companyofcreators/chat-service/internal/application/chat"
	"github.com/companyofcreators/chat-service/internal/config"
	"github.com/companyofcreators/chat-service/internal/domain/chat"
	"github.com/companyofcreators/chat-service/internal/infrastructure/db"
	"github.com/companyofcreators/chat-service/internal/infrastructure/kafka"
	"github.com/companyofcreators/chat-service/internal/infrastructure/redis"
	wsinfra "github.com/companyofcreators/chat-service/internal/infrastructure/websocket"
	httpHandler "github.com/companyofcreators/chat-service/internal/interfaces/http"
	wsHandler "github.com/companyofcreators/chat-service/internal/interfaces/ws"
	"github.com/companyofcreators/chat-service/internal/pkg"
)

// Container holds all application dependencies.
type Container struct {
	Config    *config.Config
	Logger    *slog.Logger
	DB        *sqlx.DB

	// Repositories
	ChatRepo    chat.ChatRepository
	MessageRepo chat.MessageRepository

	// Infrastructure
	Redis         *redis.PubSub
	KafkaProducer *kafka.Producer
	Hub           *wsinfra.Hub

	// Use cases
	CreateChat  *appChat.CreateChatUseCase
	SendMessage *appChat.SendMessageUseCase
	GetMessages *appChat.GetMessagesUseCase
	MarkRead    *appChat.MarkReadUseCase
	ListChats   *appChat.ListChatsUseCase

	// Handlers
	HTTPHandler *httpHandler.Handler
	WSHandler   *wsHandler.Handler
}

// NewContainer creates and wires all dependencies.
func NewContainer(ctx context.Context, cfg *config.Config) (*Container, error) {
	logger, err := pkg.NewLogger(cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	logger.Info("initializing chat service dependencies")

	// Database
	pool, err := db.NewPostgresPool(ctx, cfg.DBDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}
	logger.Info("connected to postgres")

	// Repositories
	chatRepo := db.NewChatRepo(pool)
	messageRepo := db.NewMessageRepo(pool)

	// Redis
	redisPubSub := redis.NewPubSub(cfg.RedisAddr, cfg.RedisPassword, logger)
	if err := redisPubSub.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}
	logger.Info("connected to redis")

	// Kafka
	kafkaProducer := kafka.NewProducer(cfg.KafkaBrokersList(), logger)
	logger.Info("kafka producer initialized")

	// Use cases
	createChatUC := appChat.NewCreateChatUseCase(chatRepo)
	listChatsUC := appChat.NewListChatsUseCase(chatRepo, messageRepo)
	getMessagesUC := appChat.NewGetMessagesUseCase(chatRepo, messageRepo)
	markReadUC := appChat.NewMarkReadUseCase(chatRepo, messageRepo)

	// Kafka MessageEventPublisher adapter for SendMessage use case.
	eventPub := &kafkaEventPublisher{producer: kafkaProducer, logger: logger}
	sendMessageUC := appChat.NewSendMessageUseCase(chatRepo, messageRepo, eventPub)

	// WebSocket Hub (placeholder onMessage; wired after WSHandler creation).
	hub := wsinfra.NewHub(redisPubSub, logger, nil)

	// HTTP Handler
	httpH := httpHandler.NewHandler(
		createChatUC,
		sendMessageUC,
		getMessagesUC,
		markReadUC,
		listChatsUC,
		chatRepo,
		logger,
	)

	// Load JWT public key for token validation.
	jwtPubKey, err := loadRSAPublicKey(cfg.JWTPublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load JWT public key: %w", err)
	}

	// WS Handler
	wsH := wsHandler.NewHandler(
		hub,
		redisPubSub,
		sendMessageUC,
		markReadUC,
		jwtPubKey,
		logger,
	)

	// Wire the hub's OnMessage callback to the WS handler.
	hub.SetOnMessage(wsH.OnMessage)

	return &Container{
		Config:        cfg,
		Logger:        logger,
		DB:            pool,
		ChatRepo:      chatRepo,
		MessageRepo:   messageRepo,
		Redis:         redisPubSub,
		KafkaProducer: kafkaProducer,
		Hub:           hub,
		CreateChat:    createChatUC,
		SendMessage:   sendMessageUC,
		GetMessages:   getMessagesUC,
		MarkRead:      markReadUC,
		ListChats:     listChatsUC,
		HTTPHandler:   httpH,
		WSHandler:     wsH,
	}, nil
}

// Shutdown gracefully shuts down all dependencies.
func (c *Container) Shutdown(ctx context.Context) {
	c.Logger.Info("shutting down chat service")

	if c.KafkaProducer != nil {
		if err := c.KafkaProducer.Close(); err != nil {
			c.Logger.Error("failed to close kafka producer", "error", err)
		}
	}

	if c.Redis != nil {
		if err := c.Redis.Close(); err != nil {
			c.Logger.Error("failed to close redis", "error", err)
		}
	}

	if c.DB != nil {
		c.DB.Close()
	}

	c.Logger.Info("chat service shutdown complete")
}

// loadRSAPublicKey reads and parses an RSA public key from a PEM file.
func loadRSAPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read public key file: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}
	return rsaPub, nil
}

// kafkaEventPublisher adapts the Kafka producer to the MessageEventPublisher interface.
type kafkaEventPublisher struct {
	producer *kafka.Producer
	logger   *slog.Logger
}

func (p *kafkaEventPublisher) PublishMessageSent(ctx context.Context, msg *chat.Message, c *chat.Chat) error {
	return p.producer.PublishMessageSent(ctx, msg, c)
}
