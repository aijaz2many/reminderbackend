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

type adminUser struct {
	ID               uuid.UUID `json:"id"`
	Email            string    `json:"email"`
	PhoneNumber      string    `json:"phone_number"`
	FullName         *string   `json:"full_name,omitempty"`
	Role             string    `json:"role"`
	SubscriptionTier string    `json:"subscription_tier"`
	CreatedAt        time.Time `json:"created_at"`
}

func (h *Handler) adminAnalytics(c *gin.Context) {
	var result struct {
		UserCount            int64 `json:"user_count"`
		ReminderCount        int64 `json:"reminder_count"`
		ActiveReminderCount  int64 `json:"active_reminder_count"`
		NotificationLogCount int64 `json:"notification_log_count"`
	}
	err := h.db.QueryRow(c.Request.Context(), `
		SELECT
			(SELECT count(*) FROM users),
			(SELECT count(*) FROM reminders),
			(SELECT count(*) FROM reminders WHERE status = 'active'),
			(SELECT count(*) FROM notification_logs)`,
	).Scan(&result.UserCount, &result.ReminderCount,
		&result.ActiveReminderCount, &result.NotificationLogCount)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not load platform analytics")
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) adminListUsers(c *gin.Context) {
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

	rows, err := h.db.Query(c.Request.Context(), `
		SELECT id, email, phone_number, full_name, role, subscription_tier, created_at
		FROM users ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not list users")
		return
	}
	defer rows.Close()

	items := make([]adminUser, 0)
	for rows.Next() {
		var item adminUser
		if err := rows.Scan(&item.ID, &item.Email, &item.PhoneNumber, &item.FullName,
			&item.Role, &item.SubscriptionTier, &item.CreatedAt); err != nil {
			respondError(c, http.StatusInternalServerError, "could not list users")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		respondError(c, http.StatusInternalServerError, "could not list users")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "limit": limit, "offset": offset})
}

func (h *Handler) adminUpdateSubscription(c *gin.Context) {
	id, ok := parseAdminUserID(c)
	if !ok {
		return
	}
	var request struct {
		SubscriptionTier string `json:"subscription_tier"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "invalid JSON request")
		return
	}
	request.SubscriptionTier = strings.TrimSpace(request.SubscriptionTier)
	if request.SubscriptionTier == "" || len(request.SubscriptionTier) > 20 {
		respondError(c, http.StatusBadRequest, "subscription_tier is required and must be at most 20 characters")
		return
	}
	var updated adminUser
	err := h.db.QueryRow(c.Request.Context(), `
		UPDATE users SET subscription_tier = $1 WHERE id = $2
		RETURNING id, email, phone_number, full_name, role, subscription_tier, created_at`,
		request.SubscriptionTier, id,
	).Scan(&updated.ID, &updated.Email, &updated.PhoneNumber, &updated.FullName,
		&updated.Role, &updated.SubscriptionTier, &updated.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		respondError(c, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not update subscription")
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) adminUpdateRole(c *gin.Context) {
	id, ok := parseAdminUserID(c)
	if !ok {
		return
	}
	var request struct {
		Role string `json:"role"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "invalid JSON request")
		return
	}
	if request.Role != "tenant_user" && request.Role != "super_admin" {
		respondError(c, http.StatusBadRequest, "role must be tenant_user or super_admin")
		return
	}
	var updated adminUser
	err := h.db.QueryRow(c.Request.Context(), `
		UPDATE users SET role = $1 WHERE id = $2
		RETURNING id, email, phone_number, full_name, role, subscription_tier, created_at`,
		request.Role, id,
	).Scan(&updated.ID, &updated.Email, &updated.PhoneNumber, &updated.FullName,
		&updated.Role, &updated.SubscriptionTier, &updated.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		respondError(c, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not update user role")
		return
	}
	c.JSON(http.StatusOK, updated)
}

func parseAdminUserID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid user id")
		return uuid.Nil, false
	}
	return id, true
}
