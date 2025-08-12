package git

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/AdamShannag/volare/pkg/cloner"
	"github.com/AdamShannag/volare/pkg/fetcher"
	"github.com/AdamShannag/volare/pkg/types"
	"github.com/AdamShannag/volare/pkg/utils"
)

type Fetcher struct {
	clonerFactory cloner.Factory
	logger        *slog.Logger
}

type filePath struct {
	Absolute string
	Relative string
}

func NewFetcher(factory cloner.Factory, logger *slog.Logger) fetcher.Fetcher {
	return &Fetcher{
		clonerFactory: factory,
		logger:        logger,
	}
}

func (f *Fetcher) Fetch(_ context.Context, mountPath string, src types.Source) (*fetcher.Object, error) {
	tempDir, err := os.MkdirTemp("", "gitclone-*")
	if err != nil {
		f.logger.Error("failed to create temp dir", "error", err)
		return nil, err
	}

	f.logger.Info("cloning git repository", "url", src.Git.Url)
	if err = f.clonerFactory.NewCloner(cloner.Options{
		Path:     tempDir,
		URL:      src.Git.Url,
		Username: src.Git.Username,
		Password: src.Git.Password,
		Ref:      src.Git.Ref,
		Remote:   src.Git.Remote,
	}).Clone(); err != nil {
		return nil, err
	}

	jobs, err := f.prepareJobs(tempDir, mountPath, src.Git.Paths)
	if err != nil {
		return nil, err
	}

	return &fetcher.Object{
		Processor: func(ctx context.Context, j types.ObjectToDownload) error {
			if copyErr := f.copy(j.Path, j.ActualPath); copyErr != nil {
				return copyErr
			}

			return nil
		},
		Objects: jobs,
		Workers: src.Git.Workers,
		Cleanup: func(ctx context.Context) error {
			f.logger.Info("cleaning up git clone", slog.String("url", src.Git.Url))
			return os.RemoveAll(tempDir)
		},
	}, nil
}

func (f *Fetcher) copy(src, dest string) error {
	f.logger.Info("copying file", "dest", dest)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("failed to create directory for %q: %w", dest, err)
	}

	inFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %q: %w", src, err)
	}
	defer func(inFile *os.File) {
		cerr := inFile.Close()
		if cerr != nil {
			f.logger.Warn("error closing file", "error", cerr)
		}
	}(inFile)

	outFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create destination file %q: %w", dest, err)
	}
	defer func() {
		if cerr := outFile.Close(); cerr != nil {
			f.logger.Warn("error closing file", "error", cerr)
		}
	}()

	if _, err = io.Copy(outFile, inFile); err != nil {
		return fmt.Errorf("failed to copy file to %q: %w", dest, err)
	}
	return nil
}

func (f *Fetcher) list(root, relBase string) ([]filePath, error) {
	var files []filePath

	startPath := filepath.Join(root, relBase)

	err := filepath.Walk(startPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			files = append(files, filePath{
				Absolute: path,
				Relative: relPath,
			})
		}
		return nil
	})

	return files, err
}

func (f *Fetcher) prepareJobs(tempDir, mountPath string, paths []string) ([]types.ObjectToDownload, error) {
	var jobs []types.ObjectToDownload

	for _, p := range paths {
		files, err := f.list(tempDir, p)
		if err != nil {
			return nil, err
		}

		for _, fl := range files {
			jobs = append(jobs, types.ObjectToDownload{
				Path: fl.Absolute,
				ActualPath: utils.ResolveTargetPath(mountPath, types.ObjectToDownload{
					ActualPath: fl.Relative,
					Path:       p,
				}),
			})
		}
	}

	return jobs, nil
}
