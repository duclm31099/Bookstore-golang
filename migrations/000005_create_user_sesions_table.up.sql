

CREATE TABLE IF NOT EXISTS user_sessions (
  id BIGSERIAL PRIMARY KEY,
  -- 1. Ràng buộc cứng: Session phải thuộc về User và Device
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id BIGINT NOT NULL REFERENCES user_devices(id) ON DELETE CASCADE,

  refresh_token_hash TEXT NOT NULL,
  session_status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (session_status IN ('active', 'revoked', 'expired')),
  expires_at TIMESTAMPTZ NOT NULL,
  ip_address VARCHAR(45),
  user_agent TEXT,
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  revoked_at TIMESTAMPTZ,
  revoked_reason VARCHAR(255),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  -- 2. Chốt chặn bảo mật & Tra cứu siêu tốc độ cho API /refresh-token
  UNIQUE(refresh_token_hash),
  -- 3. Phục vụ Upsert: Đảm bảo 1 thiết bị của 1 user chỉ duy trì đúng 1 Session
  UNIQUE(user_id, device_id)
);

CREATE INDEX idx_user_sessions_id_status ON user_sessions(user_id, session_status);
CREATE INDEX idx_user_sessions_expires_at ON user_sessions(expires_at);

CREATE TRIGGER trg_user_sessions_update_updated_at
  BEFORE UPDATE ON user_sessions
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at_column();
  


-- Tự động dọn dẹp sạch sẽ (No Orphans): 
-- Do có cờ ON DELETE CASCADE ở cả 2 cột user_id và device_id, 
-- nếu người dùng bấm "Xóa tài khoản" HOẶC bấm "Đăng xuất khỏi thiết bị này" 
-- (xóa dòng trong bảng user_devices), PostgreSQL sẽ ngay lập tức tự động quét và xóa luôn dòng session tương ứng. 
-- Backend không cần viết thêm lệnh DELETE session nào cả, loại bỏ hoàn toàn nguy cơ rác database.