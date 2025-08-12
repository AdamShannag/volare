package http_test

import (
	"context"
	"errors"
	"log/slog"
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

	fetcher := httpfetcher.NewFetcher(mock, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	obj, err := fetcher.Fetch(context.Background(), tmpDir, src)
	if err != nil {
		t.Fatalf("expected no error from Fetch, got %v", err)
	}

	if len(obj.Objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(obj.Objects))
	}

	job := obj.Objects[0]
	if job.ActualPath != src.Http.URI {
		t.Errorf("unexpected ActualPath: got %s", job.ActualPath)
	}
	expectedFile := filepath.Join(tmpDir, "file.txt")
	if job.Path != expectedFile {
		t.Errorf("unexpected Path: got %s, want %s", job.Path, expectedFile)
	}

	if err = obj.Processor(context.Background(), job); err != nil {
		t.Fatalf("processor returned error: %v", err)
	}

	if _, err = os.Stat(expectedFile); err != nil {
		t.Fatalf("expected file at %s, got error: %v", expectedFile, err)
	}
}

func TestFetcher_Fetch_CustomFilePath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	customFile := filepath.Join(tmpDir, "custom.txt")
	src := types.Source{
		Http: &types.HttpOptions{
			URI: "https://example.com/file.txt",
		},
	}

	mock := &MockDownloader{
		DownloadFunc: func(ctx context.Context, url string, headers map[string]string, dest string) error {
			if dest != customFile {
				t.Fatalf("expected dest %s, got %s", customFile, dest)
			}
			return nil
		},
	}

	fetcher := httpfetcher.NewFetcher(mock, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	obj, err := fetcher.Fetch(context.Background(), customFile, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(obj.Objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(obj.Objects))
	}

	if obj.Objects[0].Path != customFile {
		t.Errorf("expected Path %s, got %s", customFile, obj.Objects[0].Path)
	}
}

func TestFetcher_Fetch_ProcessorError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
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

	fetcher := httpfetcher.NewFetcher(mock, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	obj, err := fetcher.Fetch(context.Background(), tmpDir, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	job := obj.Objects[0]
	err = obj.Processor(context.Background(), job)
	if err == nil || !strings.Contains(err.Error(), "download error") {
		t.Fatalf("expected download error, got: %v", err)
	}
}
