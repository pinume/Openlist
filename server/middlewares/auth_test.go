package middlewares

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAuthenticationMiddlewareRejectsEmptyToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name    string
		handler gin.HandlerFunc
	}{
		{name: "Auth with disabled guest flag", handler: Auth(false)},
		{name: "Auth with legacy guest flag", handler: Auth(true)},
		{name: "Authn", handler: Authn},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nextCalled := false
			router := gin.New()
			router.GET("/", test.handler, func(c *gin.Context) {
				nextCalled = true
				c.Status(http.StatusNoContent)
			})

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

			if nextCalled {
				t.Fatal("request without a token reached the protected handler")
			}
			if !strings.Contains(recorder.Body.String(), `"code":401`) {
				t.Fatalf("expected authentication error, got %s", recorder.Body.String())
			}
		})
	}
}
