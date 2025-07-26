-- Drop triggers
DROP TRIGGER IF EXISTS update_user_media_updated_at ON user_media;

DROP TRIGGER IF EXISTS update_users_updated_at ON users;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column ();
