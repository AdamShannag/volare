package http_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	httpfetcher "github.com/AdamShannag/volare/pkg/fetcher/http"
	"github.com/AdamShannag/volare/pkg/types"
)

type MockDownloader struct {
	DownloadFunc func(ctx context.Context, url string, headers map[string]string, dest string) error
}

func (m *MockDownloader) Download(ctx context.Context, url string, headers map[string]string, dest string) error {
	if m.DownloadFunc != nil {
		return m.DownloadFunc(ctx, url, headers, dest)
	}
	return nil
}

func TestFetcher_Fetch_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	src := types.Source{
		Http: &types.HttpOptions{
			URI:     "https://example.com/file.txt",
			Headers: map[string]string{"Authorization": "Bearer TOKEN"},
		},
	}

	mock := &MockDownloader{
		DownloadFunc: func(ctx context.Context, url string, headers map[string]string, dest string) error {
			if url != src.Http.URI {
				t.Fatalf("unexpected URL: got %s", url)
			}
			if headers["Authorization"] != "Bearer TOKEN" {
				t.Fatalf("unexpected header: %v", headers)
			}
			return os.WriteFile(dest, []byte("test content"), 0644)
		},
	}

	fetcher := httpfetcher.NewFetcher(mock)

	err := fetcher.Fetch(context.Background(), tmpDir, src)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "file.txt")
	_, err = os.Stat(expectedPath)
	if err != nil {
		t.Fatalf("expected file to exist at %s, got error: %v", expectedPath, err)
	}
}

func TestFetcher_Fetch_InvalidConfig(t *testing.T) {
	t.Parallel()

	fetcher := httpfetcher.NewFetcher(&MockDownloader{})
	src := types.Source{Http: nil}

	err := fetcher.Fetch(context.Background(), "/tmp", src)
	if err == nil || err.Error() != "invalid source configuration: 'http' options must be provided for source type 'http'" {
		t.Fatalf("expected configuration error, got: %v", err)
	}
}

func TestFetcher_Fetch_MkdirFails(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "fakefile")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func(name string) {
		_ = os.Remove(name)
	}(tmpFile.Name())

	src := types.Source{
		Http: &types.HttpOptions{
			URI: "https://example.com/file.txt",
		},
	}

	mountPath := filepath.Join(tmpFile.Name(), "nested")

	fetcher := httpfetcher.NewFetcher(&MockDownloader{})
	err = fetcher.Fetch(context.Background(), mountPath, src)
	if err == nil || !strings.Contains(err.Error(), "failed to create target directory") {
		t.Fatalf("expected directory creation error, got: %v", err)
	}
}

func TestFetcher_Fetch_DownloadFails(t *testing.T) {
	t.Parallel()

	src := types.Source{
		Http: &types.HttpOptions{
			URI: "https://example.com/file.txt",
		},
	}

	mock := &MockDownloader{
		DownloadFunc: func(ctx context.Context, url string, headers map[string]string, dest string) error {
			return errors.New("download error")
		},
	}

	tmpDir := t.TempDir()
	fetcher := httpfetcher.NewFetcher(mock)

	err := fetcher.Fetch(context.Background(), tmpDir, src)
	if err == nil || err.Error() != `failed to download "https://example.com/file.txt": download error` {
		t.Fatalf("expected wrapped download error, got: %v", err)
	}
}
