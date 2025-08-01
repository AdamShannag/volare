package s3

import (
	"context"
	"errors"
	"fmt"
	"github.com/AdamShannag/volare/pkg/fetcher"
	"github.com/AdamShannag/volare/pkg/types"
	"github.com/AdamShannag/volare/pkg/utils"
	"github.com/AdamShannag/volare/pkg/workerpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type Client interface {
	ListObjects(ctx context.Context, bucket string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	GetObject(ctx context.Context, bucket, object string, opts minio.GetObjectOptions) (io.ReadCloser, error)
}

type ClientFactory func(opts types.S3Options) (Client, error)

type Fetcher struct {
	clientFactory ClientFactory
}

func NewFetcher(factory ClientFactory) fetcher.Fetcher {
	return &Fetcher{clientFactory: factory}
}

func (f *Fetcher) Fetch(ctx context.Context, mountPath string, src types.Source) error {
	if src.S3 == nil {
		return errors.New("invalid source configuration: 's3' options must be provided for source type 's3'")
	}
	opts := *src.S3

	client, err := f.clientFactory(opts)
	if err != nil {
		return fmt.Errorf("failed to create s3 client: %w", err)
	}

	var allObjects []types.ObjectToDownload
	for _, p := range opts.Paths {
		objectCh := client.ListObjects(ctx, opts.Bucket, minio.ListObjectsOptions{
			Prefix:    strings.TrimLeft(p, "/"),
			Recursive: true,
		})

		for object := range objectCh {
			if object.Err != nil {
				return fmt.Errorf("failed to list objects: %w", object.Err)
			}
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

	for _, s3File := range allObjects {
		if err = pool.Submit(job{s3File}); err != nil {
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
	slog.Info("downloading s3 object", "bucket", bucket, "key", file.ActualPath)

	reader, err := client.GetObject(ctx, bucket, file.ActualPath, minio.GetObjectOptions{})
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

func MinioClientFactory(opts types.S3Options) (Client, error) {
	c, err := minio.New(opts.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(utils.FromEnv(opts.AccessKeyID), utils.FromEnv(opts.SecretAccessKey), utils.FromEnv(opts.SessionToken)),
		Secure: opts.Secure,
		Region: opts.Region,
	})
	if err != nil {
		return nil, err
	}
	return &minioAdapter{client: c}, nil
}
