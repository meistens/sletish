-- Drop indexes
DROP INDEX IF EXISTS idx_media_title_fts;

DROP INDEX IF EXISTS idx_media_created_at;

DROP INDEX IF EXISTS idx_media_rating;

DROP INDEX IF EXISTS idx_media_title;

DROP INDEX IF EXISTS idx_media_type;

DROP INDEX IF EXISTS idx_media_external_id;

-- Drop constraints
ALTER TABLE media
DROP CONSTRAINT IF EXISTS check_media_rating;

ALTER TABLE media
DROP CONSTRAINT IF EXISTS check_media_type;

-- Drop table
DROP TABLE IF EXISTS media;
