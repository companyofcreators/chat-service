package chat

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	domain "github.com/companyofcreators/chat-service/internal/domain/chat"
)

type GetMessagesInput struct {
	ChatID uuid.UUID
	UserID uuid.UUID
	Limit  int
	Offset int
}

type GetMessagesOutput struct {
	Messages []*domain.Message
	Total    int
}

type GetMessagesUseCase struct {
	chatRepo domain.ChatRepository
	msgRepo  domain.MessageRepository
}

func NewGetMessagesUseCase(
	chatRepo domain.ChatRepository,
	msgRepo domain.MessageRepository,
) *GetMessagesUseCase {
	return &GetMessagesUseCase{
		chatRepo: chatRepo,
		msgRepo:  msgRepo,
	}
}

func (uc *GetMessagesUseCase) Execute(ctx context.Context, input GetMessagesInput) (*GetMessagesOutput, error) {
	if input.Limit <= 0 || input.Limit > 100 {
		input.Limit = 50
	}

	chat, err := uc.chatRepo.FindByID(ctx, input.ChatID)
	if err != nil {
		return nil, fmt.Errorf("failed to find chat: %w", err)
	}

	if !chat.IsParticipant(input.UserID) {
		return nil, domain.ErrNotParticipant
	}

	messages, total, err := uc.msgRepo.ListByChat(ctx, input.ChatID, input.Limit, input.Offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}

	return &GetMessagesOutput{
		Messages: messages,
		Total:    total,
	}, nil
}
