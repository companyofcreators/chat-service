# Chat Service

Real-time messaging between masters and customers. Chats are created only after an offer is accepted.

## Architecture

- **WebSocket**: `github.com/coder/websocket` for real-time communication
- **Redis Pub/Sub**: Multi-instance message fanout
- **PostgreSQL**: Chat and message persistence
- **Kafka**: `chat.message.sent` events for notifications

## Tech Stack

| Component    | Technology                     |
|-------------|-------------------------------|
| Language    | Go 1.22                       |
| HTTP Router | chi/v5                        |
| WebSocket   | coder/websocket               |
| Database    | PostgreSQL (pgx/v5)           |
| Cache/PubSub| Redis (go-redis/v9)           |
| Messaging   | Kafka (segmentio/kafka-go)    |
| Auth        | golang-jwt/jwt/v5             |
| Config      | cleanenv                      |
| Logging     | zap                           |

## Quick Start

```bash
cp .env.example .env
# Edit .env with your database, Redis, and Kafka credentials
go mod tidy
go run ./cmd/api
```

## Environment Variables

| Variable        | Default                | Description                    |
|----------------|------------------------|--------------------------------|
| HTTP_ADDRESS   | :8085                  | HTTP server listen address     |
| DB_DSN         | *required*             | PostgreSQL connection string   |
| REDIS_ADDR     | localhost:6379         | Redis server address           |
| REDIS_PASSWORD |                        | Redis password                |
| KAFKA_BROKERS  | localhost:9092         | Kafka broker addresses         |
| JWT_SECRET     |                        | JWT signing secret            |
| LOG_LEVEL      | info                   | Logging level                  |

## WebSocket Protocol

Connect: `ws://host:8085/ws?token=<jwt_access_token>`

### Client to Server

```json
{"type": "message.send", "chat_id": "uuid", "message": "Hello!"}
{"type": "typing.start", "chat_id": "uuid"}
{"type": "typing.stop", "chat_id": "uuid"}
{"type": "messages.read", "chat_id": "uuid"}
{"type": "ping"}
```

### Server to Client

```json
{"type": "message.new", "chat_id": "uuid", "message": {...}}
{"type": "typing", "chat_id": "uuid", "user_id": "uuid", "is_typing": true}
{"type": "messages.read", "chat_id": "uuid", "user_id": "uuid"}
{"type": "pong"}
{"type": "error", "message": "..."}
```

## HTTP API

All endpoints require `X-User-ID` header.

| Method | Path                              | Description                |
|--------|----------------------------------|----------------------------|
| GET    | /internal/chats                  | List user's chats          |
| POST   | /internal/chats                  | Create chat                |
| GET    | /internal/chats/{id}             | Get chat detail            |
| GET    | /internal/chats/{id}/messages    | Get messages (paginated)   |
| POST   | /internal/chats/{id}/messages    | Send message (REST fallback)|
| POST   | /internal/chats/{id}/read        | Mark messages as read      |
| GET    | /ws                              | WebSocket upgrade          |
| GET    | /internal/health                 | Health check               |

## Kafka Events

**Topic**: `chat.message.sent`

```json
{
    "message_id": "uuid",
    "chat_id": "uuid",
    "sender_id": "uuid",
    "receiver_id": "uuid",
    "message_preview": "text",
    "timestamp": "2024-01-01T00:00:00Z"
}
```

## Redis Pub/Sub Channels

| Channel         | Purpose                           |
|----------------|-----------------------------------|
| chat:messages  | Message fanout across instances   |
| chat:typing    | Typing indicator fanout           |
| chat:presence  | Online/offline status             |

## Directory Structure

```
chat-service/
├── cmd/api/main.go              # Entry point
├── internal/
│   ├── app/container.go         # DI container
│   ├── config/config.go         # Configuration
│   ├── domain/chat/             # Domain entities, repository interfaces, errors
│   ├── application/chat/        # Use cases (create, send, get, mark read, list)
│   ├── infrastructure/          # PostgreSQL, Redis, WebSocket hub, Kafka
│   ├── interfaces/              # HTTP handlers, WebSocket handler
│   └── pkg/logger.go            # Logger utility
├── migrations/001_chats.up.sql  # Database schema
├── Dockerfile
├── .env.example
├── go.mod
└── README.md
```
