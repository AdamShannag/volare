package gcs_test

import (
	"bytes"
	"context"
	"errors"
	"github.com/AdamShannag/volare/pkg/fetcher/gcs"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/AdamShannag/volare/pkg/types"
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
			if len(v) == 0 && strings.HasSuffix(k, "/") {
				continue
			}
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

	fetcher := gcs.NewFetcher(clientFactory)

	src := types.Source{
		Type: "gcs",
		GCS: &types.GCSOptions{
			Bucket: "bucket1",
			Paths:  []string{"file1.txt", "dir"},
		},
	}

	tmpDir := t.TempDir()

	err := fetcher.Fetch(ctx, tmpDir, src)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if len(mock.listCalls) != len(src.GCS.Paths) {
		t.Errorf("expected ListObjects calls %d, got %d", len(src.GCS.Paths), len(mock.listCalls))
	}
}

func TestFetcher_Fetch_Errors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	fetcher := gcs.NewFetcher(nil)
	err := fetcher.Fetch(ctx, "/tmp", types.Source{Type: "gcs"})
	if err == nil || !strings.Contains(err.Error(), "'gcs' options must be provided") {
		t.Errorf("expected error about missing gcs options, got %v", err)
	}

	wantErr := errors.New("factory failed")
	clientFactory := func(ctx context.Context, opts types.GCSOptions) (gcs.Client, error) {
		return nil, wantErr
	}
	fetcher = gcs.NewFetcher(clientFactory)
	err = fetcher.Fetch(ctx, "/tmp", types.Source{Type: "gcs", GCS: &types.GCSOptions{Bucket: "b", Paths: []string{"p"}}})
	if !errors.Is(err, wantErr) {
		t.Errorf("expected factory error %v, got %v", wantErr, err)
	}

	mock := &mockClient{failOnList: true, listErr: errors.New("list error")}
	clientFactory = func(ctx context.Context, opts types.GCSOptions) (gcs.Client, error) {
		return mock, nil
	}
	fetcher = gcs.NewFetcher(clientFactory)
	err = fetcher.Fetch(ctx, "/tmp", types.Source{Type: "gcs", GCS: &types.GCSOptions{Bucket: "b", Paths: []string{"p"}}})
	if err == nil || !strings.Contains(err.Error(), "failed to list objects") {
		t.Errorf("expected list error, got %v", err)
	}

	mock = &mockClient{
		objects: map[string][]byte{"file.txt": []byte("data")},
	}
	clientFactory = func(ctx context.Context, opts types.GCSOptions) (gcs.Client, error) {
		return mock, nil
	}
	fetcher = gcs.NewFetcher(clientFactory)
	invalidMountPath := string([]byte{0})
	err = fetcher.Fetch(ctx, invalidMountPath, types.Source{Type: "gcs", GCS: &types.GCSOptions{Bucket: "b", Paths: []string{"file.txt"}}})
	if err == nil {
		t.Errorf("expected error due to invalid mount path, got nil")
	}
}
