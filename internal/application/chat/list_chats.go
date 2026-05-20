package chat

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	domain "github.com/companyofcreators/chat-service/internal/domain/chat"
)

type ListChatsInput struct {
	UserID uuid.UUID
	Limit  int
	Offset int
}

type ListChatsOutput struct {
	Chats []*domain.ChatWithPreview
	Total int
}

type ListChatsUseCase struct {
	chatRepo domain.ChatRepository
	msgRepo  domain.MessageRepository
}

func NewListChatsUseCase(
	chatRepo domain.ChatRepository,
	msgRepo domain.MessageRepository,
) *ListChatsUseCase {
	return &ListChatsUseCase{
		chatRepo: chatRepo,
		msgRepo:  msgRepo,
	}
}

func (uc *ListChatsUseCase) Execute(ctx context.Context, input ListChatsInput) (*ListChatsOutput, error) {
	if input.Limit <= 0 || input.Limit > 100 {
		input.Limit = 20
	}

	chats, total, err := uc.chatRepo.ListByUser(ctx, input.UserID, input.Limit, input.Offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list chats: %w", err)
	}

	result := make([]*domain.ChatWithPreview, 0, len(chats))
	for _, c := range chats {
		lastMsg, err := uc.msgRepo.GetLastMessage(ctx, c.ID)
		if err != nil && err != domain.ErrChatNotFound {
			return nil, fmt.Errorf("failed to get last message for chat %s: %w", c.ID, err)
		}

		unread, err := uc.msgRepo.GetUnreadCount(ctx, c.ID, input.UserID)
		if err != nil {
			return nil, fmt.Errorf("failed to get unread count for chat %s: %w", c.ID, err)
		}

		result = append(result, &domain.ChatWithPreview{
			Chat:        *c,
			LastMessage: lastMsg,
			UnreadCount: unread,
		})
	}

	return &ListChatsOutput{
		Chats: result,
		Total: total,
	}, nil
}
