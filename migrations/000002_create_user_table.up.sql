


-- Users table (COMPLETE)
CREATE TABLE IF NOT EXISTS users (
  id BIGSERIAL PRIMARY KEY,
  email VARCHAR(255) NOT NULL UNIQUE,
  full_name VARCHAR(255) NOT NULL,
  phone VARCHAR(20),
  user_type VARCHAR(30) NOT NULL DEFAULT 'customer' CHECK (user_type IN ('customer', 'admin', 'operator')),
  account_status VARCHAR(30) NOT NULL DEFAULT 'pending_verification' CHECK (account_status IN ('pending_verification', 'active', 'locked', 'disabled')),
  email_verified_at TIMESTAMPTZ,
  last_login_at TIMESTAMPTZ,
  locked_reason VARCHAR(255),
  metadata JSONB DEFAULT '{}'::jsonb,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_users_account_status ON users(account_status);
CREATE INDEX idx_users_user_type ON users(user_type);
CREATE INDEX idx_users_email_verified_at ON users(email_verified_at);


-- Trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger
CREATE TRIGGER trg_users_update_updated_at
  BEFORE UPDATE ON users
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at_column();


-- Comments
COMMENT ON COLUMN users.user_type IS 'customer, admin';
COMMENT ON COLUMN users.account_status IS 'pending_verification, active, locked, disabled';
COMMENT ON COLUMN users.metadata IS 'Thông tin phụ trợ';
COMMENT ON COLUMN users.version IS 'Optimistic locking';



