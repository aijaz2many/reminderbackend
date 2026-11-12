package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type notificationLog struct {
	ID                uuid.UUID  `json:"id"`
	ReminderID        *uuid.UUID `json:"reminder_id,omitempty"`
	UserID            *uuid.UUID `json:"user_id,omitempty"`
	DeviceID          *uuid.UUID `json:"device_id,omitempty"`
	ScheduledFor      time.Time  `json:"scheduled_for"`
	SentAt            *time.Time `json:"sent_at,omitempty"`
	Status            string     `json:"status"`
	Channel           string     `json:"channel"`
	Provider          *string    `json:"provider,omitempty"`
	ProviderMessageID *string    `json:"provider_message_id,omitempty"`
	ErrorMessage      *string    `json:"error_message,omitempty"`
	RetryCount        int        `json:"retry_count"`
	CreatedAt         time.Time  `json:"created_at"`
}

func (h *Handler) listNotificationLogs(c *gin.Context) {
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
		SELECT id, reminder_id, user_id, device_id, scheduled_for, sent_at,
		       status, channel, provider, provider_message_id, error_message,
		       retry_count, created_at
		FROM notification_logs
		WHERE user_id = $1
		ORDER BY scheduled_for DESC, id DESC
		LIMIT $2 OFFSET $3`, claims.userID, limit, offset)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not list notification logs")
		return
	}
	defer rows.Close()

	items := make([]notificationLog, 0)
	for rows.Next() {
		var item notificationLog
		var reminderID, userID, deviceID pgtype.UUID
		var provider, providerMessageID, errorMessage pgtype.Text
		if err := rows.Scan(&item.ID, &reminderID, &userID, &deviceID, &item.ScheduledFor,
			&item.SentAt, &item.Status, &item.Channel, &provider, &providerMessageID,
			&errorMessage, &item.RetryCount, &item.CreatedAt); err != nil {
			respondError(c, http.StatusInternalServerError, "could not list notification logs")
			return
		}
		item.ReminderID = nullableUUID(reminderID)
		item.UserID = nullableUUID(userID)
		item.DeviceID = nullableUUID(deviceID)
		item.Provider = nullableText(provider)
		item.ProviderMessageID = nullableText(providerMessageID)
		item.ErrorMessage = nullableText(errorMessage)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		respondError(c, http.StatusInternalServerError, "could not list notification logs")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "limit": limit, "offset": offset})
}

func nullableText(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
