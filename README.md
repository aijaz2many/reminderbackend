# Reminder API

A Go 1.23 REST API built with Gin and PostgreSQL. Each authenticated tenant can access only their own reminders.

## Run locally

1. Create a PostgreSQL database and apply the schema migrations in order. For a new database, apply `migrations/001_init.sql`. If the database already has the earlier schema with a `reminders.recurrence_type` column, apply `migrations/002_align_existing_schema.sql` as well. For an existing database, apply `migrations/003_widen_notification_status.sql` and `migrations/004_fcm_devices_and_scheduling.sql`. Migration 004 is idempotent for databases where the design changes were already applied.
2. Copy `.env.example` to `.env` and set `DATABASE_URL` to the PostgreSQL connection string and `JWT_SECRET` to a random secret of at least 32 bytes. Keep `.env` private; it is ignored by Git.
3. Optionally set `PORT` (default `8080`), `JWT_TTL` (default `24h`), and `APP_ENV=production` in `.env`.
4. Run `go run ./cmd/api` from the backend directory. The API loads `.env` automatically; variables already set in the process environment take precedence.

The API listens on `:8080` by default. `GET /healthz` checks database connectivity.
For browser clients, set `CORS_ALLOWED_ORIGINS` to a comma-separated list of trusted web app origins. In development, HTTP origins on `localhost`, `127.0.0.1`, and `::1` are allowed automatically; production requires an explicit allowlist.
Swagger UI is available at `http://localhost:8080/swagger/index.html`; the OpenAPI 2.0 document is at `http://localhost:8080/swagger/doc.json`.

## Push notification worker

The background worker sends **push notifications to active FCM device registrations only**; it does not send SMS or email. Set `DATABASE_URL` and `FIREBASE_CREDENTIALS` (a path to a Firebase service-account JSON file with Firebase Cloud Messaging permission) in `.env`, then run `go run ./cmd/worker` from the backend directory. It processes due reminders on startup and checks for scheduled reminders every minute. Reminders use their configured `notification_time` and IANA `timezone`. Each device attempt is recorded with its provider message ID or error; failed sends retry up to three times, at least 15 minutes apart.

## API

All application endpoints are under `/api/v1`. Protected endpoints require `Authorization: Bearer <token>`.

| Method | Path | Purpose |
|---|---|---|
| POST | `/auth/register` | Register a tenant using `email`, E.164 `phone_number`, `password`, and optional `full_name` |
| POST | `/auth/login` | Exchange email and password for a JWT |
| GET | `/me` | Get the authenticated user's profile |
| GET | `/me/devices` | List the authenticated user's registered devices |
| POST | `/me/devices` | Register or reactivate a device using `device_token`, `platform`, and optional `device_name` |
| PATCH | `/me/devices/:id` | Activate or pause one of the authenticated user's devices |
| POST | `/reminders` | Create a reminder |
| GET | `/reminders?limit=50&offset=0` | List the authenticated user's reminders |
| GET | `/notification-logs?limit=50&offset=0` | List the authenticated user's notification logs |
| GET | `/reminders/:id` | Get one of the authenticated user's reminders |
| PUT | `/reminders/:id` | Replace reminder details |
| PATCH | `/reminders/:id/status` | Set status to `active`, `paused`, or `completed` |
| DELETE | `/reminders/:id` | Delete a reminder |
| GET | `/admin/analytics` | Get platform usage counts (super admin only) |
| GET | `/admin/users?limit=50&offset=0` | List users (super admin only) |
| PATCH | `/admin/users/:id/subscription` | Set a user's `subscription_tier` (super admin only) |
| PATCH | `/admin/users/:id/role` | Set a user's role to `tenant_user` or `super_admin` (super admin only) |

Reminder creation and replacement accept `entity_name`, `description`, `start_date` (`YYYY-MM-DD`), `recurrence`, optional `advance_notice_days`, and `notification_time` (`HH:MM`) with an IANA `timezone` (defaults to `09:00` and `Asia/Kolkata`). Recurrences are `weekly`, `15_days`, `monthly`, `yearly`, or `custom`. Custom recurrences also require `custom_interval_value` and `custom_interval_unit` (`days`, `weeks`, `months`, or `years`). Monthly and yearly dates that do not exist in the target month are clamped to its last day.

Registration creates `tenant_user` accounts only. `super_admin` accounts must be provisioned by a trusted operator directly in the database; public registration cannot assign privileged roles.

To bootstrap the first administrator, register that account normally, then run `UPDATE users SET role = 'super_admin' WHERE email = 'admin@example.com';` using a trusted database session. Device registrations are stored separately from users so an account can use multiple devices. SMS delivery is not implemented.
