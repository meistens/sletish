-- Create reminders table
CREATE TABLE IF NOT EXISTS reminders (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    media_id INTEGER NOT NULL REFERENCES media (id) ON DELETE CASCADE,
    message TEXT NOT NULL,
    remind_at TIMESTAMP
    WITH
        TIME ZONE NOT NULL,
        sent BOOLEAN DEFAULT FALSE,
        created_at TIMESTAMP
    WITH
        TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for reminders table
CREATE INDEX IF NOT EXISTS idx_reminders_user_id ON reminders (user_id);

CREATE INDEX IF NOT EXISTS idx_reminders_media_id ON reminders (media_id);

CREATE INDEX IF NOT EXISTS idx_reminders_remind_at ON reminders (remind_at);

CREATE INDEX IF NOT EXISTS idx_reminders_sent ON reminders (sent);

CREATE INDEX IF NOT EXISTS idx_reminders_created_at ON reminders (created_at);

-- Create composite indexes for common queries
CREATE INDEX IF NOT EXISTS idx_reminders_user_sent ON reminders (user_id, sent);

CREATE INDEX IF NOT EXISTS idx_reminders_due ON reminders (remind_at, sent)
WHERE
    sent = false;

-- Create index for pending reminders
CREATE INDEX IF NOT EXISTS idx_reminders_pending ON reminders (remind_at)
WHERE
    sent = false;

-- Add comments for documentation
COMMENT ON TABLE reminders IS 'Stores user reminders for media';

COMMENT ON COLUMN reminders.remind_at IS 'When to send the reminder';

COMMENT ON COLUMN reminders.sent IS 'Whether the reminder has been sent';
