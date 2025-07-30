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
	"strings"
)

const gitlabTokenHeader = "PRIVATE-TOKEN"

type Option func(*Fetcher)

type Fetcher struct {
	client     *http.Client
	downloader downloader.Downloader
}

type File struct {
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

	var filesToDownload []types.ObjectToDownload
	for _, p := range src.Gitlab.Paths {
		if utils.IsFile(p) {
			filesToDownload = append(filesToDownload, types.ObjectToDownload{
				Path:       p,
				ActualPath: strings.TrimPrefix(p, "/"),
			})
			continue
		}

		files, err := f.listFiles(ctx, *src.Gitlab, p)
		if err != nil {
			return fmt.Errorf("listing GitLab path %q: %w", p, err)
		}

		for _, fl := range files {
			if fl.Type == "blob" {
				filesToDownload = append(filesToDownload, types.ObjectToDownload{
					Path:       p,
					ActualPath: fl.Path,
				})
			}
		}
	}

	type job struct {
		file types.ObjectToDownload
	}

	processor := func(ctx context.Context, j job) error {
		return f.downloadBlob(ctx, mountPath, j.file, *src.Gitlab)
	}

	numOfWorkers := types.DefaultNumberOfWorkers
	if src.Gitlab.Workers != nil {
		numOfWorkers = *src.Gitlab.Workers
	}

	pool := workerpool.New(ctx, numOfWorkers, len(filesToDownload), processor)
	pool.Start()

	for _, fl := range filesToDownload {
		if err := pool.Submit(job{file: fl}); err != nil {
			pool.Cancel()
			pool.Stop()
			return err
		}
	}

	pool.Stop()

	for err := range pool.Errors() {
		if err != nil {
			return err
		}
	}

	return nil
}

func (f *Fetcher) listFiles(ctx context.Context, gitlabOpts types.GitlabOptions, path string) ([]File, error) {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/repository/tree?path=%s&ref=%s&recursive=true",
		gitlabOpts.Host,
		url.PathEscape(gitlabOpts.Project),
		url.QueryEscape(path),
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

	var files []File
	if err = json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("failed to decode tree: %w", err)
	}

	return files, nil
}

func (f *Fetcher) downloadBlob(ctx context.Context, mountPath string, file types.ObjectToDownload, src types.GitlabOptions) error {
	fileURL := fmt.Sprintf("%s/api/v4/projects/%s/repository/files/%s/raw?ref=%s",
		src.Host,
		url.PathEscape(src.Project),
		url.PathEscape(file.ActualPath),
		url.QueryEscape(src.Ref),
	)

	slog.Info("downloading file from url", slog.String("url", fileURL))

	headers := map[string]string{}
	if src.Token != "" {
		headers[gitlabTokenHeader] = utils.FromEnv(src.Token)
	}

	return f.downloader.Download(ctx, fileURL, headers, utils.ResolveTargetPath(mountPath, file))
}
