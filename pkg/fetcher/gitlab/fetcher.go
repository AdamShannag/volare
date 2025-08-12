package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/AdamShannag/volare/pkg/downloader"
	"github.com/AdamShannag/volare/pkg/fetcher"
	"github.com/AdamShannag/volare/pkg/types"
	"github.com/AdamShannag/volare/pkg/utils"
)

const gitlabTokenHeader = "PRIVATE-TOKEN"

type Option func(*Fetcher)

type Fetcher struct {
	client     *http.Client
	downloader downloader.Downloader
	logger     *slog.Logger
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

func NewFetcher(downloader downloader.Downloader, logger *slog.Logger, opts ...Option) fetcher.Fetcher {
	h := &Fetcher{
		client:     http.DefaultClient,
		downloader: downloader,
		logger:     logger,
	}
	for _, opt := range opts {
		opt(h)
	}

	return h
}

func (f *Fetcher) Fetch(ctx context.Context, mountPath string, src types.Source) (*fetcher.Object, error) {
	var filesToDownload []types.ObjectToDownload
	for _, p := range src.Gitlab.Paths {
		if utils.IsFile(p) {
			filesToDownload = append(filesToDownload, types.ObjectToDownload{
				Path:       p,
				ActualPath: strings.TrimPrefix(p, "/"),
			})
			continue
		}

		files, err := f.list(ctx, *src.Gitlab, p)
		if err != nil {
			return nil, fmt.Errorf("listing GitLab path %q: %w", p, err)
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

	return &fetcher.Object{
		Processor: func(ctx context.Context, j types.ObjectToDownload) error {
			return f.download(ctx, mountPath, j, *src.Gitlab)
		},
		Objects: filesToDownload,
		Workers: src.Gitlab.Workers,
	}, nil
}

func (f *Fetcher) list(ctx context.Context, gitlabOpts types.GitlabOptions, path string) ([]File, error) {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/repository/tree?path=%s&ref=%s&recursive=true",
		gitlabOpts.Host,
		url.PathEscape(gitlabOpts.Project),
		url.QueryEscape(path),
		url.QueryEscape(gitlabOpts.Ref),
	)

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
			f.logger.Warn("error closing response body", "error", err)
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

func (f *Fetcher) download(ctx context.Context, mountPath string, file types.ObjectToDownload, src types.GitlabOptions) error {
	fileURL := fmt.Sprintf("%s/api/v4/projects/%s/repository/files/%s/raw?ref=%s",
		src.Host,
		url.PathEscape(src.Project),
		url.PathEscape(file.ActualPath),
		url.QueryEscape(src.Ref),
	)

	headers := map[string]string{}
	if src.Token != "" {
		headers[gitlabTokenHeader] = utils.FromEnv(src.Token)
	}

	f.logger.Info("downloading file", slog.String("project", src.Project), slog.String("file", file.ActualPath))
	return f.downloader.Download(ctx, fileURL, headers, utils.ResolveTargetPath(mountPath, file))
}
