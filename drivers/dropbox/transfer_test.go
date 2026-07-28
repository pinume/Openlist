package dropbox

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	localdriver "github.com/OpenListTeam/OpenList/v4/drivers/local"
	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/stream"
	"github.com/go-resty/resty/v2"
)

func TestCopyFromLocalToDropboxStreamsCompleteFile(t *testing.T) {
	const content = "local to dropbox"
	sourcePath := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(sourcePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var uploaded string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2/files/upload" {
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		uploaded = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	useDropboxTestClients(t, server)

	source := &model.Object{
		Path: filepath.Base(sourcePath),
		Name: filepath.Base(sourcePath),
		Size: int64(len(content)),
	}
	local := initializedLocal(t, filepath.Dir(sourcePath))
	link, err := local.Link(context.Background(), source, model.LinkArgs{})
	if err != nil {
		t.Fatal(err)
	}
	sourceStream, err := stream.NewSeekableStream(&stream.FileStream{
		Obj: source,
		Ctx: context.Background(),
	}, link)
	if err != nil {
		t.Fatal(err)
	}
	defer sourceStream.Close()

	dropbox := &Dropbox{contentBase: server.URL}
	err = dropbox.Put(
		context.Background(),
		&model.Object{Path: "/target", IsFolder: true},
		sourceStream,
		func(float64) {},
	)
	if err != nil {
		t.Fatal(err)
	}
	if uploaded != content {
		t.Fatalf("uploaded content = %q, want %q", uploaded, content)
	}
}

func TestCopyFromDropboxToLocalStreamsCompleteFile(t *testing.T) {
	const content = "dropbox to local"
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/2/files/get_temporary_link":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"link":%q}`, server.URL+"/content")
		case "/content":
			w.Header().Set("Content-Length", fmt.Sprint(len(content)))
			_, _ = io.WriteString(w, content)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	useDropboxTestClients(t, server)

	source := &model.Object{
		Path: "/source.txt",
		Name: "source.txt",
		Size: int64(len(content)),
	}
	dropbox := &Dropbox{base: server.URL}
	link, err := dropbox.Link(context.Background(), source, model.LinkArgs{})
	if err != nil {
		t.Fatal(err)
	}
	sourceStream, err := stream.NewSeekableStream(&stream.FileStream{
		Obj: source,
		Ctx: context.Background(),
	}, link)
	if err != nil {
		t.Fatal(err)
	}

	destination := t.TempDir()
	local := initializedLocal(t, destination)
	err = local.Put(
		context.Background(),
		&model.Object{Path: ".", IsFolder: true},
		sourceStream,
		func(float64) {},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := sourceStream.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(destination, source.GetName()))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Fatalf("copied content = %q, want %q", got, content)
	}
}

func initializedLocal(t *testing.T, root string) *localdriver.Local {
	t.Helper()
	local := &localdriver.Local{}
	local.SetRootPath(root)
	if err := local.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := local.Drop(context.Background()); err != nil {
			t.Errorf("drop local driver: %v", err)
		}
	})
	return local
}

func useDropboxTestClients(t *testing.T, server *httptest.Server) {
	t.Helper()
	httpClient := base.HttpClient
	restyClient := base.RestyClient
	config := conf.Conf
	if conf.Conf == nil {
		conf.Conf = conf.DefaultConfig(t.TempDir())
	}
	base.HttpClient = server.Client()
	base.RestyClient = resty.New()
	t.Cleanup(func() {
		base.HttpClient = httpClient
		base.RestyClient = restyClient
		conf.Conf = config
	})
}
