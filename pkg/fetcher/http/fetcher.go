package http

import (
	"context"
	"log/slog"
	"path/filepath"

	"github.com/AdamShannag/volare/pkg/downloader"
	"github.com/AdamShannag/volare/pkg/fetcher"
	"github.com/AdamShannag/volare/pkg/types"
	"github.com/AdamShannag/volare/pkg/utils"
)

type Fetcher struct {
	downloader downloader.Downloader
	logger     *slog.Logger
}

func NewFetcher(downloader downloader.Downloader, logger *slog.Logger) *Fetcher {
	return &Fetcher{
		downloader: downloader,
		logger:     logger,
	}
}

func (f *Fetcher) Fetch(_ context.Context, mountPath string, src types.Source) (*fetcher.Object, error) {
	f.logger.Info("downloading file", slog.String("url", src.Http.URI))

	resolvedHeaders := make(map[string]string, len(src.Http.Headers))
	for k, v := range src.Http.Headers {
		resolvedHeaders[k] = utils.FromEnv(v)
	}

	workers := 1
	return &fetcher.Object{
		Processor: func(ctx context.Context, j types.ObjectToDownload) error {
			return f.downloader.Download(ctx, j.ActualPath, resolvedHeaders, j.Path)
		},
		Objects: []types.ObjectToDownload{
			{
				ActualPath: src.Http.URI,
				Path:       resolveFilePaths(mountPath, src.Http.URI),
			},
		},
		Workers: &workers,
	}, nil
}

func resolveFilePaths(targetPath, uri string) (fullFilePath string) {
	if filepath.Ext(targetPath) != "" {
		fullFilePath = targetPath
	} else {
		filename := filepath.Base(uri)
		fullFilePath = filepath.Join(targetPath, filename)
	}

	return fullFilePath
}
