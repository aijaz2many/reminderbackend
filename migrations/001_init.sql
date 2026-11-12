CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE user_role_type AS ENUM ('super_admin', 'tenant_user');
CREATE TYPE recurrence_type AS ENUM ('weekly', '15_days', 'monthly', 'yearly', 'custom');
CREATE TYPE interval_unit_type AS ENUM ('days', 'weeks', 'months', 'years');
CREATE TYPE reminder_status_type AS ENUM ('active', 'paused', 'completed');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    phone_number VARCHAR(20) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(100),
    role user_role_type DEFAULT 'tenant_user' NOT NULL,
    subscription_tier VARCHAR(20) DEFAULT 'free',
    fcm_device_token TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE reminders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entity_name VARCHAR(255) NOT NULL,
    description TEXT,
    start_date DATE NOT NULL,
    next_due_date DATE NOT NULL,
    recurrence recurrence_type NOT NULL,
    custom_interval_value INT,
    custom_interval_unit interval_unit_type,
    advance_notice_days INT DEFAULT 0,
    status reminder_status_type DEFAULT 'active' NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT reminders_custom_interval_check CHECK (
        (recurrence = 'custom' AND custom_interval_value IS NOT NULL
            AND custom_interval_value BETWEEN 1 AND 1000 AND custom_interval_unit IS NOT NULL)
        OR (recurrence <> 'custom' AND custom_interval_value IS NULL AND custom_interval_unit IS NULL)
    ),
    CONSTRAINT reminders_advance_notice_check CHECK (advance_notice_days BETWEEN 0 AND 365)
);

CREATE INDEX reminders_user_due_idx ON reminders (user_id, next_due_date);

CREATE TABLE notification_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reminder_id UUID REFERENCES reminders(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    scheduled_for TIMESTAMPTZ NOT NULL,
    sent_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) DEFAULT 'sent',
    channel VARCHAR(20) DEFAULT 'push'
);
