package gcs

import (
	"cloud.google.com/go/storage"
	"context"
	"errors"
	"fmt"
	"github.com/AdamShannag/volare/pkg/types"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"io"
	"path/filepath"
)

type ObjectInfo struct {
	Key  string
	Size int64
}

type Options struct {
	Bucket          string
	CredentialsFile string
}

type Client interface {
	ListObjects(ctx context.Context, bucket, prefix string) ([]ObjectInfo, error)
	GetObject(ctx context.Context, bucket, object string) (io.ReadCloser, error)
}

type ClientFactory func(ctx context.Context, opts types.GCSOptions) (Client, error)

type gcsClient struct {
	client *storage.Client
}

func NewClient(ctx context.Context, credentialsFile string) (Client, error) {
	var opt option.ClientOption
	if credentialsFile == "" {
		opt = option.WithoutAuthentication()
	} else {
		opt = option.WithCredentialsFile(credentialsFile)
	}

	client, err := storage.NewClient(ctx, opt)
	if err != nil {
		return nil, fmt.Errorf("gcs: failed to create client: %w", err)
	}

	return &gcsClient{client: client}, nil
}

func (g *gcsClient) ListObjects(ctx context.Context, bucket, prefix string) ([]ObjectInfo, error) {
	var objects []ObjectInfo
	it := g.client.Bucket(bucket).Objects(ctx, &storage.Query{Prefix: prefix})

	for {
		attr, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("list error: %w", err)
		}

		if attr.Size == 0 && attr.Name[len(attr.Name)-1] == '/' {
			continue
		}
		objects = append(objects, ObjectInfo{
			Key:  attr.Name,
			Size: attr.Size,
		})
	}
	return objects, nil
}

func (g *gcsClient) GetObject(ctx context.Context, bucket, object string) (io.ReadCloser, error) {
	return g.client.Bucket(bucket).Object(object).NewReader(ctx)
}

func GCSClientFactory(ctx context.Context, opts types.GCSOptions) (Client, error) {
	return NewClient(ctx, filepath.Join(types.ResourcesDir, opts.CredentialsFile))
}
