package chat

import "errors"

var (
	ErrChatNotFound      = errors.New("чат не найден")
	ErrChatAlreadyExists = errors.New("чат для этого заказа уже существует")
	ErrNotParticipant    = errors.New("пользователь не является участником этого чата")
	ErrEmptyMessage      = errors.New("сообщение не может быть пустым")
	ErrInvalidChatID     = errors.New("недействительный ID чата")
	ErrInvalidOrderID    = errors.New("недействительный ID заказа")
	ErrInvalidUserID     = errors.New("недействительный ID пользователя")
)
