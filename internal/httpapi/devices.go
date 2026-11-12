package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type userDevice struct {
	ID          uuid.UUID  `json:"id"`
	DeviceToken string     `json:"device_token"`
	Platform    string     `json:"platform"`
	DeviceName  *string    `json:"device_name,omitempty"`
	IsActive    bool       `json:"is_active"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (h *Handler) listDevices(c *gin.Context) {
	claims := claimsFromContext(c)
	rows, err := h.db.Query(c.Request.Context(), `
		SELECT id, device_token, platform, device_name, is_active,
		       last_used_at, created_at, updated_at
		FROM user_devices
		WHERE user_id = $1
		ORDER BY last_used_at DESC NULLS LAST, created_at DESC`, claims.userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not list devices")
		return
	}
	defer rows.Close()

	items := make([]userDevice, 0)
	for rows.Next() {
		var item userDevice
		if err := rows.Scan(&item.ID, &item.DeviceToken, &item.Platform, &item.DeviceName,
			&item.IsActive, &item.LastUsedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			respondError(c, http.StatusInternalServerError, "could not list devices")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		respondError(c, http.StatusInternalServerError, "could not list devices")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) registerDevice(c *gin.Context) {
	var request struct {
		DeviceToken string `json:"device_token"`
		Platform    string `json:"platform"`
		DeviceName  string `json:"device_name"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "invalid JSON request")
		return
	}
	request.DeviceToken = strings.TrimSpace(request.DeviceToken)
	request.Platform = strings.ToLower(strings.TrimSpace(request.Platform))
	request.DeviceName = strings.TrimSpace(request.DeviceName)
	if request.DeviceToken == "" || len(request.DeviceToken) > 4096 {
		respondError(c, http.StatusBadRequest, "device_token is required and must be at most 4096 characters")
		return
	}
	if request.Platform != "android" && request.Platform != "ios" {
		respondError(c, http.StatusBadRequest, "platform must be android or ios")
		return
	}
	if len(request.DeviceName) > 150 {
		respondError(c, http.StatusBadRequest, "device_name must be at most 150 characters")
		return
	}

	claims := claimsFromContext(c)
	var item userDevice
	err := h.db.QueryRow(c.Request.Context(), `
		INSERT INTO user_devices (user_id, device_token, platform, device_name, is_active, last_used_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), TRUE, CURRENT_TIMESTAMP)
		ON CONFLICT (device_token) DO UPDATE
		SET user_id = EXCLUDED.user_id,
		    platform = EXCLUDED.platform,
		    device_name = EXCLUDED.device_name,
		    is_active = TRUE,
		    last_used_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		RETURNING id, device_token, platform, device_name, is_active,
		          last_used_at, created_at, updated_at`,
		claims.userID, request.DeviceToken, request.Platform, request.DeviceName,
	).Scan(&item.ID, &item.DeviceToken, &item.Platform, &item.DeviceName,
		&item.IsActive, &item.LastUsedAt, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not register device")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) updateDevice(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid device id")
		return
	}
	var request struct {
		IsActive *bool `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.IsActive == nil {
		respondError(c, http.StatusBadRequest, "is_active is required")
		return
	}

	claims := claimsFromContext(c)
	var item userDevice
	err = h.db.QueryRow(c.Request.Context(), `
		UPDATE user_devices
		SET is_active = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2 AND user_id = $3
		RETURNING id, device_token, platform, device_name, is_active,
		          last_used_at, created_at, updated_at`,
		*request.IsActive, id, claims.userID,
	).Scan(&item.ID, &item.DeviceToken, &item.Platform, &item.DeviceName,
		&item.IsActive, &item.LastUsedAt, &item.CreatedAt, &item.UpdatedAt)
	if err == pgx.ErrNoRows {
		respondError(c, http.StatusNotFound, "device not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not update device")
		return
	}
	c.JSON(http.StatusOK, item)
}

func nullableUUID(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	id := uuid.UUID(value.Bytes)
	return &id
}
