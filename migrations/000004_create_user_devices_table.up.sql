

CREATE TABLE IF NOT EXISTS user_devices (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  fingerprint_hash TEXT NOT NULL,
  device_label VARCHAR(255),
  first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revoked_at TIMESTAMPTZ,
  revoked_reason VARCHAR(255),
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

  UNIQUE(user_id, fingerprint_hash)
);

-- Khi bạn khai báo UNIQUE(user_id, fingerprint_hash), PostgreSQL đã tự động tạo một Composite Index (Index phức hợp) trên 2 cột này. 
-- Do user_id đứng ở vị trí đầu tiên (leading column) trong cấu trúc đó, mọi câu query tìm kiếm theo user_id đều được dùng luôn cái Index có sẵn này. 
-- Việc tạo thêm Index riêng cho user_id chỉ làm tốn dung lượng ổ cứng và làm chậm tốc độ INSERT/UPDATE.

CREATE TRIGGER trg_user_devices_update_updated_at
  BEFORE UPDATE ON user_devices
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at_column();