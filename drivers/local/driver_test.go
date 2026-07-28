package local

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/OpenListTeam/OpenList/v4/internal/errs"
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

func TestDirectoryRefreshReusesCachedDescendants(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootDir, "parent", "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "parent", "child", "file.txt"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	localDriver := newTestLocal(t, rootDir, true)

	var reads []string
	localDriver.directoryMap.mu.Lock()
	localDriver.directoryMap.readDirFn = func(root *os.Root, path string) ([]os.FileInfo, error) {
		reads = append(reads, path)
		return readDir(root, path)
	}
	localDriver.directoryMap.mu.Unlock()

	if err := os.WriteFile(filepath.Join(rootDir, "new.txt"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	localDriver.refreshDirectory(".")

	if len(reads) != 1 || reads[0] != "." {
		t.Fatalf("Refresh() read directories %v, want only the storage root", reads)
	}
	assertLocalRootSize(t, localDriver, rootDir)
}

func TestCopyToExistingFileFallsBackWithoutTruncating(t *testing.T) {
	rootDir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(rootDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("source.txt", "source")
	write("destination.txt", "destination")
	localDriver := newTestLocal(t, rootDir, false)

	err := localDriver.CopyTo(
		context.Background(),
		&model.Object{Path: "source.txt", Name: "source.txt"},
		&model.Object{Path: ".", IsFolder: true},
		"destination.txt",
	)
	if !errors.Is(err, errs.NotImplement) {
		t.Fatalf("CopyTo() error = %v, want NotImplement fallback", err)
	}
	content, readErr := os.ReadFile(filepath.Join(rootDir, "destination.txt"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(content) != "destination" {
		t.Fatalf("destination changed before fallback: %q", content)
	}
}

func TestLocalRejectsInvalidTransferNamesAndAbsoluteSymlinkCopy(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "source.txt"), []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	localDriver := newTestLocal(t, rootDir, false)
	src := &model.Object{Path: "source.txt", Name: "source.txt"}
	dst := &model.Object{Path: ".", IsFolder: true}
	for _, name := range []string{"", ".", "..", "nested/name"} {
		if err := localDriver.CopyTo(context.Background(), src, dst, name); !errors.Is(err, errs.PermissionDenied) {
			t.Fatalf("CopyTo(..., %q) error = %v, want permission denied", name, err)
		}
		if err := localDriver.MoveTo(context.Background(), src, dst, name); !errors.Is(err, errs.PermissionDenied) {
			t.Fatalf("MoveTo(..., %q) error = %v, want permission denied", name, err)
		}
	}

	absoluteTarget := filepath.Join(rootDir, "source.txt")
	if err := os.Symlink(absoluteTarget, filepath.Join(rootDir, "absolute-link")); err != nil {
		t.Skipf("symlink is unavailable: %v", err)
	}
	err := localDriver.CopyTo(
		context.Background(),
		&model.Object{Path: "absolute-link", Name: "absolute-link"},
		dst,
		"copied-link",
	)
	if err == nil || !strings.Contains(err.Error(), "absolute target") {
		t.Fatalf("CopyTo() absolute symlink error = %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(rootDir, "copied-link")); !os.IsNotExist(statErr) {
		t.Fatalf("absolute symlink copy created a destination: %v", statErr)
	}
}

func TestLocalOperationAfterDropReturnsStorageNotInit(t *testing.T) {
	rootDir := t.TempDir()
	localDriver := newTestLocal(t, rootDir, false)
	if err := localDriver.Drop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := localDriver.GetRoot(context.Background()); !errors.Is(err, errs.StorageNotInit) {
		t.Fatalf("GetRoot() error = %v, want storage not init", err)
	}
}

func TestLocalRootLifecycleIsSafeDuringList(t *testing.T) {
	rootDir := t.TempDir()
	localDriver := newTestLocal(t, rootDir, false)
	var wg sync.WaitGroup
	errsCh := make(chan error, 100)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 100 {
			_, err := localDriver.List(
				context.Background(),
				&model.Object{Path: ".", IsFolder: true},
				model.ListArgs{},
			)
			if err != nil && !errors.Is(err, errs.StorageNotInit) {
				errsCh <- err
			}
		}
	}()
	for range 20 {
		if err := localDriver.Drop(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err := localDriver.Init(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
	close(errsCh)
	for err := range errsCh {
		t.Fatalf("List() during root reload returned unexpected error: %v", err)
	}
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
