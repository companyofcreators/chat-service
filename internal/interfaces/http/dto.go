package http

import (
	"time"

	"github.com/google/uuid"

	domain "github.com/companyofcreators/chat-service/internal/domain/chat"
)

// --- Request DTOs ---

type CreateChatRequest struct {
	OrderID    string `json:"order_id"`
	CustomerID string `json:"customer_id"`
	MasterID   string `json:"master_id"`
	OrderTitle string `json:"order_title"`
}

type SendMessageRequest struct {
	Message          string  `json:"message"`
	AttachmentFileID *string `json:"attachment_file_id,omitempty"`
}

type MarkReadRequest struct {
	UserID string `json:"user_id"`
}

// --- Response DTOs ---

type ChatResponse struct {
	ID         string `json:"id"`
	OrderID    string `json:"order_id"`
	OrderTitle string `json:"order_title"`
	CustomerID string `json:"customer_id"`
	MasterID   string `json:"master_id"`
	CreatedAt  string `json:"created_at"`
}

type MessageResponse struct {
	ID               string  `json:"id"`
	ChatID           string  `json:"chat_id"`
	SenderID         string  `json:"sender_id"`
	Message          string  `json:"message"`
	AttachmentFileID *string `json:"attachment_file_id,omitempty"`
	CreatedAt        string  `json:"created_at"`
	ReadAt           *string `json:"read_at,omitempty"`
}

type ChatWithPreviewResponse struct {
	ChatResponse
	LastMessage   *MessageResponse `json:"last_message,omitempty"`
	UnreadCount   int              `json:"unread_count"`
	OtherUserName string           `json:"other_user_name,omitempty"`
}

type ChatListResponse struct {
	Chats []*ChatWithPreviewResponse `json:"chats"`
	Total int                        `json:"total"`
}

type MessageListResponse struct {
	Messages []*MessageResponse `json:"messages"`
	Total    int                `json:"total"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// --- Mappers ---

func toChatResponse(c *domain.Chat) ChatResponse {
	return ChatResponse{
		ID:         c.ID.String(),
		OrderID:    c.OrderID.String(),
		OrderTitle: c.OrderTitle,
		CustomerID: c.CustomerID.String(),
		MasterID:   c.MasterID.String(),
		CreatedAt:  c.CreatedAt.Format(time.RFC3339),
	}
}

func toMessageResponse(m *domain.Message) MessageResponse {
	resp := MessageResponse{
		ID:        m.ID.String(),
		ChatID:    m.ChatID.String(),
		SenderID:  m.SenderID.String(),
		Message:   m.Message,
		CreatedAt: m.CreatedAt.Format(time.RFC3339),
	}

	if m.AttachmentFileID != nil {
		s := m.AttachmentFileID.String()
		resp.AttachmentFileID = &s
	}

	if m.ReadAt != nil {
		s := m.ReadAt.Format(time.RFC3339)
		resp.ReadAt = &s
	}

	return resp
}

func toChatWithPreviewResponse(cwp *domain.ChatWithPreview) ChatWithPreviewResponse {
	resp := ChatWithPreviewResponse{
		ChatResponse:  toChatResponse(&cwp.Chat),
		UnreadCount:   cwp.UnreadCount,
		OtherUserName: cwp.OtherUserName,
	}

	if cwp.LastMessage != nil {
		lm := toMessageResponse(cwp.LastMessage)
		resp.LastMessage = &lm
	}

	return resp
}

func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
