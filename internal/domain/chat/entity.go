package chat

import (
	"time"

	"github.com/google/uuid"
)

type Chat struct {
	ID         uuid.UUID
	OrderID    uuid.UUID
	CustomerID uuid.UUID
	MasterID   uuid.UUID
	OrderTitle string
	CreatedAt  time.Time
}

type Message struct {
	ID               uuid.UUID
	ChatID           uuid.UUID
	SenderID         uuid.UUID
	Message          string
	AttachmentFileID *uuid.UUID
	CreatedAt        time.Time
	ReadAt           *time.Time
}

type ChatWithPreview struct {
	Chat
	LastMessage   *Message
	UnreadCount   int
	OtherUserName string
}

type TypingEvent struct {
	ChatID   uuid.UUID
	UserID   uuid.UUID
	IsTyping bool
}

// IsParticipant checks if the given user is a participant of the chat.
func (c *Chat) IsParticipant(userID uuid.UUID) bool {
	return c.CustomerID == userID || c.MasterID == userID
}

// OtherParticipant returns the ID of the other participant in the chat.
func (c *Chat) OtherParticipant(userID uuid.UUID) uuid.UUID {
	if c.CustomerID == userID {
		return c.MasterID
	}
	return c.CustomerID
}
