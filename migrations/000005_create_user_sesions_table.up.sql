

CREATE TABLE IF NOT EXISTS user_sessions (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id BIGINT REFERENCES user_devices(id) ON DELETE SET NULL,
  refresh_token_hash TEXT NOT NULL,
  session_status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (session_status IN ('active', 'revoked', 'expired')),
  expires_at TIMESTAMPTZ NOT NULL,
  ip_address INET,
  user_agent TEXT,
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  revoked_at TIMESTAMPTZ,
  revoked_reason VARCHAR(255),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE(refresh_token_hash)
);

CREATE INDEX idx_user_sessions_id_status ON user_sessions(user_id, session_status);
CREATE INDEX idx_user_sessions_expires_at ON user_sessions(expires_at);

CREATE TRIGGER trg_user_sessions_update_updated_at
  BEFORE UPDATE ON user_sessions
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at_column();