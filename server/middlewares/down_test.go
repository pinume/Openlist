package middlewares

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/server/common"
	"github.com/gin-gonic/gin"
)

func TestSignedDownloadAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, authorization := range []string{"", "stale-browser-token"} {
		t.Run(authorization, func(t *testing.T) {
			nextCalled := false
			router := gin.New()
			router.GET("/download", func(c *gin.Context) {
				common.GinAppendValues(c, conf.PathKey, "/local/file.txt")
				c.Next()
			}, DownloadAuth(func(path, signature string) error {
				if path != "/local/file.txt" || signature != "valid" {
					return errors.New("invalid signature")
				}
				return nil
			}), UserPathAccess, func(c *gin.Context) {
				nextCalled = true
				c.Status(http.StatusNoContent)
			})

			request := httptest.NewRequest(http.MethodGet, "/download?sign=valid", nil)
			if authorization != "" {
				request.Header.Set("Authorization", authorization)
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			if !nextCalled {
				t.Fatal("valid signed download did not reach handler")
			}
			if recorder.Code != http.StatusNoContent {
				t.Fatalf("expected status 204, got %d", recorder.Code)
			}
		})
	}
}

func TestSignedDownloadRejectsMissingOrInvalidSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, target := range []string{"/download", "/download?sign=invalid"} {
		t.Run(target, func(t *testing.T) {
			nextCalled := false
			router := gin.New()
			router.GET("/download", func(c *gin.Context) {
				common.GinAppendValues(c, conf.PathKey, "/local/file.txt")
				c.Next()
			}, DownloadAuth(func(_, signature string) error {
				if signature != "valid" {
					return errors.New("invalid signature")
				}
				return nil
			}), func(c *gin.Context) {
				nextCalled = true
				c.Status(http.StatusNoContent)
			})

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))

			if nextCalled {
				t.Fatal("unsigned download reached handler")
			}
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("expected status 401, got %d", recorder.Code)
			}
		})
	}
}

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
