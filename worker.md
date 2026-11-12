package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"
)

type Reminder struct {
	ID                  string
	UserID              string
	EntityName          string
	Description         string
	NextDueDate         time.Time
	Recurrence          string
	CustomIntervalValue *int
	CustomIntervalUnit  *string
	FCMDeviceToken      *string
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func main() {
	ctx := context.Background()
	dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/reminder_saas?sslmode=disable")

	// Initialize PostgreSQL connection pool using pgx
	dbPool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer dbPool.Close()

	if err := dbPool.Ping(ctx); err != nil {
		log.Fatalf("Database ping failed: %v\n", err)
	}

	// Initialize Cron Scheduler
	c := cron.New(cron.WithLocation(time.UTC))

	// Schedule job to run every day at 00:01 AM UTC
	_, err = c.AddFunc("1 0 * * *", func() {
		log.Println("Running daily reminder background job...")
		processDueReminders(dbPool)
	})

	if err != nil {
		log.Fatalf("Failed to schedule cron job: %v", err)
	}

	c.Start()
	log.Println("Background push-notification worker started...")

	select {}
}

// Core processing logic
func processDueReminders(db *pgxpool.Pool) {
	ctx := context.Background()

	// Query active reminders due today or within advance notice window
	query := `
		SELECT 
			r.id, r.user_id, r.entity_name, r.description, r.next_due_date, 
			r.recurrence, r.custom_interval_value, r.custom_interval_unit,
			u.fcm_device_token
		FROM reminders r
		JOIN users u ON r.user_id = u.id
		WHERE r.status = 'active'
		  AND r.next_due_date <= CURRENT_DATE + (r.advance_notice_days || ' days')::INTERVAL
	`

	rows, err := db.Query(ctx, query)
	if err != nil {
		log.Printf("Error querying reminders: %v\n", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var rem Reminder
		err := rows.Scan(
			&rem.ID, &rem.UserID, &rem.EntityName, &rem.Description, &rem.NextDueDate,
			&rem.Recurrence, &rem.CustomIntervalValue, &rem.CustomIntervalUnit,
			&rem.FCMDeviceToken,
		)
		if err != nil {
			log.Printf("Error scanning reminder row: %v\n", err)
			continue
		}

		// 1. Dispatch Push Notification Only
		sendPushNotification(rem)

		// 2. Calculate Next Due Date & Update Database
		newDueDate := calculateNextDueDate(rem.NextDueDate, rem.Recurrence, rem.CustomIntervalValue, rem.CustomIntervalUnit)
		
		updateQuery := `
			UPDATE reminders 
			SET next_due_date = $1, 
			    status = CASE WHEN $2 = 'one_time' THEN 'completed' ELSE 'active' END
			WHERE id = $3
		`
		_, err = db.Exec(ctx, updateQuery, newDueDate, rem.Recurrence, rem.ID)
		if err != nil {
			log.Printf("Failed to update next due date for reminder %s: %v\n", rem.ID, err)
			continue
		}

		// 3. Log notification execution
		logQuery := `
			INSERT INTO notification_logs (reminder_id, user_id, scheduled_for, status, channel)
			VALUES ($1, $2, $3, 'sent', 'push')
		`
		_, _ = db.Exec(ctx, logQuery, rem.ID, rem.UserID, rem.NextDueDate)
		log.Printf("Successfully processed reminder [%s] for entity: %s\n", rem.ID, rem.EntityName)
	}
}

// Recurrence calculation supporting 15-day, monthly, yearly, and custom intervals
func calculateNextDueDate(currentDue time.Time, recurrence string, intervalVal *int, intervalUnit *string) time.Time {
	switch recurrence {
	case "weekly":
		return currentDue.AddDate(0, 0, 7)
	case "15_days":
		return currentDue.AddDate(0, 0, 15)
	case "monthly":
		return currentDue.AddDate(0, 1, 0)
	case "yearly":
		return currentDue.AddDate(1, 0, 0)
	case "custom":
		if intervalVal == nil || intervalUnit == nil {
			return currentDue.AddDate(0, 0, 1)
		}
		val := *intervalVal
		switch *intervalUnit {
		case "days":
			return currentDue.AddDate(0, 0, val)
		case "weeks":
			return currentDue.AddDate(0, 0, val*7)
		case "months":
			return currentDue.AddDate(0, val, 0)
		case "years":
			return currentDue.AddDate(val, 0, 0)
		default:
			return currentDue.AddDate(0, 0, val)
		}
	default:
		return currentDue.AddDate(0, 1, 0)
	}
}

// Dispatch Push Notification via Firebase FCM
func sendPushNotification(rem Reminder) {
	if rem.FCMDeviceToken == nil || *rem.FCMDeviceToken == "" {
		log.Printf("Skipping push: No FCM token for user ID %s\n", rem.UserID)
		return
	}

	title := fmt.Sprintf("Due Soon: %s", rem.EntityName)
	body := fmt.Sprintf("'%s' is due on %s. %s", rem.EntityName, rem.NextDueDate.Format("02 Jan 2006"), rem.Description)

	serverKey := getEnv("FCM_SERVER_KEY", "mock_fcm_key")
	if serverKey == "mock_fcm_key" {
		log.Printf("[Mock Push to Token %s]\nTitle: %s\nBody: %s\n", *rem.FCMDeviceToken, title, body)
		return
	}

	payload := map[string]interface{}{
		"to": *rem.FCMDeviceToken,
		"notification": map[string]string{
			"title": title,
			"body":  body,
		},
	}
	jsonPayload, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "https://fcm.googleapis.com/fcm/send", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Authorization", "key="+serverKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode >= 400 {
		log.Printf("Failed to send FCM push notification for reminder %s\n", rem.ID)
		return
	}
	defer resp.Body.Close()
}