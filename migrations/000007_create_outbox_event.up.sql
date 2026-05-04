CREATE TABLE IF NOT EXISTS outbox_events (
    id BIGSERIAL PRIMARY KEY,
    topic VARCHAR(100) NOT NULL,
    event_key VARCHAR(255) NOT NULL,
    aggregate_type VARCHAR(50) NOT NULL,
    aggregate_id VARCHAR(255), -- Sửa thành VARCHAR để bọc được cả UUID
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    metadata JSONB, -- MỚI: Dành cho TraceID, CorrelationID, Headers...
    state VARCHAR(30) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    last_error TEXT,
    CHECK (state IN ('pending', 'published', 'failed'))
);

-- Chỉ giữ lại Index cốt lõi phục vụ Polling
CREATE INDEX IF NOT EXISTS idx_outbox_events_state_created_at
    ON outbox_events (state, created_at);