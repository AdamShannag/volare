package downloader_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AdamShannag/volare/pkg/downloader"
)

func TestHTTPDownloader_Download(t *testing.T) {
	t.Parallel()

	const fileContent = "hello world"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, fileContent)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destFile := filepath.Join(tmpDir, "test.txt")

	d := downloader.NewHTTPDownloader()

	err := d.Download(context.Background(), server.URL, map[string]string{
		"X-Test": "value",
	}, destFile)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	data, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("Reading downloaded file failed: %v", err)
	}

	if got := string(data); got != fileContent {
		t.Errorf("Expected file content %q, got %q", fileContent, got)
	}
}

func TestHTTPDownloader_Download_404(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destFile := filepath.Join(tmpDir, "notfound.txt")

	d := downloader.NewHTTPDownloader()
	err := d.Download(context.Background(), server.URL, nil, destFile)
	if err == nil || !strings.Contains(err.Error(), "unexpected HTTP status") {
		t.Fatalf("Expected error for 404, got: %v", err)
	}
}

func TestHTTPDownloader_Download_InvalidURL(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	destFile := filepath.Join(tmpDir, "invalid.txt")

	d := downloader.NewHTTPDownloader()
	err := d.Download(context.Background(), "://invalid-url", nil, destFile)
	if err == nil || !strings.Contains(err.Error(), "failed to create HTTP request") {
		t.Fatalf("Expected error for invalid URL, got: %v", err)
	}
}
