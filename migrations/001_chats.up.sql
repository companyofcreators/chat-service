-- Enable uuid-ossp extension for UUID generation.
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Chats table: one chat per order (created when an offer is accepted).
CREATE TABLE IF NOT EXISTS chats (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id UUID NOT NULL UNIQUE,
    customer_id UUID NOT NULL,
    master_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Messages table: all messages within a chat.
CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    sender_id UUID NOT NULL,
    message TEXT NOT NULL,
    attachment_file_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    read_at TIMESTAMPTZ
);

-- Indexes for query performance.
CREATE INDEX IF NOT EXISTS idx_messages_chat_id ON messages(chat_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_chats_customer ON chats(customer_id);
CREATE INDEX IF NOT EXISTS idx_chats_master ON chats(master_id);
