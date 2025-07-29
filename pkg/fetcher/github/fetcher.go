package github

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/AdamShannag/volare/pkg/downloader"
	"github.com/AdamShannag/volare/pkg/fetcher"
	"github.com/AdamShannag/volare/pkg/types"
	"github.com/AdamShannag/volare/pkg/workerpool"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

type Option func(*Fetcher)

type Fetcher struct {
	client     *http.Client
	downloader downloader.Downloader
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

	files, err := f.listFiles(ctx, *src.GitHub)
	if err != nil {
		return err
	}

	type job struct {
		item githubItem
	}

	processor := func(ctx context.Context, j job) error {
		return f.downloadBlob(ctx, mountPath, j.item, *src.GitHub)
	}

	numOfWorkers := types.DefaultNumberOfWorkers
	if src.GitHub.Workers != nil {
		numOfWorkers = *src.GitHub.Workers
	}

	pool := workerpool.New(ctx, numOfWorkers, len(files), processor)
	pool.Start()

	for _, file := range files {
		if file.Type != "blob" {
			continue
		}
		if err = pool.Submit(job{item: file}); err != nil {
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

func (f *Fetcher) listFiles(ctx context.Context, ghOpts types.GitHubOptions) ([]githubItem, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1",
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
		req.Header.Add("Authorization", "Bearer "+ghOpts.Token)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list GitHub tree: %w", err)
	}
	defer func() {
		if err = resp.Body.Close(); err != nil {
			slog.Warn("error closing response body", "error", err)
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
	trimPrefix := strings.Trim(ghOpts.Path, "/") + "/"
	for _, item := range tree.Tree {
		if item.Type != "blob" {
			continue
		}
		if ghOpts.Path == "" || strings.HasPrefix(item.Path, trimPrefix) || item.Path == ghOpts.Path {
			filtered = append(filtered, item)
		}
	}

	return filtered, nil
}

func (f *Fetcher) downloadBlob(ctx context.Context, mountPath string, item githubItem, ghOpts types.GitHubOptions) error {
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s",
		ghOpts.Owner,
		ghOpts.Repo,
		ghOpts.Ref,
		item.Path,
	)

	slog.Info("downloading file from github", slog.String("url", rawURL))

	headers := map[string]string{}
	if ghOpts.Token != "" {
		headers["Authorization"] = "Bearer " + ghOpts.Token
	}

	fullPath := filepath.Join(mountPath, item.Path)
	return f.downloader.Download(ctx, rawURL, headers, fullPath)
}
