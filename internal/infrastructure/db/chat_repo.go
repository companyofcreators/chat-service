package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	domain "github.com/companyofcreators/chat-service/internal/domain/chat"
)

type ChatRepo struct {
	db *sqlx.DB
}

func NewChatRepo(db *sqlx.DB) *ChatRepo {
	return &ChatRepo{db: db}
}

func (r *ChatRepo) Create(ctx context.Context, c *domain.Chat) error {
	const query = `
		INSERT INTO chats (id, order_id, customer_id, master_id, order_title, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.ExecContext(ctx, query,
		c.ID, c.OrderID, c.CustomerID, c.MasterID, c.OrderTitle, c.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert chat: %w", err)
	}

	return nil
}

func (r *ChatRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Chat, error) {
	const query = `
		SELECT id, order_id, customer_id, master_id, COALESCE(order_title, ''), created_at
		FROM chats
		WHERE id = $1
	`

	c := &domain.Chat{}
	err := r.db.QueryRowxContext(ctx, query, id).Scan(
		&c.ID, &c.OrderID, &c.CustomerID, &c.MasterID, &c.OrderTitle, &c.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrChatNotFound
		}
		return nil, fmt.Errorf("failed to find chat by ID: %w", err)
	}

	return c, nil
}

func (r *ChatRepo) FindByOrder(ctx context.Context, orderID uuid.UUID) (*domain.Chat, error) {
	const query = `
		SELECT id, order_id, customer_id, master_id, COALESCE(order_title, ''), created_at
		FROM chats
		WHERE order_id = $1
	`

	c := &domain.Chat{}
	err := r.db.QueryRowxContext(ctx, query, orderID).Scan(
		&c.ID, &c.OrderID, &c.CustomerID, &c.MasterID, &c.OrderTitle, &c.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrChatNotFound
		}
		return nil, fmt.Errorf("failed to find chat by order: %w", err)
	}

	return c, nil
}

func (r *ChatRepo) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Chat, int, error) {
	const countQuery = `
		SELECT COUNT(*) FROM chats
		WHERE customer_id = $1 OR master_id = $1
	`

	var total int
	err := r.db.QueryRowxContext(ctx, countQuery, userID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count chats: %w", err)
	}

	const listQuery = `
		SELECT id, order_id, customer_id, master_id, COALESCE(order_title, ''), created_at
		FROM chats
		WHERE customer_id = $1 OR master_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryxContext(ctx, listQuery, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list chats: %w", err)
	}
	defer rows.Close()

	var chats []*domain.Chat
	for rows.Next() {
		c := &domain.Chat{}
		if err := rows.Scan(&c.ID, &c.OrderID, &c.CustomerID, &c.MasterID, &c.OrderTitle, &c.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan chat row: %w", err)
		}
		chats = append(chats, c)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating chat rows: %w", err)
	}

	return chats, total, nil
}

func (r *ChatRepo) FindByOrderAndUsers(ctx context.Context, orderID, customerID, masterID uuid.UUID) (*domain.Chat, error) {
	const query = `
		SELECT id, order_id, customer_id, master_id, COALESCE(order_title, ''), created_at
		FROM chats
		WHERE order_id = $1 AND customer_id = $2 AND master_id = $3
	`

	c := &domain.Chat{}
	err := r.db.QueryRowxContext(ctx, query, orderID, customerID, masterID).Scan(
		&c.ID, &c.OrderID, &c.CustomerID, &c.MasterID, &c.OrderTitle, &c.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrChatNotFound
		}
		return nil, fmt.Errorf("failed to find chat by order and users: %w", err)
	}

	return c, nil
}

// MessageRepo implements domain.MessageRepository.
type MessageRepo struct {
	db *sqlx.DB
}

func NewMessageRepo(db *sqlx.DB) *MessageRepo {
	return &MessageRepo{db: db}
}

