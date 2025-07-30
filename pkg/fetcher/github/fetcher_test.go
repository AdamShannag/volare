package github_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/AdamShannag/volare/pkg/fetcher/github"
	"github.com/AdamShannag/volare/pkg/types"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
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

	treeResp := map[string]interface{}{
		"tree": []map[string]string{
			{"path": "example/file1.txt", "type": "blob"},
			{"path": "example/file2.txt", "type": "blob"},
			{"path": "example/dir", "type": "tree"},
		},
	}

	rawServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("file content"))
	}))
	defer rawServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/trees/") {
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(treeResp)
			if err != nil {
				return
			}
			return
		}
		t.Fatalf("Unexpected request URL: %s", r.URL.Path)
	}))
	defer apiServer.Close()

	downloader := &MockDownloader{ErrAfter: 999}
	fetcher := github.NewFetcher(downloader,
		github.WithHTTPClient(apiServer.Client()),
		github.WithBaseURL(apiServer.URL),
	)

	src := types.Source{
		GitHub: &types.GitHubOptions{
			Owner: "owner",
			Repo:  "repo",
			Ref:   "main",
			Paths: []string{"example"},
			Token: "",
		},
	}

	tmpDir := t.TempDir()
	err := fetcher.Fetch(context.Background(), tmpDir, src)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if downloader.Calls != 2 {
		t.Errorf("Expected 2 download calls, got %d", downloader.Calls)
	}
}

func TestFetcher_Fetch_MissingGitHubOptions(t *testing.T) {
	t.Parallel()

	downloader := &MockDownloader{}
	fetcher := github.NewFetcher(downloader)

	src := types.Source{
		GitHub: nil,
	}

	err := fetcher.Fetch(context.Background(), ".", src)
	if err == nil || !strings.Contains(err.Error(), "invalid source configuration") {
		t.Errorf("Expected invalid source configuration error, got %v", err)
	}
}

func TestFetcher_Fetch_DownloadError(t *testing.T) {
	t.Parallel()

	treeResp := map[string]interface{}{
		"tree": []map[string]string{
			{"path": "file1.txt", "type": "blob"},
		},
	}

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/trees/") {
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(treeResp)
			if err != nil {
				return
			}
			return
		}
	}))
	defer apiServer.Close()

	downloader := &MockDownloader{ErrAfter: 0}
	fetcher := github.NewFetcher(downloader,
		github.WithHTTPClient(apiServer.Client()),
		github.WithBaseURL(apiServer.URL),
	)

	src := types.Source{
		GitHub: &types.GitHubOptions{
			Owner: "owner",
			Repo:  "repo",
			Ref:   "main",
			Paths: []string{""},
		},
	}

	err := fetcher.Fetch(context.Background(), t.TempDir(), src)
	if err == nil || !strings.Contains(err.Error(), "mock download error") {
		t.Errorf("Expected download error, got %v", err)
	}
}
