//go:build linux

package fs

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// TestCopySkipExistingDoesNotBlockOnNamedPipe guards against skip_existing
// forcing same-storage copies through the per-object FileTransferTask,
// which opens the source via op.Link before comparing sizes. Opening a
// FIFO for read blocks until a writer shows up, so a copy of a named pipe
// must go through the driver's native Copy (which special-cases FIFOs)
// whenever there is nothing to skip.
func TestCopySkipExistingDoesNotBlockOnNamedPipe(t *testing.T) {
	mountPath, rootDir := setupCopyMoveLocalStorage(t)
	if err := os.MkdirAll(filepath.Join(rootDir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(filepath.Join(rootDir, "src", "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(rootDir, "dst"), 0o755); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := Copy(syncCopyCtx(true), mountPath+"/src/pipe", mountPath+"/dst")
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Copy() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		// If the regression this test guards against reappears, the
		// goroutine above is parked in a blocking read-open of the FIFO
		// while holding Local's rootMu.RLock. Unblock it by opening the
		// write end before failing, so the goroutine (and the lock it
		// holds) doesn't outlive the test and hang t.Cleanup's storage
		// teardown, which needs rootMu's write lock.
		if writer, werr := os.OpenFile(filepath.Join(rootDir, "src", "pipe"), os.O_WRONLY, 0); werr == nil {
			_ = writer.Close()
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
		t.Fatal("Copy() blocked opening a named pipe while skip_existing was set")
	}

	info, err := os.Lstat(filepath.Join(rootDir, "dst", "pipe"))
	if err != nil {
		t.Fatalf("stat copied pipe: %v", err)
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("copied dst/pipe mode = %v, want a named pipe", info.Mode())
	}
}
