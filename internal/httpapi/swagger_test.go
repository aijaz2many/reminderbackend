package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/reminderapp/reminderbackend/internal/config"
)

func TestSwaggerDocumentIsServed(t *testing.T) {
	router := NewRouter(nil, config.Config{})
	request := httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var document struct {
		Swagger string                 `json:"swagger"`
		Paths   map[string]interface{} `json:"paths"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
		t.Fatalf("Swagger document is not valid JSON: %v", err)
	}
	if document.Swagger != "2.0" {
		t.Fatalf("got Swagger version %q, want 2.0", document.Swagger)
	}
	for _, path := range []string{"/auth/register", "/reminders", "/notification-logs", "/admin/analytics"} {
		if _, ok := document.Paths[path]; !ok {
			t.Errorf("Swagger document is missing path %q", path)
		}
	}
}
