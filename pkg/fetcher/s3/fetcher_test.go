package s3_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/AdamShannag/volare/pkg/fetcher/s3"
	"github.com/AdamShannag/volare/pkg/types"
	"github.com/minio/minio-go/v7"
)

type mockClient struct {
	listObjectsFunc func(ctx context.Context, bucket string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	getObjectFunc   func(ctx context.Context, bucket, object string, opts minio.GetObjectOptions) (io.ReadCloser, error)
}

func (m *mockClient) ListObjects(ctx context.Context, bucket string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	return m.listObjectsFunc(ctx, bucket, opts)
}

func (m *mockClient) GetObject(ctx context.Context, bucket, object string, opts minio.GetObjectOptions) (io.ReadCloser, error) {
	return m.getObjectFunc(ctx, bucket, object, opts)
}

func TestFetcher_Fetch_Success(t *testing.T) {
	t.Parallel()

	objects := []minio.ObjectInfo{
		{Key: "file1.txt"},
		{Key: "file2.txt"},
		{Key: "dir/"},
	}

	var calls int32
	mock := &mockClient{
		listObjectsFunc: func(_ context.Context, _ string, _ minio.ListObjectsOptions) <-chan minio.ObjectInfo {
			ch := make(chan minio.ObjectInfo, len(objects))
			for _, o := range objects {
				ch <- o
			}
			close(ch)
			return ch
		},
		getObjectFunc: func(_ context.Context, _, _ string, _ minio.GetObjectOptions) (io.ReadCloser, error) {
			atomic.AddInt32(&calls, 1)
			return io.NopCloser(strings.NewReader("data")), nil
		},
	}

	fetcher := s3.NewFetcher(func(opts types.S3Options) (s3.Client, error) {
		return mock, nil
	})

	src := types.Source{
		S3: &types.S3Options{
			Bucket: "bucket",
			Paths:  []string{""},
		},
	}

	tmpDir := t.TempDir()
	err := fetcher.Fetch(context.Background(), tmpDir, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("expected 2 downloads, got %d", got)
	}
}

func TestFetcher_Fetch_ListObjectsError(t *testing.T) {
	t.Parallel()

	mock := &mockClient{
		listObjectsFunc: func(_ context.Context, _ string, _ minio.ListObjectsOptions) <-chan minio.ObjectInfo {
			ch := make(chan minio.ObjectInfo, 1)
			ch <- minio.ObjectInfo{Err: errors.New("list error")}
			close(ch)
			return ch
		},
		getObjectFunc: func(_ context.Context, _, _ string, _ minio.GetObjectOptions) (io.ReadCloser, error) {
			t.Fatal("GetObject should not be called")
			return nil, nil
		},
	}

	fetcher := s3.NewFetcher(func(opts types.S3Options) (s3.Client, error) {
		return mock, nil
	})

	err := fetcher.Fetch(context.Background(), t.TempDir(), types.Source{
		S3: &types.S3Options{
			Bucket: "bucket",
			Paths:  []string{"/bad"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "failed to list objects") {
		t.Fatalf("expected list error, got %v", err)
	}
}

func TestFetcher_Fetch_DownloadError(t *testing.T) {
	t.Parallel()

	mock := &mockClient{
		listObjectsFunc: func(_ context.Context, _ string, _ minio.ListObjectsOptions) <-chan minio.ObjectInfo {
			ch := make(chan minio.ObjectInfo, 1)
			ch <- minio.ObjectInfo{Key: "file.txt"}
			close(ch)
			return ch
		},
		getObjectFunc: func(_ context.Context, _, _ string, _ minio.GetObjectOptions) (io.ReadCloser, error) {
			return nil, errors.New("download error")
		},
	}

	fetcher := s3.NewFetcher(func(opts types.S3Options) (s3.Client, error) {
		return mock, nil
	})

	err := fetcher.Fetch(context.Background(), t.TempDir(), types.Source{
		S3: &types.S3Options{
			Bucket: "bucket",
			Paths:  []string{"a"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "download error") {
		t.Fatalf("expected download error, got %v", err)
	}
}

func TestFetcher_Fetch_InvalidConfig(t *testing.T) {
	t.Parallel()

	fetcher := s3.NewFetcher(nil)

	err := fetcher.Fetch(context.Background(), t.TempDir(), types.Source{})
	if err == nil || !strings.Contains(err.Error(), "invalid source configuration") {
		t.Fatalf("expected invalid source config error, got %v", err)
	}
}
