package git

import (
	"context"
	"fmt"
	"github.com/AdamShannag/volare/pkg/cloner"
	"github.com/AdamShannag/volare/pkg/fetcher"
	"github.com/AdamShannag/volare/pkg/types"
	"github.com/AdamShannag/volare/pkg/utils"
	"github.com/AdamShannag/volare/pkg/workerpool"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

type Fetcher struct {
	clonerFactory cloner.Factory
}

type job struct {
	srcPath  string
	destPath string
}

type filePath struct {
	Absolute string
	Relative string
}

func NewFetcher(factory cloner.Factory) fetcher.Fetcher {
	return &Fetcher{factory}
}

func (f *Fetcher) Fetch(ctx context.Context, mountPath string, src types.Source) error {
	if src.Git == nil {
		return fmt.Errorf("invalid source configuration: 'git' options must be provided for source type 'git'")
	}

	tempDir, err := os.MkdirTemp("", "gitclone-*")
	if err != nil {
		slog.Error("failed to create temp dir", "error", err)
		return err
	}
	defer func(path string) {
		if rErr := os.RemoveAll(path); rErr != nil {
			slog.Error("cleanup failed", "error", rErr)
		}
	}(tempDir)

	if err = f.clonerFactory.NewCloner(cloner.Options{
		Path:     tempDir,
		URL:      src.Git.Url,
		Username: src.Git.Username,
		Password: src.Git.Password,
		Ref:      src.Git.Ref,
		Remote:   src.Git.Remote,
	}).Clone(); err != nil {
		return err
	}

	jobs, err := f.prepareJobs(tempDir, mountPath, src.Git.Paths)
	if err != nil {
		return err
	}

	processor := func(ctx context.Context, j job) error {
		return f.copy(j.srcPath, j.destPath)
	}

	numOfWorkers := types.DefaultNumberOfWorkers
	if src.Git.Workers != nil {
		numOfWorkers = *src.Git.Workers
	}

	pool := workerpool.New(ctx, numOfWorkers, len(jobs), processor)
	pool.Start()

	for _, fl := range jobs {
		if err = pool.Submit(fl); err != nil {
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

func (f *Fetcher) prepareJobs(tempDir, mountPath string, paths []string) ([]job, error) {
	var jobs []job

	for _, p := range paths {
		files, err := f.getFiles(tempDir, p)
		if err != nil {
			return nil, err
		}

		for _, fl := range files {
			jobs = append(jobs, job{
				srcPath: fl.Absolute,
				destPath: utils.ResolveTargetPath(mountPath, types.ObjectToDownload{
					ActualPath: fl.Relative,
					Path:       p,
				}),
			})
		}
	}

	return jobs, nil
}

func (f *Fetcher) copy(src, dest string) error {
	slog.Info("Copying file from repo to destination", "dest", dest)
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
			slog.Warn("error closing file", "error", cerr)
		}
	}(inFile)

	outFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create destination file %q: %w", dest, err)
	}
	defer func() {
		if cerr := outFile.Close(); cerr != nil {
			slog.Warn("error closing file", "error", cerr)
		}
	}()

	if _, err = io.Copy(outFile, inFile); err != nil {
		return fmt.Errorf("failed to copy file to %q: %w", dest, err)
	}
	return nil
}

func (f *Fetcher) getFiles(root, relBase string) ([]filePath, error) {
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