func (r *MessageRepo) Create(ctx context.Context, m *domain.Message) error {
	const query = `
		INSERT INTO messages (id, chat_id, sender_id, message, attachment_file_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.ExecContext(ctx, query,
		m.ID, m.ChatID, m.SenderID, m.Message, m.AttachmentFileID, m.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert message: %w", err)
	}

	return nil
}

func (r *MessageRepo) ListByChat(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*domain.Message, int, error) {
	const countQuery = `
		SELECT COUNT(*) FROM messages WHERE chat_id = $1
	`

	var total int
	err := r.db.QueryRowxContext(ctx, countQuery, chatID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count messages: %w", err)
	}

	const listQuery = `
		SELECT id, chat_id, sender_id, message, attachment_file_id, created_at, read_at
		FROM messages
		WHERE chat_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryxContext(ctx, listQuery, chatID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list messages: %w", err)
	}
	defer rows.Close()

	var messages []*domain.Message
	for rows.Next() {
		m := &domain.Message{}
		if err := rows.Scan(
			&m.ID, &m.ChatID, &m.SenderID, &m.Message,
			&m.AttachmentFileID, &m.CreatedAt, &m.ReadAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan message row: %w", err)
		}
		messages = append(messages, m)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating message rows: %w", err)
	}

	return messages, total, nil
}

func (r *MessageRepo) MarkAsRead(ctx context.Context, chatID, userID uuid.UUID) error {
	const query = `
		UPDATE messages
		SET read_at = $3
		WHERE chat_id = $1
		  AND sender_id != $2
		  AND read_at IS NULL
	`

	_, err := r.db.ExecContext(ctx, query, chatID, userID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to mark messages as read: %w", err)
	}

	return nil
}

func (r *MessageRepo) GetUnreadCount(ctx context.Context, chatID, userID uuid.UUID) (int, error) {
	const query = `
		SELECT COUNT(*)
		FROM messages
		WHERE chat_id = $1
		  AND sender_id != $2
		  AND read_at IS NULL
	`

	var count int
	err := r.db.QueryRowxContext(ctx, query, chatID, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}

	return count, nil
}

func (r *MessageRepo) GetLastMessage(ctx context.Context, chatID uuid.UUID) (*domain.Message, error) {
	const query = `
		SELECT id, chat_id, sender_id, message, attachment_file_id, created_at, read_at
		FROM messages
		WHERE chat_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	m := &domain.Message{}
	err := r.db.QueryRowxContext(ctx, query, chatID).Scan(
		&m.ID, &m.ChatID, &m.SenderID, &m.Message,
		&m.AttachmentFileID, &m.CreatedAt, &m.ReadAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get last message: %w", err)
	}

	return m, nil
}

// GetLastMessages fetches the latest message for each of the given chat IDs
// in a single query, avoiding N+1 round-trips.
func (r *MessageRepo) GetLastMessages(ctx context.Context, chatIDs []uuid.UUID) (map[uuid.UUID]*domain.Message, error) {
	if len(chatIDs) == 0 {
		return map[uuid.UUID]*domain.Message{}, nil
	}

	query := `
		SELECT DISTINCT ON (chat_id) id, chat_id, sender_id, message, attachment_file_id, created_at, read_at
		FROM messages
		WHERE chat_id = ANY($1)
		ORDER BY chat_id, created_at DESC
	`

	rows, err := r.db.QueryxContext(ctx, query, pq.Array(chatIDs))
	if err != nil {
		return nil, fmt.Errorf("failed to get last messages: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]*domain.Message)
	for rows.Next() {
		m := &domain.Message{}
		if err := rows.Scan(
			&m.ID, &m.ChatID, &m.SenderID, &m.Message,
			&m.AttachmentFileID, &m.CreatedAt, &m.ReadAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan last message row: %w", err)
		}
		result[m.ChatID] = m
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating last messages: %w", err)
	}

	return result, nil
}

// GetUnreadCounts fetches unread message counts for multiple chat IDs
// in a single query, avoiding N+1 round-trips.
func (r *MessageRepo) GetUnreadCounts(ctx context.Context, chatIDs []uuid.UUID, userID uuid.UUID) (map[uuid.UUID]int, error) {
	if len(chatIDs) == 0 {
		return map[uuid.UUID]int{}, nil
	}

	query := `
		SELECT chat_id, COUNT(*)
		FROM messages
		WHERE chat_id = ANY($1)
		  AND sender_id != $2
		  AND read_at IS NULL
		GROUP BY chat_id
	`

	rows, err := r.db.QueryxContext(ctx, query, pq.Array(chatIDs), userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get unread counts: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]int)
	for rows.Next() {
		var chatID uuid.UUID
		var count int
		if err := rows.Scan(&chatID, &count); err != nil {
			return nil, fmt.Errorf("failed to scan unread count row: %w", err)
		}
		result[chatID] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating unread counts: %w", err)
	}

	return result, nil
}
