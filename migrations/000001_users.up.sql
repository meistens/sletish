-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(255) PRIMARY KEY,
    username VARCHAR(255),
    platform VARCHAR(50) NOT NULL DEFAULT 'telegram',
    created_at TIMESTAMP
    WITH
        TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP
    WITH
        TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Add constraints for valid platforms
ALTER TABLE users ADD CONSTRAINT check_users_platform CHECK (platform IN ('telegram'));

-- Create indexes for users table
CREATE INDEX IF NOT EXISTS idx_users_platform ON users (platform);

CREATE INDEX IF NOT EXISTS idx_users_created_at ON users (created_at);

-- Add comments for documentation
COMMENT ON TABLE users IS 'Stores user information from various platforms';

COMMENT ON COLUMN users.id IS 'Platform-specific user ID (e.g., Telegram user ID)';

COMMENT ON COLUMN users.platform IS 'Platform where user is from (telegram, discord, web)';
