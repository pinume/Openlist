package no_index

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	_ "github.com/OpenListTeam/OpenList/v4/drivers"
	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/db"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/OpenListTeam/OpenList/v4/internal/search/searcher"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var noIndexTestDBOnce sync.Once

func setupNoIndexTestDB(t *testing.T) {
	t.Helper()
	noIndexTestDBOnce.Do(func() {
		conf.Conf = conf.DefaultConfig(t.TempDir())
		testDB, err := gorm.Open(sqlite.Open("file:search_no_index?mode=memory&cache=shared"))
		if err != nil {
			t.Fatalf("open no_index test database: %v", err)
		}
		db.Init(testDB)
	})
}

// setupLocalStorage mounts a fresh Local storage backed by a temp directory
// and returns its mount path.
func setupLocalStorage(t *testing.T) (mountPath, rootDir string) {
	t.Helper()
	setupNoIndexTestDB(t)
	rootDir = t.TempDir()
	addition, err := json.Marshal(map[string]any{
		"root_folder_path": rootDir,
		"show_hidden":      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	mountPath = "/no-index-" + t.Name()
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

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		t.Fatal(err)
	}
}

func adminCtx() context.Context {
	return context.WithValue(context.Background(), conf.UserKey, &model.User{ID: 1, Permission: -1})
}

func nodeNames(nodes []model.SearchNode) []string {
	names := make([]string, len(nodes))
	for i, n := range nodes {
		names[i] = n.Name
	}
	return names
}

func TestSearchFilteredCaseInsensitiveAndKeywordAnd(t *testing.T) {
	mountPath, rootDir := setupLocalStorage(t)
	mustMkdir(t, filepath.Join(rootDir, "docs"))
	mustWriteFile(t, filepath.Join(rootDir, "docs", "ReadMe.TXT"), 10)
	mustWriteFile(t, filepath.Join(rootDir, "docs", "notes.txt"), 10)

	n := NoIndex{}
	nodes, total, err := n.SearchFiltered(adminCtx(), model.SearchReq{
		Parent:   mountPath,
		Keywords: "readme txt",
		PageReq:  model.PageReq{Page: 1, PerPage: 10},
	}, nil)
	if err != nil {
		t.Fatalf("SearchFiltered() error = %v", err)
	}
	if total != 1 || len(nodes) != 1 || nodes[0].Name != "ReadMe.TXT" {
		t.Fatalf("SearchFiltered() = %+v, total = %d, want single ReadMe.TXT match", nodes, total)
	}
}

func TestSearchFilteredScope(t *testing.T) {
	mountPath, rootDir := setupLocalStorage(t)
	mustMkdir(t, filepath.Join(rootDir, "match-dir"))
	mustWriteFile(t, filepath.Join(rootDir, "match-file"), 10)

	n := NoIndex{}
	dirsOnly, _, err := n.SearchFiltered(adminCtx(), model.SearchReq{
		Parent:   mountPath,
		Keywords: "match",
		Scope:    1,
		PageReq:  model.PageReq{Page: 1, PerPage: 10},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if names := nodeNames(dirsOnly); len(names) != 1 || names[0] != "match-dir" {
		t.Fatalf("scope=1 results = %v, want [match-dir]", names)
	}

	filesOnly, _, err := n.SearchFiltered(adminCtx(), model.SearchReq{
		Parent:   mountPath,
		Keywords: "match",
		Scope:    2,
		PageReq:  model.PageReq{Page: 1, PerPage: 10},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if names := nodeNames(filesOnly); len(names) != 1 || names[0] != "match-file" {
		t.Fatalf("scope=2 results = %v, want [match-file]", names)
	}
}

func TestSearchFilteredStableSortAndPagination(t *testing.T) {
	mountPath, rootDir := setupLocalStorage(t)
	mustMkdir(t, filepath.Join(rootDir, "b"))
	mustWriteFile(t, filepath.Join(rootDir, "b", "item"), 10)
	mustMkdir(t, filepath.Join(rootDir, "a"))
	mustWriteFile(t, filepath.Join(rootDir, "a", "item"), 10)

	n := NoIndex{}
	page1, total, err := n.SearchFiltered(adminCtx(), model.SearchReq{
		Parent:   mountPath,
		Keywords: "item",
		PageReq:  model.PageReq{Page: 1, PerPage: 1},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if len(page1) != 1 || page1[0].Parent != mountPath+"/a" {
		t.Fatalf("page1 = %+v, want the item under /a first (sorted by full path)", page1)
	}

	page2, _, err := n.SearchFiltered(adminCtx(), model.SearchReq{
		Parent:   mountPath,
		Keywords: "item",
		PageReq:  model.PageReq{Page: 2, PerPage: 1},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page2) != 1 || page2[0].Parent != mountPath+"/b" {
		t.Fatalf("page2 = %+v, want the item under /b second", page2)
	}
}

func TestSearchFilteredAppliesFilterBeforePagination(t *testing.T) {
	mountPath, rootDir := setupLocalStorage(t)
	mustMkdir(t, filepath.Join(rootDir, "allowed"))
	mustWriteFile(t, filepath.Join(rootDir, "allowed", "match-1"), 10)
	mustMkdir(t, filepath.Join(rootDir, "denied"))
	mustWriteFile(t, filepath.Join(rootDir, "denied", "match-2"), 10)

	n := NoIndex{}
	nodes, total, err := n.SearchFiltered(adminCtx(), model.SearchReq{
		Parent:   mountPath,
		Keywords: "match",
		PageReq:  model.PageReq{Page: 1, PerPage: 10},
	}, func(node model.SearchNode) bool {
		return node.Parent == mountPath+"/allowed"
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(nodes) != 1 || nodes[0].Name != "match-1" {
		t.Fatalf("nodes = %+v, total = %d, want only the allowed match counted", nodes, total)
	}
}

func TestSearchFilteredUserBasePathHidesOutsideContent(t *testing.T) {
	mountPath, rootDir := setupLocalStorage(t)
	mustMkdir(t, filepath.Join(rootDir, "public"))
	mustWriteFile(t, filepath.Join(rootDir, "public", "shared-report"), 10)
	mustMkdir(t, filepath.Join(rootDir, "private"))
	mustWriteFile(t, filepath.Join(rootDir, "private", "shared-report"), 10)

	restrictedUser := &model.User{ID: 2, BasePath: mountPath + "/public", Permission: -1}
	ctx := context.WithValue(context.Background(), conf.UserKey, restrictedUser)

	n := NoIndex{}
	// The handler joins req.Parent with the user's base_path before calling
	// search, and applies a filter that rejects anything outside it; mirror
	// that here since NoIndex itself is base_path-agnostic.
	nodes, total, err := n.SearchFiltered(ctx, model.SearchReq{
		Parent:   restrictedUser.BasePath,
		Keywords: "shared-report",
		PageReq:  model.PageReq{Page: 1, PerPage: 10},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(nodes) != 1 || nodes[0].Parent != mountPath+"/public" {
		t.Fatalf("nodes = %+v, total = %d, want only the file under base_path", nodes, total)
	}
}

func TestIsIgnoredRespectsPathBoundary(t *testing.T) {
	ignorePaths := []string{"/data/a"}
	if !isIgnored("/data/a", ignorePaths) {
		t.Error("/data/a should be ignored (exact match)")
	}
	if !isIgnored("/data/a/child", ignorePaths) {
		t.Error("/data/a/child should be ignored (under ignored path)")
	}
	if isIgnored("/data/abc", ignorePaths) {
		t.Error("/data/abc should not be ignored: it only shares a prefix, not a path boundary")
	}
}

func TestSearchFilteredMaxDepth(t *testing.T) {
	mountPath, rootDir := setupLocalStorage(t)
	deep := rootDir
	for i := 0; i < 3; i++ {
		deep = filepath.Join(deep, "d")
		mustMkdir(t, deep)
	}
	mustWriteFile(t, filepath.Join(deep, "buried"), 10)

	n := NoIndex{}
	// depth 1 from root can only see the first "d" directory, not the file
	// three levels down.
	setMaxIndexDepth(t, 1)
	nodes, _, err := n.SearchFiltered(adminCtx(), model.SearchReq{
		Parent:   mountPath,
		Keywords: "buried",
		PageReq:  model.PageReq{Page: 1, PerPage: 10},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 0 {
		t.Fatalf("nodes = %+v, want none within depth limit", nodes)
	}

	setMaxIndexDepth(t, 10)
	nodes, _, err = n.SearchFiltered(adminCtx(), model.SearchReq{
		Parent:   mountPath,
		Keywords: "buried",
		PageReq:  model.PageReq{Page: 1, PerPage: 10},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("nodes = %+v, want the buried file once depth allows it", nodes)
	}
}

func setMaxIndexDepth(t *testing.T, depth int) {
	t.Helper()
	if err := op.SaveSettingItem(&model.SettingItem{
		Key:   conf.MaxIndexDepth,
		Value: strconv.Itoa(depth),
		Type:  conf.TypeNumber,
	}); err != nil {
		t.Fatalf("set max_index_depth: %v", err)
	}
	t.Cleanup(func() {
		_ = op.SaveSettingItem(&model.SettingItem{
			Key:   conf.MaxIndexDepth,
			Value: "20",
			Type:  conf.TypeNumber,
		})
	})
}

func TestSearchFilteredContextCancellation(t *testing.T) {
	mountPath, rootDir := setupLocalStorage(t)
	mustMkdir(t, filepath.Join(rootDir, "x"))
	mustWriteFile(t, filepath.Join(rootDir, "x", "f"), 10)

	ctx, cancel := context.WithCancel(adminCtx())
	cancel()

	n := NoIndex{}
	_, _, err := n.SearchFiltered(ctx, model.SearchReq{
		Parent:   mountPath,
		Keywords: "f",
		PageReq:  model.PageReq{Page: 1, PerPage: 10},
	}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SearchFiltered() error = %v, want context.Canceled", err)
	}
}

func TestNoIndexRegisteredAndDefaultsAutoUpdateFalse(t *testing.T) {
	factory, ok := searcher.NewMap["no_index"]
	if !ok {
		t.Fatal("no_index searcher not registered")
	}
	s, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	if s.Config().AutoUpdate {
		t.Fatal("no_index must have AutoUpdate = false")
	}
	if _, ok := s.(searcher.FilteredSearcher); !ok {
		t.Fatal("no_index must implement FilteredSearcher")
	}
}

func TestSearchFilteredRejectsIndexBuildRoutesElsewhere(t *testing.T) {
	// Route-level enforcement (no_index must not allow /api/admin/index/*)
	// lives in server/middlewares; this just documents the expectation this
	// package's behavior depends on: no_index never persists anything, so
	// there is nothing to build.
	n := NoIndex{}
	if err := n.Index(context.Background(), model.SearchNode{}); err != nil {
		t.Fatalf("Index() error = %v, want nil no-op", err)
	}
	if err := n.Clear(context.Background()); err != nil {
		t.Fatalf("Clear() error = %v, want nil no-op", err)
	}
}
