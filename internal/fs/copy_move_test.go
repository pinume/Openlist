package fs

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	_ "github.com/OpenListTeam/OpenList/v4/drivers"
	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/db"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var copyMoveTestDBOnce sync.Once

func setupCopyMoveTestDB(t *testing.T) {
	t.Helper()
	copyMoveTestDBOnce.Do(func() {
		conf.Conf = conf.DefaultConfig(t.TempDir())
		testDB, err := gorm.Open(sqlite.Open("file:fs_copy_move?mode=memory&cache=shared"))
		if err != nil {
			t.Fatalf("open copy/move test database: %v", err)
		}
		db.Init(testDB)
	})
}

func setupCopyMoveLocalStorage(t *testing.T) (mountPath, rootDir string) {
	t.Helper()
	setupCopyMoveTestDB(t)
	rootDir = t.TempDir()
	addition, err := json.Marshal(map[string]any{
		"root_folder_path": rootDir,
		"show_hidden":      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	mountPath = "/copy-move-" + t.Name()
	storageID, err := op.CreateStorage(context.Background(), model.Storage{
		Driver:    "Local",
		MountPath: mountPath,
		Addition:  string(addition),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := op.DeleteStorageById(context.Background(), storageID); err != nil {
			t.Errorf("delete test storage: %v", err)
		}
	})
	return mountPath, rootDir
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// syncCopyCtx forces transfer() to run the FileTransferTask synchronously
// in the calling goroutine (see the NoTaskKey branch in transfer), so tests
// don't need to poll an async task manager.
func syncCopyCtx(skipExisting bool) context.Context {
	ctx := context.WithValue(context.Background(), conf.UserKey, &model.User{ID: 1, Permission: -1})
	ctx = context.WithValue(ctx, conf.NoTaskKey, struct{}{})
	if skipExisting {
		ctx = context.WithValue(ctx, conf.SkipExistingKey, struct{}{})
	}
	return ctx
}

func TestCopySkipsExistingFileWithSameSize(t *testing.T) {
	mountPath, rootDir := setupCopyMoveLocalStorage(t)
	mustWrite(t, filepath.Join(rootDir, "src", "report"), "AAAAA")
	mustWrite(t, filepath.Join(rootDir, "dst", "report"), "ZZZZZ") // same size, different content

	_, err := Copy(syncCopyCtx(true), mountPath+"/src/report", mountPath+"/dst")
	if err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	if got := mustRead(t, filepath.Join(rootDir, "dst", "report")); got != "ZZZZZ" {
		t.Fatalf("destination content = %q, want unchanged %q (skip_existing should not touch it)", got, "ZZZZZ")
	}
}

func TestCopyOverwritesExistingFileWithDifferentSize(t *testing.T) {
	mountPath, rootDir := setupCopyMoveLocalStorage(t)
	mustWrite(t, filepath.Join(rootDir, "src", "report"), "full content")
	mustWrite(t, filepath.Join(rootDir, "dst", "report"), "short")

	_, err := Copy(syncCopyCtx(true), mountPath+"/src/report", mountPath+"/dst")
	if err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	if got := mustRead(t, filepath.Join(rootDir, "dst", "report")); got != "full content" {
		t.Fatalf("destination content = %q, want overwritten with %q", got, "full content")
	}
}

func TestCopyPropagatesSkipExistingToNestedFiles(t *testing.T) {
	mountPath, rootDir := setupCopyMoveLocalStorage(t)
	mustWrite(t, filepath.Join(rootDir, "src", "dir", "same-size"), "12345")
	mustWrite(t, filepath.Join(rootDir, "src", "dir", "diff-size"), "full content")
	mustWrite(t, filepath.Join(rootDir, "src", "dir", "brand-new"), "new")
	mustWrite(t, filepath.Join(rootDir, "dst", "dir", "same-size"), "67890") // same size (5)
	mustWrite(t, filepath.Join(rootDir, "dst", "dir", "diff-size"), "x")     // different size

	_, err := Copy(syncCopyCtx(true), mountPath+"/src/dir", mountPath+"/dst")
	if err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	if got := mustRead(t, filepath.Join(rootDir, "dst", "dir", "same-size")); got != "67890" {
		t.Fatalf("same-size nested file = %q, want left untouched %q", got, "67890")
	}
	if got := mustRead(t, filepath.Join(rootDir, "dst", "dir", "diff-size")); got != "full content" {
		t.Fatalf("diff-size nested file = %q, want overwritten with %q", got, "full content")
	}
	if got := mustRead(t, filepath.Join(rootDir, "dst", "dir", "brand-new")); got != "new" {
		t.Fatalf("brand-new nested file = %q, want copied as %q", got, "new")
	}
}

func TestMoveIgnoresSkipExistingContextValue(t *testing.T) {
	mountPath, rootDir := setupCopyMoveLocalStorage(t)
	mustWrite(t, filepath.Join(rootDir, "src", "report"), "moved content")
	mustWrite(t, filepath.Join(rootDir, "dst", "report"), "old content, same size!!") // deliberately different size

	// SkipExisting is set in ctx (as if a copy request had set it), but
	// this is a Move: shouldSkipExistingFile/SkipExisting must not apply.
	_, err := Move(syncCopyCtx(true), mountPath+"/src/report", mountPath+"/dst")
	if err != nil {
		t.Fatalf("Move() error = %v", err)
	}
	if got := mustRead(t, filepath.Join(rootDir, "dst", "report")); got != "moved content" {
		t.Fatalf("destination content = %q, want overwritten with %q (move ignores skip_existing)", got, "moved content")
	}
	if _, statErr := os.Stat(filepath.Join(rootDir, "src", "report")); !os.IsNotExist(statErr) {
		t.Fatalf("move source still exists or stat failed: %v", statErr)
	}
}

func TestShouldSkipExistingFile(t *testing.T) {
	file5 := &model.Object{Name: "f", Size: 5}
	file9 := &model.Object{Name: "f", Size: 9}
	dir5 := &model.Object{Name: "d", Size: 5, IsFolder: true}

	cases := []struct {
		name     string
		src, dst model.Obj
		want     bool
	}{
		{"same size files", file5, &model.Object{Name: "f", Size: 5}, true},
		{"different size files", file5, file9, false},
		{"src is dir", dir5, &model.Object{Name: "f", Size: 5}, false},
		{"dst is dir", file5, dir5, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldSkipExistingFile(tc.src, tc.dst); got != tc.want {
				t.Fatalf("shouldSkipExistingFile() = %v, want %v", got, tc.want)
			}
		})
	}
}
