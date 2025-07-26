-- Drop indexes
DROP INDEX IF EXISTS idx_users_created_at;

DROP INDEX IF EXISTS idx_users_platform;

-- Drop constraints
ALTER TABLE users
DROP CONSTRAINT IF EXISTS check_users_platform;

-- Drop table
DROP TABLE IF EXISTS users;
