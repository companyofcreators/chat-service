package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"github.com/google/uuid"
	"strconv"

	"github.com/go-chi/chi/v5"

	appChat "github.com/companyofcreators/chat-service/internal/application/chat"
	domain "github.com/companyofcreators/chat-service/internal/domain/chat"
)

type Handler struct {
	createChat  *appChat.CreateChatUseCase
	sendMessage *appChat.SendMessageUseCase
	getMessages *appChat.GetMessagesUseCase
	markRead    *appChat.MarkReadUseCase
	listChats   *appChat.ListChatsUseCase
	chatRepo    domain.ChatRepository
	logger      *slog.Logger
}

func NewHandler(
	createChat *appChat.CreateChatUseCase,
	sendMessage *appChat.SendMessageUseCase,
	getMessages *appChat.GetMessagesUseCase,
	markRead *appChat.MarkReadUseCase,
	listChats *appChat.ListChatsUseCase,
	chatRepo domain.ChatRepository,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		createChat:  createChat,
		sendMessage: sendMessage,
		getMessages: getMessages,
		markRead:    markRead,
		listChats:   listChats,
		chatRepo:    chatRepo,
		logger:      logger,
	}
}

// CreateChat handles POST /internal/chats
func (h *Handler) CreateChat(w http.ResponseWriter, r *http.Request) {
	var req CreateChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "некорректное тело запроса")
		return
	}

	orderID, err := parseUUID(req.OrderID)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "недействительный order_id")
		return
	}

	customerID, err := parseUUID(req.CustomerID)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "недействительный customer_id")
		return
	}

	masterID, err := parseUUID(req.MasterID)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "недействительный master_id")
		return
	}

	// Authorization: only a participant (customer or master) can create the chat.
	userIDStr := r.Header.Get("X-User-ID")
	if userIDStr == "" {
		h.writeError(w, http.StatusUnauthorized, "отсутствует заголовок X-User-ID")
		return
	}
	callerID, err := parseUUID(userIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "недействительный X-User-ID")
		return
	}
	if callerID != customerID && callerID != masterID {
		h.writeError(w, http.StatusForbidden, "только участник может создать этот чат")
		return
	}

	input := appChat.CreateChatInput{
		OrderID:    orderID,
		CustomerID: customerID,
		MasterID:   masterID,
	}

	chat, err := h.createChat.Execute(r.Context(), input)
	if err != nil {
		if errors.Is(err, domain.ErrChatAlreadyExists) {
			h.writeError(w, http.StatusConflict, "чат для этого заказа уже существует")
			return
		}
		h.logger.Error("не удалось создать чат", "error", err)
		h.writeError(w, http.StatusInternalServerError, "не удалось создать чат")
		return
	}

	h.writeJSON(w, http.StatusCreated, toChatResponse(chat))
}

// GetChat handles GET /internal/chats/{id}
func (h *Handler) GetChat(w http.ResponseWriter, r *http.Request) {
	chatID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "недействительный chat_id")
		return
	}

	chat, err := h.chatRepo.FindByID(r.Context(), chatID)
	if err != nil {
		if errors.Is(err, domain.ErrChatNotFound) {
			h.writeError(w, http.StatusNotFound, "чат не найден")
			return
		}
		h.logger.Error("не удалось найти чат", "error", err)
		h.writeError(w, http.StatusInternalServerError, "не удалось найти чат")
		return
	}

	h.writeJSON(w, http.StatusOK, toChatResponse(chat))
}

