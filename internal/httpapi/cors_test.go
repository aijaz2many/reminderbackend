package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/reminderapp/reminderbackend/internal/config"
)

func TestCORSMiddlewareAllowsLocalDevelopmentPreflight(t *testing.T) {
	router := corsTestRouter(config.Config{Environment: "development"})
	request := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	request.Header.Set("Origin", "http://localhost:8081")
	request.Header.Set("Access-Control-Request-Method", "POST")
	request.Header.Set("Access-Control-Request-Headers", "content-type")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:8081" {
		t.Errorf("Access-Control-Allow-Origin = %q", got)
	}
	if got := response.Header().Get("Access-Control-Allow-Headers"); got != "Accept, Authorization, Content-Type" {
		t.Errorf("Access-Control-Allow-Headers = %q", got)
	}
}

func TestCORSMiddlewareRequiresConfiguredOriginInProduction(t *testing.T) {
	router := corsTestRouter(config.Config{
		Environment:        "production",
		CORSAllowedOrigins: []string{"https://app.example.com"},
	})

	allowedResponse := httptest.NewRecorder()
	allowedRequest := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	allowedRequest.Header.Set("Origin", "https://app.example.com")
	router.ServeHTTP(allowedResponse, allowedRequest)
	if got := allowedResponse.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("configured origin was not allowed: got %q", got)
	}

	blockedResponse := httptest.NewRecorder()
	blockedRequest := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	blockedRequest.Header.Set("Origin", "https://untrusted.example")
	router.ServeHTTP(blockedResponse, blockedRequest)
	if got := blockedResponse.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("unconfigured origin was allowed: %q", got)
	}
}

func corsTestRouter(cfg config.Config) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(corsMiddleware(cfg))
	router.POST("/resource", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	return router
}
