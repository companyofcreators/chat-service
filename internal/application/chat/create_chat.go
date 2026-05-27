package chat

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	domain "github.com/companyofcreators/chat-service/internal/domain/chat"
)

type CreateChatInput struct {
	OrderID    uuid.UUID
	CustomerID uuid.UUID
	MasterID   uuid.UUID
	OrderTitle string
}

type CreateChatUseCase struct {
	chatRepo domain.ChatRepository
}

func NewCreateChatUseCase(chatRepo domain.ChatRepository) *CreateChatUseCase {
	return &CreateChatUseCase{chatRepo: chatRepo}
}

func (uc *CreateChatUseCase) Execute(ctx context.Context, input CreateChatInput) (*domain.Chat, error) {
	if input.OrderID == uuid.Nil {
		return nil, domain.ErrInvalidOrderID
	}
	if input.CustomerID == uuid.Nil || input.MasterID == uuid.Nil {
		return nil, domain.ErrInvalidUserID
	}

	existing, err := uc.chatRepo.FindByOrder(ctx, input.OrderID)
	if err != nil && err != domain.ErrChatNotFound {
		return nil, fmt.Errorf("failed to check existing chat: %w", err)
	}
	if existing != nil {
		return nil, domain.ErrChatAlreadyExists
	}

	chat := &domain.Chat{
		ID:         uuid.New(),
		OrderID:    input.OrderID,
		CustomerID: input.CustomerID,
		MasterID:   input.MasterID,
		OrderTitle: input.OrderTitle,
		CreatedAt:  time.Now().UTC(),
	}

	if err := uc.chatRepo.Create(ctx, chat); err != nil {
		return nil, fmt.Errorf("failed to create chat: %w", err)
	}

	return chat, nil
}
