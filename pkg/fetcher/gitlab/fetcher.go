package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/AdamShannag/volare/pkg/downloader"
	"github.com/AdamShannag/volare/pkg/fetcher"
	"github.com/AdamShannag/volare/pkg/types"
	"github.com/AdamShannag/volare/pkg/utils"
	"github.com/AdamShannag/volare/pkg/workerpool"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
)

const gitlabTokenHeader = "PRIVATE-TOKEN"

type Option func(*Fetcher)

type Fetcher struct {
	client     *http.Client
	downloader downloader.Downloader
}

type gitlabFile struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
	Mode string `json:"mode"`
}

func WithHTTPClient(client *http.Client) Option {
	return func(h *Fetcher) {
		h.client = client
	}
}

func NewFetcher(downloader downloader.Downloader, opts ...Option) fetcher.Fetcher {
	h := &Fetcher{
		client: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(h)
	}
	h.downloader = downloader
	return h
}
func (f *Fetcher) Fetch(ctx context.Context, mountPath string, src types.Source) error {
	if src.Gitlab == nil {
		return fmt.Errorf("invalid source configuration: 'gitlab' options must be provided for source type 'gitlab'")
	}

	files, err := f.listFiles(ctx, *src.Gitlab)
	if err != nil {
		return err
	}

	type job struct {
		filePath string
	}

	processor := func(ctx context.Context, j job) error {
		return f.downloadBlob(ctx, mountPath, j.filePath, *src.Gitlab)
	}

	numOfWorkers := types.DefaultNumberOfWorkers
	if src.Gitlab.Workers != nil {
		numOfWorkers = *src.Gitlab.Workers
	}

	pool := workerpool.New(ctx, numOfWorkers, len(files), processor)
	pool.Start()

	for _, file := range files {
		if file.Type != "blob" {
			continue
		}
		if err = pool.Submit(job{filePath: file.Path}); err != nil {
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

func (f *Fetcher) listFiles(ctx context.Context, gitlabOpts types.GitlabOptions) ([]gitlabFile, error) {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/repository/tree?path=%s&ref=%s&recursive=true",
		gitlabOpts.Host,
		url.PathEscape(gitlabOpts.Project),
		url.QueryEscape(gitlabOpts.Path),
		url.QueryEscape(gitlabOpts.Ref),
	)

	slog.Info("listing files from gitlab", slog.String("url", apiURL))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if gitlabOpts.Token != "" {
		req.Header.Add(gitlabTokenHeader, utils.FromEnv(gitlabOpts.Token))
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list GitLab repo tree: %w", err)
	}
	defer func() {
		if err = resp.Body.Close(); err != nil {
			slog.Warn("error closing response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list tree: status %d", resp.StatusCode)
	}

	var files []gitlabFile
	if err = json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("failed to decode tree: %w", err)
	}

	return files, nil
}

func (f *Fetcher) downloadBlob(ctx context.Context, mountPath, filePath string, src types.GitlabOptions) error {
	fileURL := fmt.Sprintf("%s/api/v4/projects/%s/repository/files/%s/raw?ref=%s",
		src.Host,
		url.PathEscape(src.Project),
		url.PathEscape(filePath),
		url.QueryEscape(src.Ref),
	)

	slog.Info("downloading file from url", slog.String("url", fileURL))

	headers := map[string]string{}
	if src.Token != "" {
		headers[gitlabTokenHeader] = utils.FromEnv(src.Token)
	}

	fullPath := filepath.Join(mountPath, filePath)
	return f.downloader.Download(ctx, fileURL, headers, fullPath)
}
