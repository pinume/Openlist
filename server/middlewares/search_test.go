package middlewares

import (
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

var searchIndexTestDBOnce sync.Once

func setupSearchIndexTestDB(t *testing.T) {
	t.Helper()
	searchIndexTestDBOnce.Do(func() {
		testDB, err := gorm.Open(sqlite.Open("file:middlewares_search_index?mode=memory&cache=shared"))
		if err != nil {
			t.Fatalf("open search index test database: %v", err)
		}
		db.Init(testDB)
	})
}

func setSearchIndexMode(t *testing.T, mode string) {
	t.Helper()
	if err := op.SaveSettingItem(&model.SettingItem{
		Key:   conf.SearchIndex,
		Value: mode,
		Type:  conf.TypeSelect,
	}); err != nil {
		t.Fatalf("set search_index: %v", err)
	}
}

func runMiddleware(mw gin.HandlerFunc) (nextCalled bool, recorder *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/probe", mw, func(c *gin.Context) {
		nextCalled = true
		c.Status(http.StatusNoContent)
	})
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/probe", nil))
	return nextCalled, recorder
}

func TestSearchIndexAllowsNoIndexMode(t *testing.T) {
	setupSearchIndexTestDB(t)
	setSearchIndexMode(t, "no_index")

	nextCalled, recorder := runMiddleware(SearchIndex)
	if !nextCalled {
		t.Fatal("search request was blocked in no_index mode")
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
}

func TestSearchIndexBlocksNoneMode(t *testing.T) {
	setupSearchIndexTestDB(t)
	setSearchIndexMode(t, "none")

	nextCalled, recorder := runMiddleware(SearchIndex)
	if nextCalled {
		t.Fatal("search request reached the handler while search_index = none")
	}
	if !strings.Contains(recorder.Body.String(), `"code":404`) {
		t.Fatalf("body = %s, want code 404", recorder.Body.String())
	}
}

func TestIndexManageBlocksBothNoneAndNoIndex(t *testing.T) {
	setupSearchIndexTestDB(t)
	for _, mode := range []string{"none", "no_index"} {
		t.Run(mode, func(t *testing.T) {
			setSearchIndexMode(t, mode)
			nextCalled, recorder := runMiddleware(IndexManage)
			if nextCalled {
				t.Fatalf("index management request reached the handler while search_index = %s", mode)
			}
			if !strings.Contains(recorder.Body.String(), `"code":404`) {
				t.Fatalf("body = %s, want code 404", recorder.Body.String())
			}
		})
	}
}

func TestIndexManageAllowsDatabaseMode(t *testing.T) {
	setupSearchIndexTestDB(t)
	setSearchIndexMode(t, "database")

	nextCalled, recorder := runMiddleware(IndexManage)
	if !nextCalled {
		t.Fatal("index management request was blocked in database mode")
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
}
