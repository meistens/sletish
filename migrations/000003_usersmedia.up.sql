-- Create user media tracking table
CREATE TABLE IF NOT EXISTS user_media (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    media_id INTEGER NOT NULL REFERENCES media (id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL,
    rating DECIMAL(3, 2),
    notes TEXT,
    created_at TIMESTAMP
    WITH
        TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP
    WITH
        TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        UNIQUE (user_id, media_id)
);

-- Add constraints for valid status values
ALTER TABLE user_media ADD CONSTRAINT check_user_media_status CHECK (
    status IN (
        'watching',
        'completed',
        'on_hold',
        'dropped',
        'watchlist'
    )
);

-- Add constraints for valid rating values
ALTER TABLE user_media ADD CONSTRAINT check_user_media_rating CHECK (
    rating >= 0
    AND rating <= 10
);

-- Create indexes for user_media table
CREATE INDEX IF NOT EXISTS idx_user_media_user_id ON user_media (user_id);

CREATE INDEX IF NOT EXISTS idx_user_media_media_id ON user_media (media_id);

CREATE INDEX IF NOT EXISTS idx_user_media_status ON user_media (status);

CREATE INDEX IF NOT EXISTS idx_user_media_rating ON user_media (rating);

CREATE INDEX IF NOT EXISTS idx_user_media_updated_at ON user_media (updated_at);

CREATE INDEX IF NOT EXISTS idx_user_media_created_at ON user_media (created_at);

-- Create composite indexes for common queries
CREATE INDEX IF NOT EXISTS idx_user_media_user_status ON user_media (user_id, status);

CREATE INDEX IF NOT EXISTS idx_user_media_user_updated ON user_media (user_id, updated_at DESC);

-- Create partial indexes for common queries
CREATE INDEX IF NOT EXISTS idx_user_media_watching ON user_media (user_id, updated_at DESC)
WHERE
    status = 'watching';

CREATE INDEX IF NOT EXISTS idx_user_media_completed ON user_media (user_id, updated_at DESC)
WHERE
    status = 'completed';

CREATE INDEX IF NOT EXISTS idx_user_media_watchlist ON user_media (user_id, updated_at DESC)
WHERE
    status = 'watchlist';

-- Add comments for documentation
COMMENT ON TABLE user_media IS 'Tracks user''s media consumption and ratings';

COMMENT ON COLUMN user_media.status IS 'User''s watching status (watching, completed, on_hold, dropped, watchlist)';

COMMENT ON COLUMN user_media.rating IS 'User''s rating (0-10)';
