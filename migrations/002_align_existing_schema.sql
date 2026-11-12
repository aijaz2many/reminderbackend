BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_role_type') THEN
        CREATE TYPE user_role_type AS ENUM ('super_admin', 'tenant_user');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'recurrence_type') THEN
        CREATE TYPE recurrence_type AS ENUM ('weekly', '15_days', 'monthly', 'yearly', 'custom');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'interval_unit_type') THEN
        CREATE TYPE interval_unit_type AS ENUM ('days', 'weeks', 'months', 'years');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'reminder_status_type') THEN
        CREATE TYPE reminder_status_type AS ENUM ('active', 'paused', 'completed');
    END IF;
END
$$;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS role user_role_type NOT NULL DEFAULT 'tenant_user';

UPDATE users SET subscription_tier = 'free' WHERE subscription_tier IS NULL;
ALTER TABLE users
    ALTER COLUMN subscription_tier SET DEFAULT 'free',
    ALTER COLUMN subscription_tier SET NOT NULL;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'reminders'
          AND column_name = 'recurrence_type'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'reminders'
          AND column_name = 'recurrence'
    ) THEN
        ALTER TABLE reminders RENAME COLUMN recurrence_type TO recurrence;
    END IF;
END
$$;

ALTER TABLE reminders
    ALTER COLUMN recurrence TYPE recurrence_type USING recurrence::text::recurrence_type,
    ALTER COLUMN custom_interval_unit TYPE interval_unit_type USING custom_interval_unit::text::interval_unit_type;

ALTER TABLE reminders ALTER COLUMN status DROP DEFAULT;
UPDATE reminders SET status = 'active' WHERE status IS NULL;
ALTER TABLE reminders
    ALTER COLUMN status TYPE reminder_status_type USING status::text::reminder_status_type,
    ALTER COLUMN status SET DEFAULT 'active',
    ALTER COLUMN status SET NOT NULL;

UPDATE reminders SET advance_notice_days = 0 WHERE advance_notice_days IS NULL;
ALTER TABLE reminders
    ALTER COLUMN advance_notice_days SET DEFAULT 0,
    ALTER COLUMN advance_notice_days SET NOT NULL;

CREATE INDEX IF NOT EXISTS reminders_user_due_idx ON reminders (user_id, next_due_date);

COMMIT;
