package local

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/stream"
)

func TestLocalRootRejectsEscapingSymlink(t *testing.T) {
	rootDir := t.TempDir()
	outsideDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outsideDir, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideDir, filepath.Join(rootDir, "outside")); err != nil {
		t.Skipf("symlink is unavailable: %v", err)
	}
	localDriver := newTestLocal(t, rootDir, true)

	objs, err := localDriver.List(
		context.Background(),
		&model.Object{Path: ".", IsFolder: true},
		model.ListArgs{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 1 {
		t.Fatalf("List() returned %d objects, want 1", len(objs))
	}
	if objs[0].IsDir() {
		t.Fatal("escaping directory symlink was exposed as a traversable directory")
	}
	if _, err := localDriver.Get(context.Background(), "/outside/secret.txt"); err == nil {
		t.Fatal("Get() followed a symlink outside the storage root")
	}
}

func TestLocalRootAllowsRelativeSymlinkWithinRoot(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(rootDir, "target"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "target", "file.txt"), []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target", filepath.Join(rootDir, "alias")); err != nil {
		t.Skipf("symlink is unavailable: %v", err)
	}
	localDriver := newTestLocal(t, rootDir, false)
	obj, err := localDriver.Get(context.Background(), "/alias/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if obj.GetSize() != int64(len("inside")) {
		t.Fatalf("symlinked file size = %d, want %d", obj.GetSize(), len("inside"))
	}
}

func TestDirectorySizeStopsAtSymlinkCycle(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(rootDir, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "child", "file.txt"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("..", filepath.Join(rootDir, "child", "back")); err != nil {
		t.Skipf("symlink is unavailable: %v", err)
	}
	localDriver := newTestLocal(t, rootDir, true)
	rootObj, err := localDriver.GetRoot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := rootObj.GetSize(), int64(len("data")); got != want {
		t.Fatalf("root size with symlink cycle = %d, want %d", got, want)
	}
}

func TestDirectorySizeStaysConsistentAcrossMutations(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootDir, "parent", "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "parent", "child", "one.txt"), []byte("12345"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, ".hidden"), []byte("hidden"), 0o600); err != nil {
		t.Fatal(err)
	}
	localDriver := newTestLocal(t, rootDir, true)
	assertLocalRootSize(t, localDriver, rootDir)

	if _, err := localDriver.List(
		context.Background(),
		&model.Object{Path: "parent", IsFolder: true},
		model.ListArgs{Refresh: true},
	); err != nil {
		t.Fatal(err)
	}
	assertLocalRootSize(t, localDriver, rootDir)

	upload := &stream.FileStream{
		Ctx:    context.Background(),
		Obj:    &model.Object{Name: "two.txt", Size: 3},
		Reader: bytes.NewBufferString("two"),
	}
	if err := localDriver.Put(
		context.Background(),
		&model.Object{Path: "parent", IsFolder: true},
		upload,
		func(float64) {},
	); err != nil {
		t.Fatal(err)
	}
	assertLocalRootSize(t, localDriver, rootDir)

	if err := localDriver.Remove(context.Background(), &model.Object{
		Path: "parent/two.txt",
		Name: "two.txt",
	}); err != nil {
		t.Fatal(err)
	}
	assertLocalRootSize(t, localDriver, rootDir)

	if err := localDriver.Rename(context.Background(), &model.Object{
		Path:     "parent/child",
		Name:     "child",
		IsFolder: true,
	}, "renamed"); err != nil {
		t.Fatal(err)
	}
	assertLocalRootSize(t, localDriver, rootDir)

	if _, err := localDriver.List(
		context.Background(),
		&model.Object{Path: "parent", IsFolder: true},
		model.ListArgs{Refresh: true},
	); err != nil {
		t.Fatal(err)
	}
	assertLocalRootSize(t, localDriver, rootDir)
}

func TestPutPreservesExistingZeroMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not preserve Unix permission bits")
	}
	rootDir := t.TempDir()
	dstPath := filepath.Join(rootDir, "locked.txt")
	if err := os.WriteFile(dstPath, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dstPath, 0); err != nil {
		t.Fatal(err)
	}
	localDriver := newTestLocal(t, rootDir, false)
	upload := &stream.FileStream{
		Ctx:    context.Background(),
		Obj:    &model.Object{Name: "locked.txt", Size: 3},
		Reader: bytes.NewBufferString("new"),
	}
	if err := localDriver.Put(
		context.Background(),
		&model.Object{Path: ".", IsFolder: true},
		upload,
		func(float64) {},
	); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0 {
		t.Fatalf("destination mode = %o, want 0", got)
	}
}

func assertLocalRootSize(t *testing.T, localDriver *Local, rootDir string) {
	t.Helper()
	want, err := walkFileBytes(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	rootObj, err := localDriver.GetRoot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := rootObj.GetSize(); got != want {
		t.Fatalf("root size = %d, want %d", got, want)
	}
}

func walkFileBytes(rootDir string) (int64, error) {
	var total int64
	err := filepath.WalkDir(rootDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	return total, err
}
