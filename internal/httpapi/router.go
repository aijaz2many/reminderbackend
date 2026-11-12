package httpapi

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/reminderapp/reminderbackend/docs"
	"github.com/reminderapp/reminderbackend/internal/config"
)

func NewRouter(pool *pgxpool.Pool, cfg config.Config) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), corsMiddleware(cfg))

	handler := &Handler{db: pool, config: cfg}
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	router.GET("/healthz", handler.health)

	api := router.Group("/api/v1")
	api.POST("/auth/register", handler.register)
	api.POST("/auth/login", handler.login)

	protected := api.Group("")
	protected.Use(handler.authenticate())
	protected.GET("/me", handler.getMe)
	protected.GET("/me/devices", handler.listDevices)
	protected.POST("/me/devices", handler.registerDevice)
	protected.PATCH("/me/devices/:id", handler.updateDevice)
	protected.POST("/reminders", handler.createReminder)
	protected.GET("/reminders", handler.listReminders)
	protected.GET("/notification-logs", handler.listNotificationLogs)
	protected.GET("/reminders/:id", handler.getReminder)
	protected.PUT("/reminders/:id", handler.replaceReminder)
	protected.PATCH("/reminders/:id/status", handler.updateReminderStatus)
	protected.DELETE("/reminders/:id", handler.deleteReminder)

	admin := protected.Group("/admin")
	admin.Use(handler.requireSuperAdmin())
	admin.GET("/analytics", handler.adminAnalytics)
	admin.GET("/users", handler.adminListUsers)
	admin.PATCH("/users/:id/subscription", handler.adminUpdateSubscription)
	admin.PATCH("/users/:id/role", handler.adminUpdateRole)

	return router
}
