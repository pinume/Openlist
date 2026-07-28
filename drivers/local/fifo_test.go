//go:build linux

package local

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"golang.org/x/sys/unix"
)

func TestListAndGetDoNotBlockOnNamedPipe(t *testing.T) {
	rootDir := t.TempDir()
	if err := unix.Mkfifo(filepath.Join(rootDir, "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	localDriver := newTestLocal(t, rootDir, false)

	done := make(chan error, 1)
	go func() {
		_, err := localDriver.List(
			context.Background(),
			&model.Object{Path: ".", IsFolder: true},
			model.ListArgs{},
		)
		if err == nil {
			_, err = localDriver.Get(context.Background(), "pipe")
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("List/Get blocked while opening a named pipe")
	}
}
