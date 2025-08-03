package git_test

import (
	"context"
	"github.com/AdamShannag/volare/pkg/cloner"
	"github.com/AdamShannag/volare/pkg/fetcher/git"
	"github.com/AdamShannag/volare/pkg/types"
	"os"
	"path/filepath"
	"testing"
)

type mockCloner struct {
	cloneCalled bool
	err         error
	options     cloner.Options
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

func TestFetcher_Fetch_UsesRealGetFiles(t *testing.T) {
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
	f := git.NewFetcher(&mockFactory{cloner: mock})

	destDir := t.TempDir()
	src := types.Source{
		Git: &types.GitOptions{
			Url:   "https://example.com/repo.git",
			Paths: []string{"subdir"},
		},
	}

	err := f.Fetch(context.Background(), destDir, src)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if !mock.cloneCalled {
		t.Errorf("expected Clone() to be called")
	}

	expectedFile := filepath.Join(destDir, "file.txt")
	if _, err = os.Stat(expectedFile); err != nil {
		t.Errorf("expected file to be copied to %s but got error: %v", expectedFile, err)
	}
}

func TestFetcher_Fetch_InvalidSource(t *testing.T) {
	t.Parallel()

	f := git.NewFetcher(&mockFactory{cloner: &mockCloner{}})
	src := types.Source{}

	err := f.Fetch(context.Background(), t.TempDir(), src)
	if err == nil || err.Error() != "invalid source configuration: 'git' options must be provided for source type 'git'" {
		t.Errorf("expected invalid source configuration error, got %v", err)
	}
}

func TestFetcher_Fetch_CloneError(t *testing.T) {
	t.Parallel()

	mock := &mockCloner{
		err:         os.ErrPermission,
		createFiles: nil,
	}
	f := git.NewFetcher(&mockFactory{cloner: mock})

	src := types.Source{
		Git: &types.GitOptions{
			Url:   "https://example.com/repo.git",
			Paths: []string{"subdir"},
		},
	}

	err := f.Fetch(context.Background(), t.TempDir(), src)
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
	f := git.NewFetcher(&mockFactory{cloner: mock})

	src := types.Source{
		Git: &types.GitOptions{
			Url:   "https://example.com/repo.git",
			Paths: []string{"nonexistentpath"},
		},
	}

	err := f.Fetch(context.Background(), t.TempDir(), src)
	if err == nil {
		t.Errorf("expected error due to prepareJobs failure, got nil")
	}
}

func TestFetcher_Fetch_MultiplePaths(t *testing.T) {
	t.Parallel()

	mock := &mockCloner{
		createFiles: func(baseDir string) error {
			for _, p := range []string{"path1", "path2"} {
				dir := filepath.Join(baseDir, p)
				if err := os.MkdirAll(dir, 0755); err != nil {
					return err
				}
				file := filepath.Join(dir, "file.txt")
				if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
					return err
				}
			}
			return nil
		},
	}
	f := git.NewFetcher(&mockFactory{cloner: mock})

	destDir := t.TempDir()
	src := types.Source{
		Git: &types.GitOptions{
			Url:   "https://example.com/repo.git",
			Paths: []string{"path1", "path2"},
		},
	}

	err := f.Fetch(context.Background(), destDir, src)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	expectedFile := filepath.Join(destDir, "file.txt")
	if _, err = os.Stat(expectedFile); err != nil {
		t.Errorf("expected file %s to exist, got error: %v", expectedFile, err)
	}
}

func TestFetcher_Fetch_CustomWorkers(t *testing.T) {
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
	f := git.NewFetcher(&mockFactory{cloner: mock})

	destDir := t.TempDir()
	workers := 2
	src := types.Source{
		Git: &types.GitOptions{
			Url:     "https://example.com/repo.git",
			Paths:   []string{"subdir"},
			Workers: &workers,
		},
	}

	err := f.Fetch(context.Background(), destDir, src)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if !mock.cloneCalled {
		t.Errorf("expected Clone() to be called")
	}

	expectedFile := filepath.Join(destDir, "file.txt")
	if _, err = os.Stat(expectedFile); err != nil {
		t.Errorf("expected file to be copied to %s but got error: %v", expectedFile, err)
	}
}
