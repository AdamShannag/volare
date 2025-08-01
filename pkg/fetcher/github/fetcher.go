package github

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

type Option func(*Fetcher)

type Fetcher struct {
	client     *http.Client
	downloader downloader.Downloader
	baseURL    string
}

func WithHTTPClient(client *http.Client) Option {
	return func(h *Fetcher) {
		h.client = client
	}
}

func WithBaseURL(baseURL string) Option {
	return func(f *Fetcher) {
		f.baseURL = baseURL
	}
}

func NewFetcher(downloader downloader.Downloader, opts ...Option) fetcher.Fetcher {
	h := &Fetcher{
		client:  http.DefaultClient,
		baseURL: "https://api.github.com",
	}
	for _, opt := range opts {
		opt(h)
	}
	h.downloader = downloader
	return h
}

type githubResponse struct {
	Tree []githubItem `json:"tree"`
}

type githubItem struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

func (f *Fetcher) Fetch(ctx context.Context, mountPath string, src types.Source) error {
	if src.GitHub == nil {
		return fmt.Errorf("invalid source configuration: 'github' options must be provided for source type 'github'")
	}

	var filesToDownload []types.ObjectToDownload
	for _, p := range src.GitHub.Paths {
		if utils.IsFile(p) {
			filesToDownload = append(filesToDownload, types.ObjectToDownload{
				Path:       p,
				ActualPath: strings.TrimPrefix(p, "/"),
			})
			continue
		}

		files, err := f.listFiles(ctx, *src.GitHub, p)
		if err != nil {
			return err
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
		return f.downloadBlob(ctx, mountPath, j.file, *src.GitHub)
	}

	numOfWorkers := types.DefaultNumberOfWorkers
	if src.GitHub.Workers != nil {
		numOfWorkers = *src.GitHub.Workers
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

func (f *Fetcher) listFiles(ctx context.Context, ghOpts types.GitHubOptions, path string) ([]githubItem, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/git/trees/%s?recursive=1",
		f.baseURL,
		url.PathEscape(ghOpts.Owner),
		url.PathEscape(ghOpts.Repo),
		url.PathEscape(ghOpts.Ref),
	)

	slog.Info("listing files from github", slog.String("url", apiURL))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if ghOpts.Token != "" {
		req.Header.Add("Authorization", "Bearer "+utils.FromEnv(ghOpts.Token))
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list GitHub tree: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			slog.Warn("error closing response body", "error", cerr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var tree githubResponse
	if err = json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return nil, fmt.Errorf("failed to decode tree: %w", err)
	}

	var filtered []githubItem
	trimPrefix := strings.Trim(path, "/") + "/"
	for _, item := range tree.Tree {
		if item.Type != "blob" {
			continue
		}
		if path == "" || strings.HasPrefix(item.Path, trimPrefix) || item.Path == path {
			filtered = append(filtered, item)
		}
	}

	return filtered, nil
}

func (f *Fetcher) downloadBlob(ctx context.Context, mountPath string, file types.ObjectToDownload, ghOpts types.GitHubOptions) error {
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s",
		ghOpts.Owner,
		ghOpts.Repo,
		ghOpts.Ref,
		file.ActualPath,
	)

	headers := map[string]string{}
	if ghOpts.Token != "" {
		headers["Authorization"] = "Bearer " + utils.FromEnv(ghOpts.Token)
	}

	return f.downloader.Download(ctx, rawURL, headers, utils.ResolveTargetPath(mountPath, file))
}
