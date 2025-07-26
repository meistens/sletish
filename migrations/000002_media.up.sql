-- Create media table
CREATE TABLE IF NOT EXISTS media (
    id SERIAL PRIMARY KEY,
    external_id VARCHAR(255) UNIQUE NOT NULL,
    title VARCHAR(500) NOT NULL,
    type VARCHAR(50) NOT NULL DEFAULT 'anime',
    description TEXT,
    release_date VARCHAR(100),
    poster_url VARCHAR(1000),
    rating DECIMAL(3, 2),
    created_at TIMESTAMP
    WITH
        TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Add constraints for valid media types
ALTER TABLE media ADD CONSTRAINT check_media_type CHECK (type IN ('anime', 'movie', 'series', 'manga'));

-- Add constraints for valid rating values
ALTER TABLE media ADD CONSTRAINT check_media_rating CHECK (
    rating >= 0
    AND rating <= 10
);

-- Create indexes for media table
CREATE INDEX IF NOT EXISTS idx_media_external_id ON media (external_id);

CREATE INDEX IF NOT EXISTS idx_media_type ON media (type);

CREATE INDEX IF NOT EXISTS idx_media_title ON media (title);

CREATE INDEX IF NOT EXISTS idx_media_rating ON media (rating);

CREATE INDEX IF NOT EXISTS idx_media_created_at ON media (created_at);

-- Create full-text search index on media titles
CREATE INDEX IF NOT EXISTS idx_media_title_fts ON media USING gin (to_tsvector ('english', title));

-- Add comments for documentation
COMMENT ON TABLE media IS 'Stores anime, movie, and series information';

COMMENT ON COLUMN media.external_id IS 'External ID from source API (e.g., MyAnimeList ID)';

COMMENT ON COLUMN media.type IS 'Type of media (anime, movie, series, manga)';
