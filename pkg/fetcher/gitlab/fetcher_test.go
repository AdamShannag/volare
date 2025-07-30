package gitlab_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/AdamShannag/volare/pkg/fetcher/gitlab"
	"github.com/AdamShannag/volare/pkg/types"
)

type MockDownloader struct {
	Calls    int32
	ErrAfter int32
}

func (m *MockDownloader) Download(_ context.Context, _ string, _ map[string]string, _ string) error {
	c := atomic.AddInt32(&m.Calls, 1)
	if m.ErrAfter == 0 || c > m.ErrAfter {
		return errors.New("mock download error")
	}
	return nil
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
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(files)
			return
		}
		t.Fatalf("Unexpected request URL: %s", r.URL.Path)
	}))
	defer apiServer.Close()

	mockDownloader := &MockDownloader{ErrAfter: 999}
	fetcher := gitlab.NewFetcher(mockDownloader,
		gitlab.WithHTTPClient(apiServer.Client()),
	)

	src := types.Source{
		Gitlab: &types.GitlabOptions{
			Host:    apiServer.URL,
			Project: "project",
			Ref:     "main",
			Paths:   []string{"path"},
		},
	}

	tmpDir := t.TempDir()
	err := fetcher.Fetch(context.Background(), tmpDir, src)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if calls := atomic.LoadInt32(&mockDownloader.Calls); calls != 2 {
		t.Errorf("Expected 2 download calls, got %d", calls)
	}
}

func TestFetcher_Fetch_ListFilesError(t *testing.T) {
	t.Parallel()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer apiServer.Close()

	mockDownloader := &MockDownloader{}
	fetcher := gitlab.NewFetcher(mockDownloader,
		gitlab.WithHTTPClient(apiServer.Client()),
	)

	src := types.Source{
		Gitlab: &types.GitlabOptions{
			Host:    apiServer.URL,
			Project: "project",
			Ref:     "main",
			Paths:   []string{""},
		},
	}

	err := fetcher.Fetch(context.Background(), t.TempDir(), src)
	if err == nil || !strings.Contains(err.Error(), "failed to list tree") {
		t.Fatalf("Expected list tree error, got: %v", err)
	}
}

func TestFetcher_Fetch_DownloadError(t *testing.T) {
	t.Parallel()

	files := []gitlab.File{
		{Name: "file1.txt", Type: "blob", Path: "file1.txt"},
	}

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repository/tree") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(files)
			return
		}
	}))
	defer apiServer.Close()

	mockDownloader := &MockDownloader{ErrAfter: 0}
	fetcher := gitlab.NewFetcher(mockDownloader,
		gitlab.WithHTTPClient(apiServer.Client()),
	)

	src := types.Source{
		Gitlab: &types.GitlabOptions{
			Host:    apiServer.URL,
			Project: "project",
			Ref:     "main",
			Paths:   []string{""},
		},
	}

	err := fetcher.Fetch(context.Background(), t.TempDir(), src)
	if err == nil || !strings.Contains(err.Error(), "mock download error") {
		t.Fatalf("Expected download error, got: %v", err)
	}
}

func TestFetcher_Fetch_InvalidConfig(t *testing.T) {
	t.Parallel()

	mockDownloader := &MockDownloader{}
	fetcher := gitlab.NewFetcher(mockDownloader)

	src := types.Source{}

	err := fetcher.Fetch(context.Background(), t.TempDir(), src)
	if err == nil || !strings.Contains(err.Error(), "invalid source configuration") {
		t.Fatalf("Expected invalid source configuration error, got: %v", err)
	}
}
