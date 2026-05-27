package chat

import (
	"context"

	"github.com/google/uuid"
)

type ChatRepository interface {
	Create(ctx context.Context, c *Chat) error
	FindByID(ctx context.Context, id uuid.UUID) (*Chat, error)
	FindByOrder(ctx context.Context, orderID uuid.UUID) (*Chat, error)
	ListByUser(ctx context.Context, userID uuid.UUID, orderID *uuid.UUID, limit, offset int) ([]*Chat, int, error)
	FindByOrderAndUsers(ctx context.Context, orderID, customerID, masterID uuid.UUID) (*Chat, error)
}

type MessageRepository interface {
	Create(ctx context.Context, m *Message) error
	ListByChat(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*Message, int, error)
	MarkAsRead(ctx context.Context, chatID, userID uuid.UUID) error
	GetUnreadCount(ctx context.Context, chatID, userID uuid.UUID) (int, error)
	GetLastMessage(ctx context.Context, chatID uuid.UUID) (*Message, error)
	// Batch queries to avoid N+1 when listing chats with previews.
	GetLastMessages(ctx context.Context, chatIDs []uuid.UUID) (map[uuid.UUID]*Message, error)
	GetUnreadCounts(ctx context.Context, chatIDs []uuid.UUID, userID uuid.UUID) (map[uuid.UUID]int, error)
}
