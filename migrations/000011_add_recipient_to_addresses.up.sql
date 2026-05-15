-- Thêm recipient_name và recipient_phone vào addresses
-- Người nhận hàng có thể khác chủ tài khoản (mua hộ, quà tặng)
-- SRD v3.0 §13.1, ERD v3.0 §5.1.5

ALTER TABLE addresses
    ADD COLUMN recipient_name  VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN recipient_phone VARCHAR(20)  NOT NULL DEFAULT '';

-- Xóa default sau khi add column (application layer validate non-empty)
ALTER TABLE addresses
    ALTER COLUMN recipient_name  DROP DEFAULT,
    ALTER COLUMN recipient_phone DROP DEFAULT;

COMMENT ON COLUMN addresses.recipient_name  IS 'Tên người nhận hàng (có thể khác chủ tài khoản)';
COMMENT ON COLUMN addresses.recipient_phone IS 'SĐT người nhận hàng';
