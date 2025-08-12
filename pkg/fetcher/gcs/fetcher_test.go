package gcs_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/AdamShannag/volare/pkg/fetcher/gcs"
	"github.com/AdamShannag/volare/pkg/types"
	"github.com/AdamShannag/volare/pkg/utils"
)

type mockClient struct {
	mu         sync.Mutex
	objects    map[string][]byte
	listErr    error
	getErr     error
	listCalls  []string
	getCalls   []string
	failOnList bool
	failOnGet  bool
}

func (m *mockClient) ListObjects(_ context.Context, _, prefix string) ([]gcs.ObjectInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listCalls = append(m.listCalls, prefix)

	if m.failOnList {
		return nil, m.listErr
	}

	var res []gcs.ObjectInfo
	for k, v := range m.objects {
		if strings.HasPrefix(k, prefix) {
			res = append(res, gcs.ObjectInfo{Key: k, Size: int64(len(v))})
		}
	}
	return res, nil
}

func (m *mockClient) GetObject(_ context.Context, _, object string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getCalls = append(m.getCalls, object)

	if m.failOnGet {
		return nil, m.getErr
	}

	data, ok := m.objects[object]
	if !ok {
		return nil, errors.New("object not found")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func TestFetcher_Fetch_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mock := &mockClient{
		objects: map[string][]byte{
			"file1.txt":     []byte("hello"),
			"dir/file2.txt": []byte("world"),
		},
	}

	clientFactory := func(ctx context.Context, opts types.GCSOptions) (gcs.Client, error) {
		return mock, nil
	}

	fetcherInstance := gcs.NewFetcher(clientFactory, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	workers := 2
	src := types.Source{
		Type: "gcs",
		GCS: &types.GCSOptions{
			Bucket:  "bucket1",
			Paths:   []string{"file1.txt", "dir"},
			Workers: &workers,
		},
	}

	tmpDir := t.TempDir()
	obj, err := fetcherInstance.Fetch(ctx, tmpDir, src)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if obj == nil {
		t.Fatal("expected non-nil fetcher.Object")
	}
	if len(obj.Objects) != workers {
		t.Fatalf("expected 2 objects to download, got %d", len(obj.Objects))
	}

	for _, o := range obj.Objects {
		if pErr := obj.Processor(ctx, o); pErr != nil {
			t.Fatalf("Processor failed for %q: %v", o.ActualPath, pErr)
		}
	}

	for _, o := range obj.Objects {
		targetPath := utils.ResolveTargetPath(tmpDir, o)
		data, readErr := os.ReadFile(targetPath)
		if readErr != nil {
			t.Fatalf("failed to read downloaded file %q: %v", targetPath, readErr)
		}
		want := mock.objects[o.ActualPath]
		if !bytes.Equal(data, want) {
			t.Errorf("content mismatch for %q: got %q, want %q", o.ActualPath, data, want)
		}
	}
}

func TestFetcher_Fetch_FactoryError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	wantErr := errors.New("factory failed")
	clientFactory := func(ctx context.Context, opts types.GCSOptions) (gcs.Client, error) {
		return nil, wantErr
	}

	fetcherInstance := gcs.NewFetcher(clientFactory, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	_, err := fetcherInstance.Fetch(ctx, "/tmp", types.Source{
		Type: "gcs",
		GCS:  &types.GCSOptions{Bucket: "b", Paths: []string{"p"}},
	})
	if !errors.Is(err, wantErr) {
		t.Errorf("expected factory error %v, got %v", wantErr, err)
	}
}

func TestFetcher_Fetch_ListObjectsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mock := &mockClient{failOnList: true, listErr: errors.New("list error")}
	clientFactory := func(ctx context.Context, opts types.GCSOptions) (gcs.Client, error) {
		return mock, nil
	}

	fetcherInstance := gcs.NewFetcher(clientFactory, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	_, err := fetcherInstance.Fetch(ctx, "/tmp", types.Source{
		Type: "gcs",
		GCS:  &types.GCSOptions{Bucket: "b", Paths: []string{"p"}},
	})
	if err == nil || !strings.Contains(err.Error(), "failed to list objects") {
		t.Errorf("expected list error, got %v", err)
	}
}

func TestFetcher_Processor_GetObjectError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mock := &mockClient{
		objects:   map[string][]byte{"file.txt": []byte("data")},
		failOnGet: true,
		getErr:    errors.New("get error"),
	}
	clientFactory := func(ctx context.Context, opts types.GCSOptions) (gcs.Client, error) {
		return mock, nil
	}

	fetcherInstance := gcs.NewFetcher(clientFactory, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	obj, err := fetcherInstance.Fetch(ctx, t.TempDir(), types.Source{
		Type: "gcs",
		GCS:  &types.GCSOptions{Bucket: "b", Paths: []string{"file.txt"}},
	})
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if pErr := obj.Processor(ctx, obj.Objects[0]); pErr == nil || !strings.Contains(pErr.Error(), "failed to get object") {
		t.Errorf("expected get error, got %v", pErr)
	}
}
