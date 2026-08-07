package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/gin-gonic/gin"
)

// TestStoragesLoadedBlocksUntilReloadSignal pins the contract that the
// storage management UI relies on after /admin/storage/load_all: while a
// reload is in flight (signal reset but not yet sent), every API request
// blocks server-side until the reload completes. An immediate list refresh
// after load_all therefore cannot observe a stale reload.
func TestStoragesLoadedBlocksUntilReloadSignal(t *testing.T) {
	conf.Conf = conf.DefaultConfig(t.TempDir())
	conf.ResetStoragesLoadSignal()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(StoragesLoaded)
	r.GET("/api/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	type result struct {
		status  int
		elapsed time.Duration
	}
	done := make(chan result, 1)
	go func() {
		start := time.Now()
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		done <- result{status: w.Code, elapsed: time.Since(start)}
	}()

	select {
	case res := <-done:
		t.Fatalf("request completed before reload signal: status=%d elapsed=%v", res.status, res.elapsed)
	case <-time.After(150 * time.Millisecond):
		// still blocked, as expected; now complete the reload
	}

	conf.SendStoragesLoadedSignal()

	select {
	case res := <-done:
		if res.status != http.StatusOK {
			t.Fatalf("request status = %d, want 200", res.status)
		}
		if res.elapsed < 150*time.Millisecond {
			t.Fatalf("request did not block until signal: elapsed=%v", res.elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("request did not complete after reload signal")
	}

	// restore the package-level default state for other tests
	conf.ResetStoragesLoadSignal()
}
