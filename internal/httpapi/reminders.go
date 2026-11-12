package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type reminder struct {
	ID                  uuid.UUID `json:"id"`
	UserID              uuid.UUID `json:"user_id"`
	EntityName          string    `json:"entity_name"`
	Description         *string   `json:"description,omitempty"`
	StartDate           string    `json:"start_date"`
	NextDueDate         string    `json:"next_due_date"`
	Recurrence          string    `json:"recurrence"`
	CustomIntervalValue *int      `json:"custom_interval_value,omitempty"`
	CustomIntervalUnit  *string   `json:"custom_interval_unit,omitempty"`
	AdvanceNoticeDays   int       `json:"advance_notice_days"`
	NotificationTime    string    `json:"notification_time"`
	Timezone            string    `json:"timezone"`
	Status              string    `json:"status"`
	CreatedAt           time.Time `json:"created_at"`
}

type reminderRequest struct {
	EntityName          string  `json:"entity_name"`
	Description         *string `json:"description"`
	StartDate           string  `json:"start_date"`
	Recurrence          string  `json:"recurrence"`
	CustomIntervalValue *int    `json:"custom_interval_value"`
	CustomIntervalUnit  *string `json:"custom_interval_unit"`
	AdvanceNoticeDays   int     `json:"advance_notice_days"`
	NotificationTime    string  `json:"notification_time"`
	Timezone            string  `json:"timezone"`
}

func (h *Handler) createReminder(c *gin.Context) {
	request, startDate, nextDue, ok := parseReminderRequest(c)
	if !ok {
		return
	}
	claims := claimsFromContext(c)
	var created reminder
	err := h.db.QueryRow(c.Request.Context(), `
		INSERT INTO reminders (
			user_id, entity_name, description, start_date, next_due_date, recurrence,
			custom_interval_value, custom_interval_unit, advance_notice_days,
			notification_time, timezone
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::time, $11)
		RETURNING id, user_id, entity_name, description,
		          to_char(start_date, 'YYYY-MM-DD'), to_char(next_due_date, 'YYYY-MM-DD'),
		          recurrence, custom_interval_value, custom_interval_unit,
		          advance_notice_days, to_char(notification_time, 'HH24:MI'), timezone,
		          status, created_at`,
		claims.userID, request.EntityName, request.Description, startDate, nextDue,
		request.Recurrence, request.CustomIntervalValue, request.CustomIntervalUnit,
		request.AdvanceNoticeDays, request.NotificationTime, request.Timezone,
	).Scan(&created.ID, &created.UserID, &created.EntityName, &created.Description,
		&created.StartDate, &created.NextDueDate, &created.Recurrence,
		&created.CustomIntervalValue, &created.CustomIntervalUnit,
		&created.AdvanceNoticeDays, &created.NotificationTime, &created.Timezone,
		&created.Status, &created.CreatedAt)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not create reminder")
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) replaceReminder(c *gin.Context) {
	id, ok := parseReminderID(c)
	if !ok {
		return
	}
	request, startDate, nextDue, ok := parseReminderRequest(c)
	if !ok {
		return
	}
	claims := claimsFromContext(c)
	var updated reminder
	err := h.db.QueryRow(c.Request.Context(), `
		UPDATE reminders
		SET entity_name = $1, description = $2, start_date = $3, next_due_date = $4,
		    recurrence = $5, custom_interval_value = $6, custom_interval_unit = $7,
		    advance_notice_days = $8, notification_time = $9::time, timezone = $10
		WHERE id = $11 AND user_id = $12
		RETURNING id, user_id, entity_name, description,
		          to_char(start_date, 'YYYY-MM-DD'), to_char(next_due_date, 'YYYY-MM-DD'),
		          recurrence, custom_interval_value, custom_interval_unit,
		          advance_notice_days, to_char(notification_time, 'HH24:MI'), timezone,
		          status, created_at`,
		request.EntityName, request.Description, startDate, nextDue,
		request.Recurrence, request.CustomIntervalValue, request.CustomIntervalUnit,
		request.AdvanceNoticeDays, request.NotificationTime, request.Timezone, id, claims.userID,
	).Scan(&updated.ID, &updated.UserID, &updated.EntityName, &updated.Description,
		&updated.StartDate, &updated.NextDueDate, &updated.Recurrence,
		&updated.CustomIntervalValue, &updated.CustomIntervalUnit,
		&updated.AdvanceNoticeDays, &updated.NotificationTime, &updated.Timezone,
		&updated.Status, &updated.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		respondError(c, http.StatusNotFound, "reminder not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not update reminder")
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) listReminders(c *gin.Context) {
	limit := 50
	offset := 0
	if value := c.Query("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			respondError(c, http.StatusBadRequest, "limit must be between 1 and 100")
			return
		}
		limit = parsed
	}
	if value := c.Query("offset"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			respondError(c, http.StatusBadRequest, "offset must be a non-negative integer")
			return
		}
		offset = parsed
	}

	claims := claimsFromContext(c)
	rows, err := h.db.Query(c.Request.Context(), `
		SELECT id, user_id, entity_name, description,
		       to_char(start_date, 'YYYY-MM-DD'), to_char(next_due_date, 'YYYY-MM-DD'),
		       recurrence, custom_interval_value, custom_interval_unit,
		       advance_notice_days, to_char(notification_time, 'HH24:MI'), timezone,
		       status, created_at
		FROM reminders WHERE user_id = $1
		ORDER BY next_due_date, created_at
		LIMIT $2 OFFSET $3`, claims.userID, limit, offset)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not list reminders")
		return
	}
	defer rows.Close()

	items := make([]reminder, 0)
	for rows.Next() {
		var item reminder
		if err := rows.Scan(&item.ID, &item.UserID, &item.EntityName, &item.Description,
			&item.StartDate, &item.NextDueDate, &item.Recurrence,
			&item.CustomIntervalValue, &item.CustomIntervalUnit,
			&item.AdvanceNoticeDays, &item.NotificationTime, &item.Timezone,
			&item.Status, &item.CreatedAt); err != nil {
			respondError(c, http.StatusInternalServerError, "could not list reminders")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		respondError(c, http.StatusInternalServerError, "could not list reminders")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "limit": limit, "offset": offset})
}

