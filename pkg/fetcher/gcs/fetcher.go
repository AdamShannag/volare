package gcs

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/AdamShannag/volare/pkg/fetcher"
	"github.com/AdamShannag/volare/pkg/types"
	"github.com/AdamShannag/volare/pkg/utils"
)

type Fetcher struct {
	clientFactory ClientFactory
	logger        *slog.Logger
}

func NewFetcher(clientFactory ClientFactory, logger *slog.Logger) fetcher.Fetcher {
	return &Fetcher{
		clientFactory: clientFactory,
		logger:        logger,
	}
}

func (f *Fetcher) Fetch(ctx context.Context, mountPath string, src types.Source) (*fetcher.Object, error) {
	client, err := f.clientFactory(ctx, *src.GCS)
	if err != nil {
		return nil, fmt.Errorf("failed to create s3 client: %w", err)
	}

	var allObjects []types.ObjectToDownload
	for _, p := range src.GCS.Paths {
		objects, listErr := client.ListObjects(ctx, src.GCS.Bucket, p)
		if listErr != nil {
			return nil, fmt.Errorf("failed to list objects: %w", listErr)
		}
		for _, object := range objects {
			if strings.HasSuffix(object.Key, "/") {
				continue
			}
			allObjects = append(allObjects, types.ObjectToDownload{ActualPath: object.Key, Path: p})
		}
	}

	if len(allObjects) == 0 {
		f.logger.Info("no files found", "bucket", src.GCS.Bucket, "paths", src.GCS.Paths)
	}

	return &fetcher.Object{
		Processor: func(ctx context.Context, job types.ObjectToDownload) error {
			return f.download(ctx, client, mountPath, src.GCS.Bucket, job)
		},
		Objects: allObjects,
		Workers: src.GCS.Workers,
	}, nil
}

func (f *Fetcher) download(ctx context.Context, client Client, mountPath, bucket string, file types.ObjectToDownload) error {
	f.logger.Info("downloading file", "bucket", bucket, "key", file.ActualPath)

	reader, err := client.GetObject(ctx, bucket, file.ActualPath)
	if err != nil {
		return fmt.Errorf("failed to get object %q: %w", file.ActualPath, err)
	}
	defer func() {
		if err = reader.Close(); err != nil {
			f.logger.Warn("error closing object reader", "key", file.ActualPath, "error", err)
		}
	}()

	targetPath := utils.ResolveTargetPath(mountPath, file)
	if err = os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory for %q: %w", targetPath, err)
	}

	fh, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create file %q: %w", targetPath, err)
	}
	defer func() {
		if err = fh.Close(); err != nil {
			f.logger.Warn("error closing file", "file", targetPath, "error", err)
		}
	}()

	if _, err = io.Copy(fh, reader); err != nil {
		return fmt.Errorf("failed to copy content to %q: %w", targetPath, err)
	}

	return nil
}
