package cloner

import (
	"errors"
	"testing"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	gitHttp "github.com/go-git/go-git/v6/plumbing/transport/http"
)

type mockCloner struct {
	lastPath string
	lastOpts *git.CloneOptions
	err      error
}

func (m *mockCloner) PlainClone(path string, opts *git.CloneOptions) (*git.Repository, error) {
	m.lastPath = path
	m.lastOpts = opts
	return nil, m.err
}

func TestFactoryCreatesCloner(t *testing.T) {
	factory := NewGitClonerFactory()
	cloner := factory.NewCloner(Options{
		Path: "some-path",
		URL:  "https://example.com/repo.git",
	})
	if cloner == nil {
		t.Fatal("expected non-nil cloner")
	}
}

func TestClone_Success(t *testing.T) {
	mock := &mockCloner{}
	c := &gitCloner{
		options: Options{
			Path: "repo",
			URL:  "https://example.com/repo.git",
		},
		plainCloner: mock,
	}

	err := c.Clone()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if mock.lastPath != "repo" {
		t.Errorf("expected path 'repo', got %q", mock.lastPath)
	}
	if mock.lastOpts.URL != "https://example.com/repo.git" {
		t.Errorf("expected URL 'https://example.com/repo.git', got %q", mock.lastOpts.URL)
	}
	if !mock.lastOpts.SingleBranch || mock.lastOpts.Depth != 1 {
		t.Error("expected single branch and depth = 1")
	}
}

func TestClone_WithAuth(t *testing.T) {
	mock := &mockCloner{}
	c := &gitCloner{
		options: Options{
			Path:     "repo",
			URL:      "https://example.com/repo.git",
			Username: "foo",
			Password: "bar",
		},
		plainCloner: mock,
	}

	_ = c.Clone()

	auth, ok := mock.lastOpts.Auth.(*gitHttp.BasicAuth)
	if !ok {
		t.Fatalf("expected Auth to be *BasicAuth, got %T", mock.lastOpts.Auth)
	}
	if auth.Username != "foo" || auth.Password != "bar" {
		t.Errorf("unexpected auth credentials: %v", auth)
	}
}

func TestClone_WithRefAndRemote(t *testing.T) {
	mock := &mockCloner{}
	c := &gitCloner{
		options: Options{
			Path:   "repo",
			URL:    "https://example.com/repo.git",
			Ref:    "dev",
			Remote: "upstream",
		},
		plainCloner: mock,
	}

	_ = c.Clone()

	expectedRef := plumbing.NewBranchReferenceName("dev")
	if mock.lastOpts.ReferenceName != expectedRef {
		t.Errorf("expected ref %q, got %q", expectedRef, mock.lastOpts.ReferenceName)
	}
	if mock.lastOpts.RemoteName != "upstream" {
		t.Errorf("expected remote 'upstream', got %q", mock.lastOpts.RemoteName)
	}
}

func TestClone_Failure(t *testing.T) {
	mock := &mockCloner{
		err: errors.New("clone failed"),
	}
	c := &gitCloner{
		options: Options{
			Path: "repo",
			URL:  "https://example.com/repo.git",
		},
		plainCloner: mock,
	}

	err := c.Clone()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "clone failed" {
		t.Errorf("unexpected error: %v", err)
	}
}
