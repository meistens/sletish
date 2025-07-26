-- Drop indexes
DROP INDEX IF EXISTS idx_reminders_pending;

DROP INDEX IF EXISTS idx_reminders_due;

DROP INDEX IF EXISTS idx_reminders_user_sent;

DROP INDEX IF EXISTS idx_reminders_created_at;

DROP INDEX IF EXISTS idx_reminders_sent;

DROP INDEX IF EXISTS idx_reminders_remind_at;

DROP INDEX IF EXISTS idx_reminders_media_id;

DROP INDEX IF EXISTS idx_reminders_user_id;

-- Drop table
DROP TABLE IF EXISTS reminders;