func (h *Handler) getReminder(c *gin.Context) {
	id, ok := parseReminderID(c)
	if !ok {
		return
	}
	claims := claimsFromContext(c)
	item, err := h.loadReminder(c, id, claims.userID)
	if errors.Is(err, pgx.ErrNoRows) {
		respondError(c, http.StatusNotFound, "reminder not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not load reminder")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) updateReminderStatus(c *gin.Context) {
	id, ok := parseReminderID(c)
	if !ok {
		return
	}
	var request struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "invalid JSON request")
		return
	}
	switch request.Status {
	case "active", "paused", "completed":
	default:
		respondError(c, http.StatusBadRequest, "status must be active, paused, or completed")
		return
	}
	claims := claimsFromContext(c)
	var updated reminder
	err := h.db.QueryRow(c.Request.Context(), `
		UPDATE reminders SET status = $1
		WHERE id = $2 AND user_id = $3
		RETURNING id, user_id, entity_name, description,
		          to_char(start_date, 'YYYY-MM-DD'), to_char(next_due_date, 'YYYY-MM-DD'),
		          recurrence, custom_interval_value, custom_interval_unit,
		          advance_notice_days, to_char(notification_time, 'HH24:MI'), timezone,
		          status, created_at`,
		request.Status, id, claims.userID,
	).Scan(&updated.ID, &updated.UserID, &updated.EntityName, &updated.Description,
		&updated.StartDate, &updated.NextDueDate, &updated.Recurrence,
		&updated.CustomIntervalValue, &updated.CustomIntervalUnit,
		&updated.AdvanceNoticeDays, &updated.NotificationTime, &updated.Timezone,
		&updated.Status, &updated.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		respondError(c, http.StatusNotFound, "reminder not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not update reminder")
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) deleteReminder(c *gin.Context) {
	id, ok := parseReminderID(c)
	if !ok {
		return
	}
	claims := claimsFromContext(c)
	tag, err := h.db.Exec(c.Request.Context(), `DELETE FROM reminders WHERE id = $1 AND user_id = $2`,
		id, claims.userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not delete reminder")
		return
	}
	if tag.RowsAffected() == 0 {
		respondError(c, http.StatusNotFound, "reminder not found")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) loadReminder(c *gin.Context, id uuid.UUID, userID uuid.UUID) (reminder, error) {
	var item reminder
	err := h.db.QueryRow(c.Request.Context(), `
		SELECT id, user_id, entity_name, description,
		       to_char(start_date, 'YYYY-MM-DD'), to_char(next_due_date, 'YYYY-MM-DD'),
		       recurrence, custom_interval_value, custom_interval_unit,
		       advance_notice_days, to_char(notification_time, 'HH24:MI'), timezone,
		       status, created_at
		FROM reminders WHERE id = $1 AND user_id = $2`, id, userID,
	).Scan(&item.ID, &item.UserID, &item.EntityName, &item.Description,
		&item.StartDate, &item.NextDueDate, &item.Recurrence,
		&item.CustomIntervalValue, &item.CustomIntervalUnit,
		&item.AdvanceNoticeDays, &item.NotificationTime, &item.Timezone,
		&item.Status, &item.CreatedAt)
	return item, err
}

func parseReminderID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid reminder id")
		return uuid.Nil, false
	}
	return id, true
}

