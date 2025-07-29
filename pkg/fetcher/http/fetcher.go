package http

import (
	"context"
	"fmt"
	"github.com/AdamShannag/volare/pkg/downloader"
	"github.com/AdamShannag/volare/pkg/types"
	"github.com/AdamShannag/volare/pkg/utils"
	"log/slog"
	"os"
	"path/filepath"
)

type Fetcher struct {
	downloader downloader.Downloader
}

func NewFetcher(downloader downloader.Downloader) *Fetcher {
	return &Fetcher{
		downloader: downloader,
	}
}

func (h *Fetcher) Fetch(ctx context.Context, mountPath string, src types.Source) error {
	if src.Http == nil {
		return fmt.Errorf("invalid source configuration: 'http' options must be provided for source type 'http'")
	}
	slog.Info("downloading file from url", slog.String("url", src.Http.URI))

	dir, path := resolveFilePaths(mountPath, src.Http.URI)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create target directory %q: %w", dir, err)
	}

	resolvedHeaders := make(map[string]string, len(src.Http.Headers))
	for k, v := range src.Http.Headers {
		resolvedHeaders[k] = utils.FromEnv(v)
	}

	if err := h.downloader.Download(ctx, src.Http.URI, resolvedHeaders, path); err != nil {
		return fmt.Errorf("failed to download %q: %w", src.Http.URI, err)
	}

	return nil
}

func resolveFilePaths(targetPath, uri string) (dirPath string, fullFilePath string) {
	if filepath.Ext(targetPath) != "" {
		fullFilePath = targetPath
	} else {
		filename := filepath.Base(uri)
		fullFilePath = filepath.Join(targetPath, filename)
	}

	dirPath = filepath.Dir(fullFilePath)
	return dirPath, fullFilePath
}
