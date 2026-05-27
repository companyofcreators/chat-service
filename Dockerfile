FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /chat-service ./cmd/api

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /chat-service .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8085

ENV HTTP_ADDRESS=:8085

CMD ["./chat-service"]