func parseReminderRequest(c *gin.Context) (reminderRequest, time.Time, time.Time, bool) {
	var request reminderRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "invalid JSON request")
		return request, time.Time{}, time.Time{}, false
	}
	request.EntityName = strings.TrimSpace(request.EntityName)
	request.Recurrence = strings.TrimSpace(request.Recurrence)
	request.NotificationTime = strings.TrimSpace(request.NotificationTime)
	if request.NotificationTime == "" {
		request.NotificationTime = "09:00"
	}
	request.Timezone = strings.TrimSpace(request.Timezone)
	if request.Timezone == "" {
		request.Timezone = "Asia/Kolkata"
	}
	if request.CustomIntervalUnit != nil {
		unit := strings.TrimSpace(*request.CustomIntervalUnit)
		request.CustomIntervalUnit = &unit
	}
	if request.EntityName == "" || len(request.EntityName) > 255 {
		respondError(c, http.StatusBadRequest, "entity_name is required and must be at most 255 characters")
		return request, time.Time{}, time.Time{}, false
	}
	if request.Description != nil && len(*request.Description) > 10000 {
		respondError(c, http.StatusBadRequest, "description must be at most 10000 characters")
		return request, time.Time{}, time.Time{}, false
	}
	startDate, err := time.Parse("2006-01-02", request.StartDate)
	if err != nil {
		respondError(c, http.StatusBadRequest, "start_date must use YYYY-MM-DD format")
		return request, time.Time{}, time.Time{}, false
	}
	if request.AdvanceNoticeDays < 0 || request.AdvanceNoticeDays > 365 {
		respondError(c, http.StatusBadRequest, "advance_notice_days must be between 0 and 365")
		return request, time.Time{}, time.Time{}, false
	}
	if _, err := time.Parse("15:04", request.NotificationTime); err != nil {
		respondError(c, http.StatusBadRequest, "notification_time must use HH:MM format")
		return request, time.Time{}, time.Time{}, false
	}
	if _, err := time.LoadLocation(request.Timezone); err != nil {
		respondError(c, http.StatusBadRequest, "timezone must be a valid IANA timezone")
		return request, time.Time{}, time.Time{}, false
	}
	if request.Recurrence != "custom" && (request.CustomIntervalValue != nil || request.CustomIntervalUnit != nil) {
		respondError(c, http.StatusBadRequest, "custom interval fields are only valid for custom recurrence")
		return request, time.Time{}, time.Time{}, false
	}
	nextDue, err := nextDueDate(startDate, request.Recurrence, request.CustomIntervalValue, request.CustomIntervalUnit)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return request, time.Time{}, time.Time{}, false
	}
	return request, startDate, nextDue, true
}

func nextDueDate(start time.Time, recurrence string, intervalValue *int, intervalUnit *string) (time.Time, error) {
	switch recurrence {
	case "weekly":
		return start.AddDate(0, 0, 7), nil
	case "15_days":
		return start.AddDate(0, 0, 15), nil
	case "monthly":
		return addMonthsClamped(start, 1), nil
	case "yearly":
		return addMonthsClamped(start, 12), nil
	case "custom":
		if intervalValue == nil || *intervalValue < 1 || *intervalValue > 1000 || intervalUnit == nil {
			return time.Time{}, errors.New("custom recurrence requires custom_interval_value (1-1000) and custom_interval_unit")
		}
		switch *intervalUnit {
		case "days":
			return start.AddDate(0, 0, *intervalValue), nil
		case "weeks":
			return start.AddDate(0, 0, 7*(*intervalValue)), nil
		case "months":
			return addMonthsClamped(start, *intervalValue), nil
		case "years":
			return addMonthsClamped(start, 12*(*intervalValue)), nil
		default:
			return time.Time{}, errors.New("custom_interval_unit must be days, weeks, months, or years")
		}
	default:
		return time.Time{}, errors.New("recurrence must be weekly, 15_days, monthly, yearly, or custom")
	}
}

func addMonthsClamped(date time.Time, months int) time.Time {
	firstOfTargetMonth := time.Date(date.Year(), date.Month()+time.Month(months), 1, 0, 0, 0, 0, date.Location())
	lastDay := time.Date(firstOfTargetMonth.Year(), firstOfTargetMonth.Month()+1, 0, 0, 0, 0, 0, date.Location()).Day()
	day := date.Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(firstOfTargetMonth.Year(), firstOfTargetMonth.Month(), day, 0, 0, 0, 0, date.Location())
}
