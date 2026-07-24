package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/server/common"
	"github.com/gin-gonic/gin"
)

func TestUserPathAccessRejectsPathOutsideUserBase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	nextCalled := false
	router := gin.New()
	router.GET("/download", func(c *gin.Context) {
		common.GinAppendValues(
			c,
			conf.UserKey, &model.User{BasePath: "/users/alice"},
			conf.PathKey, "/users/bob/private.txt",
		)
		c.Next()
	}, UserPathAccess, func(c *gin.Context) {
		nextCalled = true
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/download", nil))

	if nextCalled {
		t.Fatal("user accessed a direct download outside their base path")
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", recorder.Code)
	}
}
