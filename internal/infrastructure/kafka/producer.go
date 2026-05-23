package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	domain "github.com/companyofcreators/chat-service/internal/domain/chat"
)

const (
	TopicMessageSent = "chat.message.sent"
)

// Producer wraps Kafka writer for publishing events.
type Producer struct {
	writer *kafka.Writer
	logger *slog.Logger
}

func NewProducer(brokers []string, logger *slog.Logger) *Producer {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireAll,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  5 * time.Second,
	}

	return &Producer{
		writer: writer,
		logger: logger,
	}
}

// PublishMessageSent publishes a chat.message.sent event to Kafka.
func (p *Producer) PublishMessageSent(ctx context.Context, msg *domain.Message, chat *domain.Chat) error {
	receiverID := chat.OtherParticipant(msg.SenderID)

	preview := msg.Message
	if len(preview) > 200 {
		preview = preview[:200]
	}

	event := MessageSentEvent{
		MessageID:      msg.ID.String(),
		ChatID:         msg.ChatID.String(),
		SenderID:       msg.SenderID.String(),
		ReceiverID:     receiverID.String(),
		MessagePreview: preview,
		Timestamp:      msg.CreatedAt,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal message sent event: %w", err)
	}

	key := uuid.New().String()

	m := kafka.Message{
		Key:   []byte(key),
		Value: data,
		Topic: TopicMessageSent,
	}

	if err := p.writer.WriteMessages(ctx, m); err != nil {
		return fmt.Errorf("failed to write kafka message: %w", err)
	}

	p.logger.Debug("published message sent event",
		"message_id", event.MessageID,
		"chat_id", event.ChatID,
	)

	return nil
}

// Close shuts down the Kafka producer.
func (p *Producer) Close() error {
	return p.writer.Close()
}

// MessageSentEvent represents the Kafka event for a sent message.
type MessageSentEvent struct {
	MessageID      string      `json:"message_id"`
	ChatID         string      `json:"chat_id"`
	SenderID       string      `json:"sender_id"`
	ReceiverID     string      `json:"receiver_id"`
	MessagePreview string      `json:"message_preview"`
	Timestamp      interface{} `json:"timestamp"`
}
