
### **System Prompt: Single-User Multi-Tenant SaaS Recurring Reminder App (Go Edition)**

**Core Objective**
Build a multi-tenant SaaS mobile application (iOS/Android) using **React Native** for the frontend, **Go (Golang)** for the high-performance backend API and background workers, and **PostgreSQL** for data persistence.

---

### **1. Architecture & Tech Stack**

* **Mobile Frontend:** React Native (Expo or CLI) with push notification handlers (`expo-notifications` or `react-native-firebase`).
* **Backend Framework:** Go (Golang) using **Gin**  for RESTful API routing, or Go's built-in `net/http` router (Go 1.22+).
* **Database Driver & ORM:** PostgreSQL with custom `ENUM` types, accessed via `pgx` (native PostgreSQL driver) or `GORM`.
* **Task Scheduling & Background Workers:** Go routines combined with `robfig/cron` for processing daily reminder queues and notifications.
* **Notifications:** Firebase Cloud Messaging (FCM) for push notifications; 

---

### **2. User Roles & Multi-Tenancy Strategy**

* **Super Admin (`super_admin`):** Platform owner managing global subscription tiers, platform usage analytics, and user account overrides.
* **Tenant / End User (`tenant_user`):** Individual registered user operating as an isolated tenant with zero team-member sub-accounts.

---

### **3. PostgreSQL Database Schema**

```sql
-- 1. Custom PostgreSQL ENUM Types
CREATE TYPE user_role_type AS ENUM ('super_admin', 'tenant_user');
CREATE TYPE recurrence_type AS ENUM ('weekly', '15_days', 'monthly', 'yearly', 'custom');
CREATE TYPE interval_unit_type AS ENUM ('days', 'weeks', 'months', 'years');
CREATE TYPE reminder_status_type AS ENUM ('active', 'paused', 'completed');

-- 2. Users (Tenants) Table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    phone_number VARCHAR(20) UNIQUE NOT NULL, -- E.164 format (e.g., +919876543210)
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(100),
    role user_role_type DEFAULT 'tenant_user' NOT NULL,
    subscription_tier VARCHAR(20) DEFAULT 'free',
    fcm_device_token TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- 3. Reminders Table
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
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- 4. Notification Logs Table
CREATE TABLE notification_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reminder_id UUID REFERENCES reminders(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    scheduled_for TIMESTAMPTZ NOT NULL,
    sent_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(20) DEFAULT 'sent',
    channel VARCHAR(20) DEFAULT 'push'
);

```

---

