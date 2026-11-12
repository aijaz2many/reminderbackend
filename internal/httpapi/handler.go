package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/reminderapp/reminderbackend/internal/config"
)

type Handler struct {
	db     *pgxpool.Pool
	config config.Config
}

type errorResponse struct {
	Error string `json:"error"`
}

func respondError(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, errorResponse{Error: message})
}

func (h *Handler) health(c *gin.Context) {
	if err := h.db.Ping(c.Request.Context()); err != nil {
		respondError(c, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
