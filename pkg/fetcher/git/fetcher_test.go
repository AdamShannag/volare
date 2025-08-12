package git_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/AdamShannag/volare/pkg/cloner"
	"github.com/AdamShannag/volare/pkg/fetcher/git"
	"github.com/AdamShannag/volare/pkg/types"
)

type mockCloner struct {
	cloneCalled bool
	options     cloner.Options
	err         error
	createFiles func(baseDir string) error
}

func (m *mockCloner) Clone() error {
	if m.createFiles != nil {
		if err := m.createFiles(m.options.Path); err != nil {
			return err
		}
	}
	m.cloneCalled = true
	return m.err
}

type mockFactory struct {
	cloner *mockCloner
}

func (m *mockFactory) NewCloner(opts cloner.Options) cloner.Cloner {
	m.cloner.options = opts
	return m.cloner
}

func TestFetcher_FetchAndProcess(t *testing.T) {
	t.Parallel()

	mock := &mockCloner{
		createFiles: func(baseDir string) error {
			subdir := filepath.Join(baseDir, "subdir")
			if err := os.MkdirAll(subdir, 0755); err != nil {
				return err
			}
			file := filepath.Join(subdir, "file.txt")
			return os.WriteFile(file, []byte("hello world"), 0644)
		},
	}

	f := git.NewFetcher(&mockFactory{cloner: mock}, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	destDir := t.TempDir()
	src := types.Source{
		Git: &types.GitOptions{
			Url:   "https://example.com/repo.git",
			Paths: []string{"subdir"},
		},
	}

	obj, err := f.Fetch(context.Background(), destDir, src)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if !mock.cloneCalled {
		t.Errorf("expected Clone to be called")
	}

	if len(obj.Objects) != 1 {
		t.Fatalf("expected 1 job, got %d", len(obj.Objects))
	}

	for _, job := range obj.Objects {
		if err = obj.Processor(context.Background(), job); err != nil {
			t.Fatalf("Processor failed: %v", err)
		}
	}

	err = obj.Cleanup(context.Background())
	if err != nil {
		t.Fatalf("cleanup err: %v", err)
	}

	expectedFile := filepath.Join(destDir, "file.txt")
	if _, err = os.Stat(expectedFile); err != nil {
		t.Errorf("expected file at %s but got error: %v", expectedFile, err)
	}
}

func TestFetcher_Fetch_CloneError(t *testing.T) {
	t.Parallel()

	mock := &mockCloner{
		err: os.ErrPermission,
	}
	f := git.NewFetcher(&mockFactory{cloner: mock}, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	src := types.Source{
		Git: &types.GitOptions{
			Url:   "https://example.com/repo.git",
			Paths: []string{"subdir"},
		},
	}

	_, err := f.Fetch(context.Background(), t.TempDir(), src)
	if err == nil {
		t.Errorf("expected clone error, got nil")
	}
}

func TestFetcher_Fetch_PrepareJobsError(t *testing.T) {
	t.Parallel()

	mock := &mockCloner{
		createFiles: func(baseDir string) error {
			return nil
		},
	}
	f := git.NewFetcher(&mockFactory{cloner: mock}, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	src := types.Source{
		Git: &types.GitOptions{
			Url:   "https://example.com/repo.git",
			Paths: []string{"does-not-exist"},
		},
	}

	_, err := f.Fetch(context.Background(), t.TempDir(), src)
	if err == nil {
		t.Errorf("expected prepareJobs error, got nil")
	}
}
