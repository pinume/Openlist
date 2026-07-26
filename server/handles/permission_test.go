package handles

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/db"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var permissionTestDBOnce sync.Once

func setupPermissionTestDB(t *testing.T) {
	t.Helper()
	permissionTestDBOnce.Do(func() {
		testDB, err := gorm.Open(sqlite.Open("file:handles_permission?mode=memory&cache=shared"))
		if err != nil {
			t.Fatalf("open permission test database: %v", err)
		}
		db.Init(testDB)
	})
}

func testContext(t *testing.T, body string, user *model.User) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req.WithContext(context.WithValue(req.Context(), conf.UserKey, user))
	return c, recorder
}

func TestFsRemoveRejectsRestrictedDescendantMeta(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupPermissionTestDB(t)
	if err := op.CreateMeta(&model.Meta{
		Path:          "/remove-parent/restricted",
		WriteUsers:    []uint{2},
		WriteUsersSub: true,
	}); err != nil {
		t.Fatalf("create restricted meta: %v", err)
	}

	c, recorder := testContext(
		t,
		`{"dir":"/","names":["remove-parent"]}`,
		&model.User{ID: 1, Permission: 1 << 7},
	)
	FsRemove(c)

	if !strings.Contains(recorder.Body.String(), `"code":403`) {
		t.Fatalf("expected restricted descendant to block removal, got %s", recorder.Body.String())
	}
}

func TestDirectUploadAuthorizesBodyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupPermissionTestDB(t)
	if err := op.CreateMeta(&model.Meta{
		Path:          "/direct-restricted",
		WriteUsers:    []uint{2},
		WriteUsersSub: true,
	}); err != nil {
		t.Fatalf("create restricted meta: %v", err)
	}

	c, recorder := testContext(
		t,
		`{"path":"/direct-restricted","file_name":"file.txt","file_size":1}`,
		&model.User{ID: 1, Permission: 1 << 3},
	)
	FsGetDirectUploadInfo(c)

	if !strings.Contains(recorder.Body.String(), `"code":403`) {
		t.Fatalf("expected body path permission failure, got %s", recorder.Body.String())
	}
}
