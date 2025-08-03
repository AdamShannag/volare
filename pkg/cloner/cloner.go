package cloner

import (
	"github.com/AdamShannag/volare/pkg/utils"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/transport/http"
	"log/slog"
)

type Options struct {
	Path     string
	URL      string
	Username string
	Password string
	Ref      string
	Remote   string
}

type Cloner interface {
	Clone() error
}

type Factory interface {
	NewCloner(options Options) Cloner
}

type plainCloner interface {
	PlainClone(path string, opts *git.CloneOptions) (*git.Repository, error)
}

type goGitCloner struct{}

func (goGitCloner) PlainClone(path string, opts *git.CloneOptions) (*git.Repository, error) {
	return git.PlainClone(path, opts)
}

type gitClonerFactory struct{}

func NewGitClonerFactory() Factory {
	return &gitClonerFactory{}
}

func (f *gitClonerFactory) NewCloner(options Options) Cloner {
	return &gitCloner{
		options:     options,
		plainCloner: goGitCloner{},
	}
}

type gitCloner struct {
	options     Options
	plainCloner plainCloner
}

func (g *gitCloner) Clone() error {
	opts := &git.CloneOptions{
		URL:          g.options.URL,
		Depth:        1,
		SingleBranch: true,
	}

	if g.options.Password != "" {
		opts.Auth = &http.BasicAuth{
			Username: utils.FromEnv(g.options.Username),
			Password: utils.FromEnv(g.options.Password),
		}
	}

	if g.options.Ref != "" {
		opts.ReferenceName = plumbing.NewBranchReferenceName(g.options.Ref)
	}

	if g.options.Remote != "" {
		opts.RemoteName = g.options.Remote
	}

	slog.Info("cloning git repository", "url", g.options.URL, "path", g.options.Path)
	_, err := g.plainCloner.PlainClone(g.options.Path, opts)
	if err != nil {
		slog.Error("failed to clone repository", "url", g.options.URL, "path", g.options.Path, "error", err)
	}
	return err
}
