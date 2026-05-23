package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/companyofcreators/chat-service/pkg/header_auth"
)

// WebSocketUpgrader is the function type for upgrading HTTP connections to WebSocket.
type WebSocketUpgrader func(w http.ResponseWriter, r *http.Request)

func NewRouter(handler *Handler, wsUpgrader WebSocketUpgrader, signer *header_auth.HeaderSigner, logger *slog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(bodySizeLimiter(500 << 10)) // 500KB
	r.Use(middleware.Heartbeat("/health"))
	r.Use(signer.VerifyMiddleware)

	// WebSocket endpoint
	r.Get("/ws", http.HandlerFunc(wsUpgrader))

	// Internal API
	r.Route("/internal", func(r chi.Router) {
		r.Get("/health", handler.Health)

		r.Route("/chats", func(r chi.Router) {
			r.Get("/", handler.ListChats)
			r.Post("/", handler.CreateChat)
			r.Get("/{id}", handler.GetChat)

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/messages", handler.GetMessages)
				r.Post("/messages", handler.SendMessage)
				r.Post("/read", handler.MarkRead)
			})
		})
	})

	return r
}

// bodySizeLimiter returns middleware that wraps http.MaxBytesReader to limit
// request body size and prevent memory exhaustion attacks.
func bodySizeLimiter(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
