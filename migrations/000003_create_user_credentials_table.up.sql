
CREATE TABLE IF NOT EXISTS user_credentials (
  user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  password_hash TEXT NOT NULL,
  password_algo VARCHAR(30) NOT NULL DEFAULT 'argon2id',
  password_changed_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  failed_login_count INT NOT NULL DEFAULT 0 CHECK (failed_login_count >= 0),
  last_failed_login_at TIMESTAMPTZ
);

