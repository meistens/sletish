-- Drop indexes
DROP INDEX IF EXISTS idx_user_media_watchlist;

DROP INDEX IF EXISTS idx_user_media_completed;

DROP INDEX IF EXISTS idx_user_media_watching;

DROP INDEX IF EXISTS idx_user_media_user_updated;

DROP INDEX IF EXISTS idx_user_media_user_status;

DROP INDEX IF EXISTS idx_user_media_created_at;

DROP INDEX IF EXISTS idx_user_media_updated_at;

DROP INDEX IF EXISTS idx_user_media_rating;

DROP INDEX IF EXISTS idx_user_media_status;

DROP INDEX IF EXISTS idx_user_media_media_id;

DROP INDEX IF EXISTS idx_user_media_user_id;

-- Drop constraints
ALTER TABLE user_media
DROP CONSTRAINT IF EXISTS check_user_media_rating;

ALTER TABLE user_media
DROP CONSTRAINT IF EXISTS check_user_media_status;

-- Drop table
DROP TABLE IF EXISTS user_media;
