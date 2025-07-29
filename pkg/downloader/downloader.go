package downloader

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
)

type Downloader interface {
	Download(ctx context.Context, url string, headers map[string]string, destPath string) error
}

type HTTPDownloader struct {
	client *http.Client
}

type Option func(*HTTPDownloader)

func WithHTTPClient(client *http.Client) Option {
	return func(d *HTTPDownloader) {
		d.client = client
	}
}

func NewHTTPDownloader(opts ...Option) *HTTPDownloader {
	d := &HTTPDownloader{
		client: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

func (d *HTTPDownloader) Download(ctx context.Context, url string, headers map[string]string, destPath string) (err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch %q: %w", url, err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			slog.Warn("error closing response body", "error", cerr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status %d fetching %q", resp.StatusCode, url)
	}

	if err = os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory for %q: %w", destPath, err)
	}

	outFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file %q: %w", destPath, err)
	}

	defer func() {
		if cerr := outFile.Close(); cerr != nil {
			slog.Warn("error closing file", "error", cerr)
		}
	}()

	if _, err = io.Copy(outFile, resp.Body); err != nil {
		return fmt.Errorf("failed to write file %q: %w", destPath, err)
	}

	return nil
}
