package gitlab_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/AdamShannag/volare/pkg/fetcher/gitlab"
	"github.com/AdamShannag/volare/pkg/types"
)

type mockDownloader struct {
	calls    int32
	lastURL  string
	lastDest string
	headers  map[string]string
	err      error
}

func (m *mockDownloader) Download(_ context.Context, url string, headers map[string]string, dest string) error {
	atomic.AddInt32(&m.calls, 1)
	m.lastURL = url
	m.lastDest = dest
	m.headers = headers
	return m.err
}

func TestFetcher_Fetch_Success(t *testing.T) {
	t.Parallel()

	files := []gitlab.File{
		{Name: "file1.txt", Type: "blob", Path: "path/file1.txt"},
		{Name: "file2.txt", Type: "blob", Path: "path/file2.txt"},
		{Name: "dir", Type: "tree", Path: "path/dir"},
	}

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repository/tree") {
			_ = json.NewEncoder(w).Encode(files)
			return
		}
		t.Fatalf("Unexpected request URL: %s", r.URL.Path)
	}))
	defer apiServer.Close()

	md := &mockDownloader{}
	fetcher := gitlab.NewFetcher(md, slog.New(slog.NewTextHandler(os.Stdout, nil)), gitlab.WithHTTPClient(apiServer.Client()))

	src := types.Source{
		Gitlab: &types.GitlabOptions{
			Host:    apiServer.URL,
			Project: "project",
			Ref:     "main",
			Paths:   []string{"path"},
		},
	}

	destDir := t.TempDir()
	obj, err := fetcher.Fetch(context.Background(), destDir, src)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(obj.Objects) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(obj.Objects))
	}

	for _, o := range obj.Objects {
		if err = obj.Processor(context.Background(), o); err != nil {
			t.Fatalf("Processor failed: %v", err)
		}
	}

	if md.calls != 2 {
		t.Errorf("expected 2 downloads, got %d", md.calls)
	}

	if !strings.Contains(md.lastURL, "/repository/files/") {
		t.Errorf("unexpected last URL: %s", md.lastURL)
	}
}

func TestFetcher_Fetch_DirectFile(t *testing.T) {
	t.Parallel()

	md := &mockDownloader{}
	fetcher := gitlab.NewFetcher(md, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	src := types.Source{
		Gitlab: &types.GitlabOptions{
			Host:    "https://gitlab.com",
			Project: "proj",
			Ref:     "main",
			Paths:   []string{"myfile.txt"},
		},
	}

	destDir := t.TempDir()
	obj, err := fetcher.Fetch(context.Background(), destDir, src)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(obj.Objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(obj.Objects))
	}

	expectedDest := filepath.Join(destDir, "myfile.txt")
	if err = obj.Processor(context.Background(), obj.Objects[0]); err != nil {
		t.Fatalf("Processor failed: %v", err)
	}
	if md.lastDest != expectedDest {
		t.Errorf("expected dest %s, got %s", expectedDest, md.lastDest)
	}
}

func TestFetcher_Fetch_ListFilesError(t *testing.T) {
	t.Parallel()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer apiServer.Close()

	md := &mockDownloader{}
	fetcher := gitlab.NewFetcher(md, slog.New(slog.NewTextHandler(os.Stdout, nil)), gitlab.WithHTTPClient(apiServer.Client()))

	src := types.Source{
		Gitlab: &types.GitlabOptions{
			Host:    apiServer.URL,
			Project: "project",
			Ref:     "main",
			Paths:   []string{"path"},
		},
	}

	_, err := fetcher.Fetch(context.Background(), t.TempDir(), src)
	if err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("expected status error, got %v", err)
	}
}

func TestFetcher_Fetch_InvalidJSON(t *testing.T) {
	t.Parallel()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repository/tree") {
			_, _ = w.Write([]byte("{invalid json}"))
			return
		}
	}))
	defer apiServer.Close()

	md := &mockDownloader{}
	fetcher := gitlab.NewFetcher(md, slog.New(slog.NewTextHandler(os.Stdout, nil)), gitlab.WithHTTPClient(apiServer.Client()))

	src := types.Source{
		Gitlab: &types.GitlabOptions{
			Host:    apiServer.URL,
			Project: "project",
			Ref:     "main",
			Paths:   []string{"path"},
		},
	}

	_, err := fetcher.Fetch(context.Background(), t.TempDir(), src)
	if err == nil || !strings.Contains(err.Error(), "failed to decode tree") {
		t.Fatalf("expected decode error, got %v", err)
	}
}

func TestFetcher_Processor_DownloadError(t *testing.T) {
	t.Parallel()

	files := []gitlab.File{{Name: "file1.txt", Type: "blob", Path: "file1.txt"}}

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repository/tree") {
			_ = json.NewEncoder(w).Encode(files)
			return
		}
	}))
	defer apiServer.Close()

	md := &mockDownloader{err: errors.New("mock download error")}
	fetcher := gitlab.NewFetcher(md, slog.New(slog.NewTextHandler(os.Stdout, nil)), gitlab.WithHTTPClient(apiServer.Client()))

	src := types.Source{
		Gitlab: &types.GitlabOptions{
			Host:    apiServer.URL,
			Project: "project",
			Ref:     "main",
			Paths:   []string{""},
		},
	}

	obj, err := fetcher.Fetch(context.Background(), t.TempDir(), src)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if err = obj.Processor(context.Background(), obj.Objects[0]); err == nil || !strings.Contains(err.Error(), "mock download error") {
		t.Fatalf("expected mock download error, got %v", err)
	}
}
