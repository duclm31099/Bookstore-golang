
CREATE TABLE IF NOT EXISTS addresses (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  address_line1 VARCHAR(255) NOT NULL,
  address_line2 VARCHAR(255),
  province_code VARCHAR(20) NOT NULL,
  district_code VARCHAR(20) NOT NULL,
  ward_code VARCHAR(20) NOT NULL,
  postal_code VARCHAR(10),
  country_code VARCHAR(10) NOT NULL DEFAULT 'VN',
  is_default BOOLEAN NOT NULL DEFAULT false,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_addresses_user_id ON addresses(user_id);

CREATE UNIQUE INDEX idx_addresses_unique_default 
ON addresses(user_id) 
WHERE is_default = true;

CREATE TRIGGER trg_addresses_update_updated_at
  BEFORE UPDATE ON addresses
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at_column();

COMMENT ON COLUMN addresses.is_default IS 'Địa chỉ mặc định';
COMMENT ON COLUMN addresses.version IS 'Optimistic locking';
COMMENT ON COLUMN addresses.created_at IS 'Thời gian tạo';
COMMENT ON COLUMN addresses.updated_at IS 'Thời gian cập nhật';