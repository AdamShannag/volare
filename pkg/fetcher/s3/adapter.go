package s3

import (
	"context"
	"io"

	"github.com/minio/minio-go/v7"
)

type minioAdapter struct {
	client *minio.Client
}

func (m *minioAdapter) ListObjects(ctx context.Context, bucket string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	return m.client.ListObjects(ctx, bucket, opts)
}

func (m *minioAdapter) GetObject(ctx context.Context, bucket, object string, opts minio.GetObjectOptions) (io.ReadCloser, error) {
	obj, err := m.client.GetObject(ctx, bucket, object, opts)
	if err != nil {
		return nil, err
	}
	return obj, nil
}
