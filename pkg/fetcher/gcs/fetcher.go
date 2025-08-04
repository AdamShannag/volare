package gcs

import (
	"context"
	"errors"
	"fmt"
	"github.com/AdamShannag/volare/pkg/fetcher"
	"github.com/AdamShannag/volare/pkg/types"
	"github.com/AdamShannag/volare/pkg/utils"
	"github.com/AdamShannag/volare/pkg/workerpool"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type Fetcher struct {
	clientFactory ClientFactory
}

func NewFetcher(clientFactory ClientFactory) fetcher.Fetcher {
	return &Fetcher{
		clientFactory: clientFactory,
	}
}

func (f *Fetcher) Fetch(ctx context.Context, mountPath string, src types.Source) error {
	if src.GCS == nil {
		return errors.New("invalid source configuration: 'gcs' options must be provided for source type 'gcs'")
	}
	opts := *src.GCS

	client, err := f.clientFactory(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to create s3 client: %w", err)
	}

	var allObjects []types.ObjectToDownload
	for _, p := range opts.Paths {
		objects, listErr := client.ListObjects(ctx, opts.Bucket, p)
		if listErr != nil {
			return fmt.Errorf("failed to list objects: %w", listErr)
		}
		for _, object := range objects {
			if strings.HasSuffix(object.Key, "/") {
				continue
			}
			allObjects = append(allObjects, types.ObjectToDownload{ActualPath: object.Key, Path: p})
		}
	}

	if len(allObjects) == 0 {
		slog.Info("no objects found for download", "bucket", opts.Bucket, "paths", opts.Paths)
		return nil
	}

	type job struct {
		file types.ObjectToDownload
	}

	processor := func(ctx context.Context, job job) error {
		return downloadObject(ctx, client, mountPath, opts.Bucket, job.file)
	}

	numOfWorkers := types.DefaultNumberOfWorkers
	if opts.Workers != nil {
		numOfWorkers = *opts.Workers
	}

	pool := workerpool.New(ctx, numOfWorkers, len(allObjects), processor)
	pool.Start()

	for _, object := range allObjects {
		if err = pool.Submit(job{object}); err != nil {
			pool.Cancel()
			pool.Stop()
			return err
		}
	}

	pool.Stop()

	for err = range pool.Errors() {
		if err != nil {
			return err
		}
	}

	return nil
}

func downloadObject(ctx context.Context, client Client, mountPath, bucket string, file types.ObjectToDownload) error {
	slog.Info("downloading gcs object", "bucket", bucket, "key", file.ActualPath)

	reader, err := client.GetObject(ctx, bucket, file.ActualPath)
	if err != nil {
		return fmt.Errorf("failed to get object %q: %w", file.ActualPath, err)
	}
	defer func() {
		if err = reader.Close(); err != nil {
			slog.Warn("error closing object reader", "key", file.ActualPath, "error", err)
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
			slog.Warn("error closing file", "file", targetPath, "error", err)
		}
	}()

	if _, err = io.Copy(fh, reader); err != nil {
		return fmt.Errorf("failed to copy content to %q: %w", targetPath, err)
	}

	return nil
}
