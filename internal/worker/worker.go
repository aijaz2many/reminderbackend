package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"

	"github.com/reminderapp/reminderbackend/internal/push"
)

type reminder struct {
	ID                  uuid.UUID
	UserID              uuid.UUID
	EntityName          string
	Description         *string
	NextDueDate         time.Time
	AdvanceNoticeDays   int
	NotificationTime    string
	Timezone            string
	Recurrence          string
	CustomIntervalValue *int
	CustomIntervalUnit  *string
}

type device struct {
	ID    uuid.UUID
	Token string
}

func RunDaily(ctx context.Context, db *pgxpool.Pool, fcm *push.Client) error {
	if db == nil || fcm == nil {
		return errors.New("database and Firebase client are required")
	}
	if err := ProcessDueReminders(ctx, db, fcm, time.Now().UTC()); err != nil {
		return err
	}
	scheduler := cron.New(cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)))
	if _, err := scheduler.AddFunc("* * * * *", func() {
		if err := ProcessDueReminders(ctx, db, fcm, time.Now().UTC()); err != nil {
			log.Printf("daily reminder job failed: %v", err)
		}
	}); err != nil {
		return fmt.Errorf("schedule daily reminder job: %w", err)
	}
	scheduler.Start()
	<-ctx.Done()
	<-scheduler.Stop().Done()
	return ctx.Err()
}

func ProcessDueReminders(ctx context.Context, db *pgxpool.Pool, fcm *push.Client, now time.Time) error {
	rows, err := db.Query(ctx, `
		SELECT id
		FROM reminders
		WHERE status = 'active'
		  AND ((next_due_date - advance_notice_days)::timestamp + notification_time)
		      AT TIME ZONE timezone <= $1
		ORDER BY next_due_date, id`, now.UTC())
	if err != nil {
		return fmt.Errorf("query due reminders: %w", err)
	}
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("read due reminder ID: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate due reminder IDs: %w", err)
	}
	rows.Close()

	var failures []error
	for _, id := range ids {
		if err := processReminder(ctx, db, fcm, id); err != nil {
			log.Printf("process reminder %s: %v", id, err)
			failures = append(failures, fmt.Errorf("reminder %s: %w", id, err))
		}
	}
	return errors.Join(failures...)
}

func processReminder(ctx context.Context, db *pgxpool.Pool, fcm *push.Client, id uuid.UUID) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var item reminder
	err = tx.QueryRow(ctx, `
		SELECT r.id, r.user_id, r.entity_name, r.description, r.next_due_date,
		       r.advance_notice_days, to_char(r.notification_time, 'HH24:MI'), r.timezone,
		       r.recurrence::text, r.custom_interval_value, r.custom_interval_unit::text
		FROM reminders r
		WHERE r.id = $1 AND r.status = 'active'
		  AND ((r.next_due_date - r.advance_notice_days)::timestamp + r.notification_time)
		      AT TIME ZONE r.timezone <= CURRENT_TIMESTAMP
		FOR UPDATE OF r SKIP LOCKED`, id,
	).Scan(&item.ID, &item.UserID, &item.EntityName, &item.Description, &item.NextDueDate,
		&item.AdvanceNoticeDays, &item.NotificationTime, &item.Timezone,
		&item.Recurrence, &item.CustomIntervalValue, &item.CustomIntervalUnit)
	if errors.Is(err, pgx.ErrNoRows) {
		return tx.Commit(ctx)
	}
	if err != nil {
		return fmt.Errorf("load reminder: %w", err)
	}

	scheduledFor, err := scheduledNotificationTime(item)
	if err != nil {
		return fmt.Errorf("calculate notification schedule: %w", err)
	}

	nextDue, err := calculateNextDueDate(item.NextDueDate, item.Recurrence,
		item.CustomIntervalValue, item.CustomIntervalUnit)
	if err != nil {
		return fmt.Errorf("calculate next due date: %w", err)
	}

	rows, err := tx.Query(ctx, `
		SELECT id, device_token FROM user_devices
		WHERE user_id = $1 AND is_active = TRUE
		ORDER BY created_at, id`, item.UserID)
	if err != nil {
		return fmt.Errorf("load active devices: %w", err)
	}
	devices := make([]device, 0)
	for rows.Next() {
		var current device
		if err := rows.Scan(&current.ID, &current.Token); err != nil {
			rows.Close()
			return fmt.Errorf("read active device: %w", err)
		}
		devices = append(devices, current)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate active devices: %w", err)
	}
	rows.Close()
	if len(devices) == 0 {
		if err := insertNoDeviceLog(ctx, tx, item, scheduledFor); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	body := fmt.Sprintf("%s is due on %s.", item.EntityName, item.NextDueDate.Format("02 Jan 2006"))
	if item.Description != nil && *item.Description != "" {
		body += " " + *item.Description
	}

	allSettled := true
	for _, current := range devices {
		lastStatus, attempts, lastAttemptAt, err := lastDeviceAttempt(ctx, tx, item.ID, current.ID, scheduledFor)
		if err != nil {
			return err
		}
		if lastStatus == "sent" || attempts >= maxNotificationAttempts {
			continue
		}
		if !lastAttemptAt.IsZero() && time.Since(lastAttemptAt) < notificationRetryDelay {
			allSettled = false
			continue
		}

		messageID, sendErr := fcm.Send(ctx, current.Token, "Reminder: "+item.EntityName, body)
		attempt := attempts + 1
		status := "sent"
		var errorMessage *string
		var providerMessageID *string
		if sendErr != nil {
			status = "retrying"
			if attempt >= maxNotificationAttempts {
				status = "failed"
			}
			message := sendErr.Error()
			errorMessage = &message
			allSettled = false
		} else {
			providerMessageID = &messageID
			if _, err := tx.Exec(ctx, `
				UPDATE user_devices SET last_used_at = CURRENT_TIMESTAMP
				WHERE id = $1 AND user_id = $2`, current.ID, item.UserID); err != nil {
				return fmt.Errorf("update device last-used timestamp: %w", err)
			}
		}
		if err := insertDeviceNotificationLog(ctx, tx, item, current.ID, scheduledFor,
			status, attempt, providerMessageID, errorMessage); err != nil {
			return err
		}
	}
	if !allSettled {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit notification attempts: %w", err)
		}
		return nil
	}

	if _, err := tx.Exec(ctx, `UPDATE reminders SET next_due_date = $1 WHERE id = $2`, nextDue, item.ID); err != nil {
		return fmt.Errorf("advance reminder due date: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit notification and reminder update: %w", err)
	}
	log.Printf("sent device notification for reminder %s", item.ID)
	return nil
}

