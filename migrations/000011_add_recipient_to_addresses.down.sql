ALTER TABLE addresses
    DROP COLUMN IF EXISTS recipient_name,
    DROP COLUMN IF EXISTS recipient_phone;
