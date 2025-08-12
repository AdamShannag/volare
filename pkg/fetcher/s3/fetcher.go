package s3

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
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client interface {
	ListObjects(ctx context.Context, bucket string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	GetObject(ctx context.Context, bucket, object string, opts minio.GetObjectOptions) (io.ReadCloser, error)
}

type ClientFactory func(opts types.S3Options) (Client, error)

type Fetcher struct {
	clientFactory ClientFactory
	logger        *slog.Logger
}

func NewFetcher(factory ClientFactory, logger *slog.Logger) fetcher.Fetcher {
	return &Fetcher{clientFactory: factory, logger: logger}
}

func (f *Fetcher) Fetch(ctx context.Context, mountPath string, src types.Source) (*fetcher.Object, error) {
	client, err := f.clientFactory(*src.S3)
	if err != nil {
		return nil, fmt.Errorf("failed to create s3 client: %w", err)
	}

	var allObjects []types.ObjectToDownload
	for _, p := range src.S3.Paths {
		objectCh := client.ListObjects(ctx, src.S3.Bucket, minio.ListObjectsOptions{
			Prefix:    strings.TrimLeft(p, "/"),
			Recursive: true,
		})

		for object := range objectCh {
			if object.Err != nil {
				return nil, fmt.Errorf("failed to list objects: %w", object.Err)
			}
			if strings.HasSuffix(object.Key, "/") {
				continue
			}
			allObjects = append(allObjects, types.ObjectToDownload{ActualPath: object.Key, Path: p})
		}
	}

	if len(allObjects) == 0 {
		f.logger.Info("no files found", "bucket", src.S3.Bucket, "paths", src.S3.Paths)
	}

	return &fetcher.Object{
		Processor: func(ctx context.Context, job types.ObjectToDownload) error {
			return f.download(ctx, client, mountPath, src.S3.Bucket, job)
		},
		Objects: allObjects,
		Workers: src.S3.Workers,
	}, nil
}

func (f *Fetcher) download(ctx context.Context, client Client, mountPath, bucket string, file types.ObjectToDownload) error {
	f.logger.Info("downloading file", "bucket", bucket, "key", file.ActualPath)

	reader, err := client.GetObject(ctx, bucket, file.ActualPath, minio.GetObjectOptions{})
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
