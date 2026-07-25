package handles

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/gin-gonic/gin"
)

func TestCurrentUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("missing user", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

		user, ok := CurrentUser(c)
		if ok || user != nil {
			t.Fatal("CurrentUser() accepted a missing user")
		}
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("CurrentUser() status = %d, want %d", recorder.Code, http.StatusUnauthorized)
		}
	})

	t.Run("authenticated user", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		want := &model.User{ID: 42, Username: "user"}
		c.Request = req.WithContext(context.WithValue(req.Context(), conf.UserKey, want))

		user, ok := CurrentUser(c)
		if !ok {
			t.Fatal("CurrentUser() rejected an authenticated user")
		}
		if user != want {
			t.Fatalf("CurrentUser() = %p, want %p", user, want)
		}
	})
}