// ListChats handles GET /internal/chats
func (h *Handler) ListChats(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.Header.Get("X-User-ID")
	if userIDStr == "" {
		h.writeError(w, http.StatusUnauthorized, "отсутствует заголовок X-User-ID")
		return
	}

	userID, err := parseUUID(userIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "недействительный X-User-ID")
		return
	}

	limit, offset := parsePagination(r)

	orderIDStr := r.URL.Query().Get("order_id")
	var orderID *uuid.UUID
	if orderIDStr != "" {
		id, err := uuid.Parse(orderIDStr)
		if err == nil {
			orderID = &id
		}
	}

	input := appChat.ListChatsInput{
		UserID:  userID,
		OrderID: orderID,
		Limit:   limit,
		Offset:  offset,
	}

	output, err := h.listChats.Execute(r.Context(), input)
	if err != nil {
		h.logger.Error("не удалось получить список чатов", "error", err)
		h.writeError(w, http.StatusInternalServerError, "не удалось получить список чатов")
		return
	}

	resp := ChatListResponse{
		Chats: make([]*ChatWithPreviewResponse, 0, len(output.Chats)),
		Total: output.Total,
	}
	for _, c := range output.Chats {
		cr := toChatWithPreviewResponse(c)
		resp.Chats = append(resp.Chats, &cr)
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// GetMessages handles GET /internal/chats/{id}/messages
func (h *Handler) GetMessages(w http.ResponseWriter, r *http.Request) {
	chatID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "недействительный chat_id")
		return
	}

	userIDStr := r.Header.Get("X-User-ID")
	if userIDStr == "" {
		h.writeError(w, http.StatusUnauthorized, "отсутствует заголовок X-User-ID")
		return
	}

	userID, err := parseUUID(userIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "недействительный X-User-ID")
		return
	}

	limit, offset := parsePagination(r)

	input := appChat.GetMessagesInput{
		ChatID: chatID,
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	}

	output, err := h.getMessages.Execute(r.Context(), input)
	if err != nil {
		if errors.Is(err, domain.ErrNotParticipant) {
			h.writeError(w, http.StatusForbidden, "не участник этого чата")
			return
		}
		h.logger.Error("не удалось получить сообщения", "error", err)
		h.writeError(w, http.StatusInternalServerError, "не удалось получить сообщения")
		return
	}

	resp := MessageListResponse{
		Messages: make([]*MessageResponse, 0, len(output.Messages)),
		Total:    output.Total,
	}
	for _, m := range output.Messages {
		mr := toMessageResponse(m)
		resp.Messages = append(resp.Messages, &mr)
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// SendMessage handles POST /internal/chats/{id}/messages (REST fallback)
func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request) {
	chatID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "недействительный chat_id")
		return
	}

	userIDStr := r.Header.Get("X-User-ID")
	if userIDStr == "" {
		h.writeError(w, http.StatusUnauthorized, "отсутствует заголовок X-User-ID")
		return
	}

	senderID, err := parseUUID(userIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "недействительный X-User-ID")
		return
	}

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "некорректное тело запроса")
		return
	}

	input := appChat.SendMessageInput{
		ChatID:   chatID,
		SenderID: senderID,
		Message:  req.Message,
	}

	if req.AttachmentFileID != nil {
		id, err := parseUUID(*req.AttachmentFileID)
		if err == nil {
			input.AttachmentFileID = &id
		}
	}

	msg, _, err := h.sendMessage.Execute(r.Context(), input)
	if err != nil {
		if errors.Is(err, domain.ErrNotParticipant) {
			h.writeError(w, http.StatusForbidden, "не участник этого чата")
			return
		}
		if errors.Is(err, domain.ErrEmptyMessage) {
			h.writeError(w, http.StatusBadRequest, "сообщение не может быть пустым")
			return
		}
		h.logger.Error("не удалось отправить сообщение", "error", err)
		h.writeError(w, http.StatusInternalServerError, "не удалось отправить сообщение")
		return
	}

	h.writeJSON(w, http.StatusCreated, toMessageResponse(msg))
}

// MarkRead handles POST /internal/chats/{id}/read (REST fallback)
func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
	chatID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "недействительный chat_id")
		return
	}

	userIDStr := r.Header.Get("X-User-ID")
	if userIDStr == "" {
		h.writeError(w, http.StatusUnauthorized, "отсутствует заголовок X-User-ID")
		return
	}

	userID, err := parseUUID(userIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "недействительный X-User-ID")
		return
	}

	input := appChat.MarkReadInput{
		ChatID: chatID,
		UserID: userID,
	}

	if err := h.markRead.Execute(r.Context(), input); err != nil {
		if errors.Is(err, domain.ErrNotParticipant) {
			h.writeError(w, http.StatusForbidden, "не участник этого чата")
			return
		}
		h.logger.Error("не удалось отметить как прочитанное", "error", err)
		h.writeError(w, http.StatusInternalServerError, "не удалось отметить как прочитанное")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Health handles GET /internal/health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "chat-service",
	})
}

// parsePagination extracts limit and offset from query parameters.
func parsePagination(r *http.Request) (int, int) {
	limit := 50
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	return limit, offset
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response", "error", err)
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, ErrorResponse{
		Error:   statusText(status),
		Message: message,
	})
}

func statusText(code int) string {
	switch code {
	case http.StatusBadRequest:
		return "некорректный запрос"
	case http.StatusUnauthorized:
		return "не авторизован"
	case http.StatusForbidden:
		return "доступ запрещён"
	case http.StatusNotFound:
		return "не найдено"
	case http.StatusConflict:
		return "конфликт"
	case http.StatusUnprocessableEntity:
		return "ошибка валидации"
	case http.StatusTooManyRequests:
		return "слишком много запросов"
	case http.StatusInternalServerError:
		return "внутренняя ошибка сервера"
	default:
		return "ошибка"
	}
}
