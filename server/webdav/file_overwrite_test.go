package webdav

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/OpenListTeam/OpenList/v4/drivers"
	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

func TestCopyAndMoveHonorOverwriteAtExactDestination(t *testing.T) {
	setupPermissionTestDB(t)
	if conf.Conf == nil {
		conf.Conf = conf.DefaultConfig(t.TempDir())
	}
	rootDir := t.TempDir()
	for _, dir := range []string{"a", "b"} {
		if err := os.Mkdir(filepath.Join(rootDir, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(rootDir, "a", "foo"), "source")
	writeTestFile(t, filepath.Join(rootDir, "a", "bar"), "source-name-collision")
	writeTestFile(t, filepath.Join(rootDir, "b", "foo"), "destination-name-collision")
	writeTestFile(t, filepath.Join(rootDir, "b", "bar"), "old-target")

	addition, err := json.Marshal(map[string]any{
		"root_folder_path": rootDir,
		"show_hidden":      true,
		"recycle_bin_path": ".trash",
	})
	if err != nil {
		t.Fatal(err)
	}
	const mountPath = "/webdav-overwrite-local"
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

	user := &model.User{ID: 1, Permission: 0x31ff}
	ctx := context.WithValue(context.Background(), conf.UserKey, user)
	src := mountPath + "/a/foo"
	dst := mountPath + "/b/bar"

	status, err := copyFiles(ctx, src, dst, false)
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusPreconditionFailed {
		t.Fatalf("copy without overwrite status = %d, want %d", status, http.StatusPreconditionFailed)
	}
	assertTestFile(t, filepath.Join(rootDir, "b", "bar"), "old-target")

	status, err = copyFiles(ctx, src, dst, true)
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusNoContent {
		t.Fatalf("copy with overwrite status = %d, want %d", status, http.StatusNoContent)
	}
	assertTestFile(t, filepath.Join(rootDir, "b", "bar"), "source")
	assertTestFile(t, filepath.Join(rootDir, "b", "foo"), "destination-name-collision")
	assertTestFile(t, filepath.Join(rootDir, "a", "bar"), "source-name-collision")

	writeTestFile(t, filepath.Join(rootDir, "b", "bar"), "old-target-again")
	status, err = moveFiles(ctx, src, dst, true)
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusNoContent {
		t.Fatalf("move with overwrite status = %d, want %d", status, http.StatusNoContent)
	}
	assertTestFile(t, filepath.Join(rootDir, "b", "bar"), "source")
	assertTestFile(t, filepath.Join(rootDir, "b", "foo"), "destination-name-collision")
	if _, err := os.Stat(filepath.Join(rootDir, "a", "foo")); !os.IsNotExist(err) {
		t.Fatalf("move source still exists or stat failed: %v", err)
	}
	if err := filepath.WalkDir(rootDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(entry.Name(), ".tinylist-webdav-") {
			t.Fatalf("WebDAV backup was not permanently removed: %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertTestFile(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != want {
		t.Fatalf("%s = %q, want %q", path, content, want)
	}
}
