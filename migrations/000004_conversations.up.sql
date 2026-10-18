CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL
        REFERENCES organizations(id),
    user_id UUID NOT NULL REFERENCES users(id),
    title TEXT NOT NULL DEFAULT 'New conversation',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX conversations_owner_idx
    ON conversations (
        organization_id, user_id, updated_at DESC
    );

CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL
        REFERENCES conversations(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (
        role IN ('user', 'assistant')
    ),
    content TEXT NOT NULL,
    model TEXT,
    prompt_tokens INTEGER,
    completion_tokens INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX messages_conversation_created_idx
    ON messages (conversation_id, created_at);

CREATE TABLE citations (
    message_id UUID NOT NULL
        REFERENCES messages(id) ON DELETE CASCADE,
    chunk_id UUID NOT NULL
        REFERENCES document_chunks(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
    page_number INTEGER,
    excerpt TEXT,
    PRIMARY KEY (message_id, chunk_id),
    UNIQUE (message_id, ordinal)
);

CREATE TABLE usage_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL
        REFERENCES organizations(id),
    user_id UUID REFERENCES users(id),
    request_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    estimated_cost NUMERIC(14, 8),
    outcome TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX usage_org_created_idx
    ON usage_events (organization_id, created_at DESC);
