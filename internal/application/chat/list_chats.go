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

	// Collect all chat IDs for batch queries.
	chatIDs := make([]uuid.UUID, len(chats))
	for i, c := range chats {
		chatIDs[i] = c.ID
	}

	// Batch-query last messages and unread counts in 2 queries total (instead of 2*N).
	lastMsgs, err := uc.msgRepo.GetLastMessages(ctx, chatIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get last messages: %w", err)
	}

	unreadCounts, err := uc.msgRepo.GetUnreadCounts(ctx, chatIDs, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get unread counts: %w", err)
	}

	result := make([]*domain.ChatWithPreview, 0, len(chats))
	for _, c := range chats {
		result = append(result, &domain.ChatWithPreview{
			Chat:        *c,
			LastMessage: lastMsgs[c.ID],
			UnreadCount: unreadCounts[c.ID],
		})
	}

	return &ListChatsOutput{
		Chats: result,
		Total: total,
	}, nil
}