const (
	maxNotificationAttempts = 3
	notificationRetryDelay  = 15 * time.Minute
)

func scheduledNotificationTime(item reminder) (time.Time, error) {
	location, err := time.LoadLocation(item.Timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("load timezone %q: %w", item.Timezone, err)
	}
	notificationTime, err := time.Parse("15:04", item.NotificationTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse notification time %q: %w", item.NotificationTime, err)
	}
	noticeDate := item.NextDueDate.AddDate(0, 0, -item.AdvanceNoticeDays)
	return time.Date(noticeDate.Year(), noticeDate.Month(), noticeDate.Day(),
		notificationTime.Hour(), notificationTime.Minute(), 0, 0, location).UTC(), nil
}

func lastDeviceAttempt(ctx context.Context, tx pgx.Tx, reminderID, deviceID uuid.UUID, scheduledFor time.Time) (string, int, time.Time, error) {
	var status string
	var attempts int
	var createdAt time.Time
	err := tx.QueryRow(ctx, `
		SELECT status, retry_count, created_at
		FROM notification_logs
		WHERE reminder_id = $1 AND device_id = $2 AND scheduled_for = $3
		ORDER BY created_at DESC, id DESC
		LIMIT 1`, reminderID, deviceID, scheduledFor,
	).Scan(&status, &attempts, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, time.Time{}, nil
	}
	if err != nil {
		return "", 0, time.Time{}, fmt.Errorf("load previous device attempt: %w", err)
	}
	return status, attempts, createdAt, nil
}

func insertNoDeviceLog(ctx context.Context, tx pgx.Tx, item reminder, scheduledFor time.Time) error {
	var existing bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM notification_logs
			WHERE reminder_id = $1 AND device_id IS NULL AND scheduled_for = $2
			  AND status = 'skipped_no_device_token'
		)`, item.ID, scheduledFor).Scan(&existing); err != nil {
		return fmt.Errorf("check missing-device log: %w", err)
	}
	if existing {
		return nil
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO notification_logs
			(reminder_id, user_id, scheduled_for, status, channel, provider, error_message, retry_count)
		VALUES ($1, $2, $3, 'skipped_no_device_token', 'push', 'firebase',
		        'No active devices are registered.', 0)`,
		item.ID, item.UserID, scheduledFor)
	if err != nil {
		return fmt.Errorf("write missing-device log: %w", err)
	}
	return nil
}

func insertDeviceNotificationLog(ctx context.Context, tx pgx.Tx, item reminder,
	deviceID uuid.UUID, scheduledFor time.Time, status string, retryCount int,
	messageID, errorMessage *string,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO notification_logs
			(reminder_id, user_id, device_id, scheduled_for, sent_at, status, channel,
			 provider, provider_message_id, error_message, retry_count)
		VALUES ($1, $2, $3, $4,
		        CASE WHEN $5 = 'sent' THEN CURRENT_TIMESTAMP ELSE NULL END,
		        $5, 'push', 'firebase', NULLIF($6, ''), NULLIF($7, ''), $8)`,
		item.ID, item.UserID, deviceID, scheduledFor, status, valueOrEmpty(messageID),
		valueOrEmpty(errorMessage), retryCount)
	if err != nil {
		return fmt.Errorf("write notification log: %w", err)
	}
	return nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func calculateNextDueDate(current time.Time, recurrence string, intervalValue *int, intervalUnit *string) (time.Time, error) {
	switch recurrence {
	case "weekly":
		return current.AddDate(0, 0, 7), nil
	case "15_days":
		return current.AddDate(0, 0, 15), nil
	case "monthly":
		return addMonthsClamped(current, 1), nil
	case "yearly":
		return addMonthsClamped(current, 12), nil
	case "custom":
		if intervalValue == nil || *intervalValue < 1 || *intervalValue > 1000 || intervalUnit == nil {
			return time.Time{}, errors.New("custom recurrence has an invalid interval")
		}
		switch *intervalUnit {
		case "days":
			return current.AddDate(0, 0, *intervalValue), nil
		case "weeks":
			return current.AddDate(0, 0, *intervalValue*7), nil
		case "months":
			return addMonthsClamped(current, *intervalValue), nil
		case "years":
			return addMonthsClamped(current, 12*(*intervalValue)), nil
		default:
			return time.Time{}, errors.New("custom recurrence has an invalid interval unit")
		}
	default:
		return time.Time{}, fmt.Errorf("unsupported recurrence %q", recurrence)
	}
}

func addMonthsClamped(date time.Time, months int) time.Time {
	targetMonth := time.Date(date.Year(), date.Month()+time.Month(months), 1, 0, 0, 0, 0, time.UTC)
	lastDay := time.Date(targetMonth.Year(), targetMonth.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	day := date.Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(targetMonth.Year(), targetMonth.Month(), day, 0, 0, 0, 0, time.UTC)
}
