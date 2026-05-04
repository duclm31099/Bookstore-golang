-- Bước 1: Tạo bảng gốc (Parent Table) có PARTITION BY RANGE
CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGSERIAL,
    actor_type VARCHAR(30) NOT NULL,
    actor_id VARCHAR(255),       -- Đổi thành VARCHAR để bọc được cả UUID
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id VARCHAR(255),    -- Đổi thành VARCHAR
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, created_at) -- Trong Partitioning, Partition key phải nằm trong PK
) PARTITION BY RANGE (created_at);

-- Bước 2: Tạo các phân vùng (Ví dụ tạo sẵn cho vài tháng)
CREATE TABLE audit_logs_y2026m05 PARTITION OF audit_logs 
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');

CREATE TABLE audit_logs_y2026m06 PARTITION OF audit_logs 
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');

-- Bước 3: Đánh Index trên Parent Table (Nó sẽ tự động áp dụng xuống các Partition)
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource
    ON audit_logs (resource_type, resource_id);

CREATE INDEX IF NOT EXISTS idx_audit_logs_actor
    ON audit_logs (actor_type, actor_id);

CREATE INDEX IF NOT EXISTS idx_audit_logs_action_created_at
    ON audit_logs (action, created_at);