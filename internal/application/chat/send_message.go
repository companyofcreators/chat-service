package chat

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	domain "github.com/companyofcreators/chat-service/internal/domain/chat"
)

type SendMessageInput struct {
	ChatID           uuid.UUID
	SenderID         uuid.UUID
	Message          string
	AttachmentFileID *uuid.UUID
}

type SendMessageUseCase struct {
	chatRepo    domain.ChatRepository
	msgRepo     domain.MessageRepository
	eventPub    MessageEventPublisher
}

type MessageEventPublisher interface {
	PublishMessageSent(ctx context.Context, msg *domain.Message, chat *domain.Chat) error
}

func NewSendMessageUseCase(
	chatRepo domain.ChatRepository,
	msgRepo domain.MessageRepository,
	eventPub MessageEventPublisher,
) *SendMessageUseCase {
	return &SendMessageUseCase{
		chatRepo: chatRepo,
		msgRepo:  msgRepo,
		eventPub: eventPub,
	}
}

func (uc *SendMessageUseCase) Execute(ctx context.Context, input SendMessageInput) (*domain.Message, *domain.Chat, error) {
	if input.Message == "" {
		return nil, nil, domain.ErrEmptyMessage
	}

	chat, err := uc.chatRepo.FindByID(ctx, input.ChatID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find chat: %w", err)
	}

	if !chat.IsParticipant(input.SenderID) {
		return nil, nil, domain.ErrNotParticipant
	}

	msg := &domain.Message{
		ID:               uuid.New(),
		ChatID:           input.ChatID,
		SenderID:         input.SenderID,
		Message:          input.Message,
		AttachmentFileID: input.AttachmentFileID,
		CreatedAt:        time.Now().UTC(),
	}

	if err := uc.msgRepo.Create(ctx, msg); err != nil {
		return nil, nil, fmt.Errorf("failed to create message: %w", err)
	}

	if err := uc.eventPub.PublishMessageSent(ctx, msg, chat); err != nil {
		return nil, nil, fmt.Errorf("failed to publish message sent event: %w", err)
	}

	return msg, chat, nil
}
