package chat

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	domain "github.com/companyofcreators/chat-service/internal/domain/chat"
)

type MarkReadInput struct {
	ChatID uuid.UUID
	UserID uuid.UUID
}

type MarkReadUseCase struct {
	chatRepo domain.ChatRepository
	msgRepo  domain.MessageRepository
}

func NewMarkReadUseCase(
	chatRepo domain.ChatRepository,
	msgRepo domain.MessageRepository,
) *MarkReadUseCase {
	return &MarkReadUseCase{
		chatRepo: chatRepo,
		msgRepo:  msgRepo,
	}
}

func (uc *MarkReadUseCase) Execute(ctx context.Context, input MarkReadInput) error {
	chat, err := uc.chatRepo.FindByID(ctx, input.ChatID)
	if err != nil {
		return fmt.Errorf("failed to find chat: %w", err)
	}

	if !chat.IsParticipant(input.UserID) {
		return domain.ErrNotParticipant
	}

	if err := uc.msgRepo.MarkAsRead(ctx, input.ChatID, input.UserID); err != nil {
		return fmt.Errorf("failed to mark messages as read: %w", err)
	}

	return nil
}
