package github_test

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

	"github.com/AdamShannag/volare/pkg/fetcher/github"
	"github.com/AdamShannag/volare/pkg/types"
)

type mockDownloader struct {
	calls    int32
	lastURL  string
	lastDest string
	err      error
	headers  map[string]string
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

	treeResp := map[string]interface{}{
		"tree": []map[string]string{
			{"path": "example/file1.txt", "type": "blob"},
			{"path": "example/file2.txt", "type": "blob"},
			{"path": "example/dir", "type": "tree"},
		},
	}

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/trees/") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(treeResp)
			return
		}
		t.Fatalf("unexpected request: %s", r.URL.Path)
	}))
	defer apiServer.Close()

	md := &mockDownloader{}
	fetcher := github.NewFetcher(md,
		slog.New(slog.NewTextHandler(os.Stdout, nil)),
		github.WithHTTPClient(apiServer.Client()),
		github.WithBaseURL(apiServer.URL),
	)

	src := types.Source{
		GitHub: &types.GitHubOptions{
			Owner: "owner",
			Repo:  "repo",
			Ref:   "main",
			Paths: []string{"example"},
		},
	}

	destDir := t.TempDir()
	obj, err := fetcher.Fetch(context.Background(), destDir, src)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(obj.Objects) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(obj.Objects))
	}

	for _, job := range obj.Objects {
		if err = obj.Processor(context.Background(), job); err != nil {
			t.Fatalf("Processor failed: %v", err)
		}
	}

	if md.calls != 2 {
		t.Errorf("expected 2 download calls, got %d", md.calls)
	}

	if !strings.Contains(md.lastURL, "raw.githubusercontent.com") {
		t.Errorf("expected raw.githubusercontent.com URL, got %s", md.lastURL)
	}

	if !strings.HasPrefix(md.lastDest, destDir) {
		t.Errorf("expected dest inside %s, got %s", destDir, md.lastDest)
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
			_ = json.NewEncoder(w).Encode(treeResp)
			return
		}
	}))
	defer apiServer.Close()

	md := &mockDownloader{err: errors.New("mock download error")}
	fetcher := github.NewFetcher(md,
		slog.New(slog.NewTextHandler(os.Stdout, nil)),
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

	obj, err := fetcher.Fetch(context.Background(), t.TempDir(), src)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if err = obj.Processor(context.Background(), obj.Objects[0]); err == nil || !strings.Contains(err.Error(), "mock download error") {
		t.Errorf("expected download error, got %v", err)
	}
}

func TestFetcher_Fetch_ListFilesError(t *testing.T) {
	t.Parallel()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer apiServer.Close()

	md := &mockDownloader{}
	fetcher := github.NewFetcher(md,
		slog.New(slog.NewTextHandler(os.Stdout, nil)),
		github.WithHTTPClient(apiServer.Client()),
		github.WithBaseURL(apiServer.URL),
	)

	src := types.Source{
		GitHub: &types.GitHubOptions{
			Owner: "owner",
			Repo:  "repo",
			Ref:   "main",
			Paths: []string{"path"},
		},
	}

	_, err := fetcher.Fetch(context.Background(), t.TempDir(), src)
	if err == nil || !strings.Contains(err.Error(), "GitHub API returned status") {
		t.Errorf("expected GitHub API returned status error, got %v", err)
	}
}

func TestFetcher_Fetch_FilePathDirectly(t *testing.T) {
	t.Parallel()

	md := &mockDownloader{}
	fetcher := github.NewFetcher(md, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	src := types.Source{
		GitHub: &types.GitHubOptions{
			Owner: "o",
			Repo:  "r",
			Ref:   "main",
			Paths: []string{"file.txt"},
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

	if obj.Objects[0].Path != "file.txt" {
		t.Errorf("unexpected path: %s", obj.Objects[0].Path)
	}

	expectedDest := filepath.Join(destDir, "file.txt")
	if err = obj.Processor(context.Background(), obj.Objects[0]); err != nil {
		t.Fatalf("Processor failed: %v", err)
	}
	if md.lastDest != expectedDest {
		t.Errorf("expected dest %s, got %s", expectedDest, md.lastDest)
	}
}
