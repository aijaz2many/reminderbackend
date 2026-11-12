CREATE TABLE IF NOT EXISTS user_devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_token TEXT NOT NULL UNIQUE,
    platform VARCHAR(20) NOT NULL,
    device_name VARCHAR(150),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_user_devices_user_active
    ON user_devices(user_id, is_active);

ALTER TABLE notification_logs
    ALTER COLUMN status TYPE VARCHAR(50),
    ALTER COLUMN status SET DEFAULT 'pending',
    ALTER COLUMN channel TYPE VARCHAR(50),
    ALTER COLUMN channel SET DEFAULT 'push',
    ALTER COLUMN sent_at DROP DEFAULT,
    ADD COLUMN IF NOT EXISTS device_id UUID,
    ADD COLUMN IF NOT EXISTS provider VARCHAR(30) DEFAULT 'firebase',
    ADD COLUMN IF NOT EXISTS provider_message_id TEXT,
    ADD COLUMN IF NOT EXISTS error_message TEXT,
    ADD COLUMN IF NOT EXISTS retry_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP;

UPDATE notification_logs SET status = 'pending' WHERE status IS NULL;
UPDATE notification_logs SET channel = 'push' WHERE channel IS NULL;
ALTER TABLE notification_logs
    ALTER COLUMN status SET NOT NULL,
    ALTER COLUMN channel SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_notification_logs_device'
    ) THEN
        ALTER TABLE notification_logs
            ADD CONSTRAINT fk_notification_logs_device
            FOREIGN KEY (device_id) REFERENCES user_devices(id) ON DELETE SET NULL;
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_notification_logs_reminder
    ON notification_logs(reminder_id);
CREATE INDEX IF NOT EXISTS idx_notification_logs_user
    ON notification_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_notification_logs_status_scheduled
    ON notification_logs(status, scheduled_for);
CREATE INDEX IF NOT EXISTS idx_notification_logs_device_schedule
    ON notification_logs(reminder_id, device_id, scheduled_for, created_at DESC);

ALTER TABLE reminders
    ADD COLUMN IF NOT EXISTS notification_time TIME NOT NULL DEFAULT '09:00:00',
    ADD COLUMN IF NOT EXISTS timezone VARCHAR(50) NOT NULL DEFAULT 'Asia/Kolkata',
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP;
