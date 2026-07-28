package local

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/stream"
)

func newTestLocal(t *testing.T, dir string, directorySize bool) *Local {
	t.Helper()
	driver := &Local{
		Addition: Addition{
			RootPath:      driver.RootPath{RootFolderPath: dir},
			DirectorySize: directorySize,
			ShowHidden:    true,
		},
	}
	if err := driver.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := driver.Drop(context.Background()); err != nil {
			t.Errorf("drop local driver: %v", err)
		}
	})
	return driver
}

func TestPutAtomicallyReplacesDestination(t *testing.T) {
	dir := t.TempDir()
	dstPath := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(dstPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	upload := &stream.FileStream{
		Ctx:    context.Background(),
		Obj:    &model.Object{Name: "file.txt", Size: 3},
		Reader: bytes.NewBufferString("new"),
	}

	driver := newTestLocal(t, dir, false)
	err := driver.Put(
		context.Background(),
		&model.Object{Path: ".", IsFolder: true},
		upload,
		func(float64) {},
	)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("destination = %q, want new", got)
	}
	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("destination mode = %o, want 644", info.Mode().Perm())
	}
	assertNoUploadTemps(t, dir)
}

func TestPutFailurePreservesDestination(t *testing.T) {
	dir := t.TempDir()
	dstPath := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(dstPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	upload := &stream.FileStream{
		Ctx: context.Background(),
		Obj: &model.Object{Name: "file.txt", Size: 6},
		Reader: io.MultiReader(
			bytes.NewBufferString("new"),
			errorReader{},
		),
	}

	driver := newTestLocal(t, dir, false)
	err := driver.Put(
		context.Background(),
		&model.Object{Path: ".", IsFolder: true},
		upload,
		func(float64) {},
	)
	if err == nil {
		t.Fatal("Put() error = nil, want read failure")
	}
	got, readErr := os.ReadFile(dstPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "old" {
		t.Fatalf("destination = %q, want preserved old content", got)
	}
	assertNoUploadTemps(t, dir)
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

func assertNoUploadTemps(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".tinylist-upload-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary upload files remain: %v", matches)
	}
}
