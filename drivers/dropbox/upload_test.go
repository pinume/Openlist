package dropbox

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/stream"
)

func TestPutUsesSingleRequestForSmallFiles(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/2/files/upload" {
			t.Errorf("path = %q, want /2/files/upload", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("Dropbox-API-Arg"); !strings.Contains(got, `"path":"/target/file.txt"`) {
			t.Errorf("Dropbox-API-Arg = %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != "hello" {
			t.Errorf("body = %q, want hello", body)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	httpClient := base.HttpClient
	base.HttpClient = server.Client()
	t.Cleanup(func() {
		base.HttpClient = httpClient
	})

	driver := &Dropbox{contentBase: server.URL}
	driver.AccessToken = "token"
	upload := &stream.FileStream{
		Ctx:    context.Background(),
		Obj:    &model.Object{Name: "file.txt", Size: 5},
		Reader: bytes.NewBufferString("hello"),
	}
	var progress float64
	err := driver.Put(
		context.Background(),
		&model.Object{Path: "/target", IsFolder: true},
		upload,
		func(value float64) { progress = value },
	)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
	if progress != 100 {
		t.Fatalf("progress = %v, want 100", progress)
	}
}

func TestPutReturnsDropboxContentError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "quota exceeded", http.StatusInsufficientStorage)
	}))
	defer server.Close()
	httpClient := base.HttpClient
	base.HttpClient = server.Client()
	t.Cleanup(func() {
		base.HttpClient = httpClient
	})

	driver := &Dropbox{contentBase: server.URL}
	upload := &stream.FileStream{
		Ctx:    context.Background(),
		Obj:    &model.Object{Name: "file.txt", Size: 5},
		Reader: bytes.NewBufferString("hello"),
	}
	err := driver.Put(
		context.Background(),
		&model.Object{Path: "/target", IsFolder: true},
		upload,
		func(float64) {},
	)
	if err == nil || !strings.Contains(err.Error(), "quota exceeded") {
		t.Fatalf("error = %v, want Dropbox response body", err)
	}
}

func TestUploadSessionSendsDataInStartAppendAndFinish(t *testing.T) {
	var paths []string
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, r.URL.Path)
		bodies = append(bodies, string(body))
		if r.URL.Path == "/2/files/upload_session/start" {
			_, _ = io.WriteString(w, `{"session_id":"session"}`)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	httpClient := base.HttpClient
	base.HttpClient = server.Client()
	t.Cleanup(func() {
		base.HttpClient = httpClient
	})

	driver := &Dropbox{contentBase: server.URL}
	driver.AccessToken = "token"
	upload := &stream.FileStream{
		Ctx:    context.Background(),
		Obj:    &model.Object{Name: "file.txt", Size: 10},
		Reader: bytes.NewBufferString("0123456789"),
	}
	var progress float64
	err := driver.uploadSession(
		context.Background(),
		"/target/file.txt",
		upload,
		func(value float64) { progress = value },
		4,
	)
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := []string{
		"/2/files/upload_session/start",
		"/2/files/upload_session/append_v2",
		"/2/files/upload_session/finish",
	}
	if !slices.Equal(paths, wantPaths) {
		t.Fatalf("paths = %#v, want %#v", paths, wantPaths)
	}
	if got := strings.Join(bodies, ""); got != "0123456789" {
		t.Fatalf("uploaded body = %q, want complete source", got)
	}
	if progress != 100 {
		t.Fatalf("progress = %v, want 100", progress)
	}
}
