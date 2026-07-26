package webdav

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/db"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var permissionTestDBOnce sync.Once

func setupPermissionTestDB(t *testing.T) {
	t.Helper()
	permissionTestDBOnce.Do(func() {
		testDB, err := gorm.Open(sqlite.Open("file:webdav_permission?mode=memory&cache=shared"))
		if err != nil {
			t.Fatalf("open permission test database: %v", err)
		}
		db.Init(testDB)
	})
}

func TestMoveFilesRejectsRestrictedDescendantMeta(t *testing.T) {
	setupPermissionTestDB(t)
	if err := op.CreateMeta(&model.Meta{
		Path:          "/webdav-parent/restricted",
		WriteUsers:    []uint{2},
		WriteUsersSub: true,
	}); err != nil {
		t.Fatalf("create restricted meta: %v", err)
	}
	user := &model.User{ID: 1, Permission: 1 << 5}
	ctx := context.WithValue(context.Background(), conf.UserKey, user)

	status, err := moveFiles(ctx, "/webdav-parent", "/destination/webdav-parent", false)
	if err != nil {
		t.Fatalf("moveFiles() error = %v", err)
	}
	if status != http.StatusForbidden {
		t.Fatalf("moveFiles() status = %d, want %d", status, http.StatusForbidden)
	}
}
