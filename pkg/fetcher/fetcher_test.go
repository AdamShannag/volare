package fetcher_test

import (
	"context"
	"testing"

	"github.com/AdamShannag/volare/pkg/fetcher"
	"github.com/AdamShannag/volare/pkg/types"
)

type MockFetcher struct {
	called bool
}

func (m *MockFetcher) Fetch(_ context.Context, _ string, _ types.Source) error {
	m.called = true
	return nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := fetcher.NewRegistry()
	mock := &MockFetcher{}

	err := reg.Register("mock", mock)
	if err != nil {
		t.Fatalf("unexpected error registering fetcher: %v", err)
	}

	f, err := reg.Get("mock")
	if err != nil {
		t.Fatalf("unexpected error getting fetcher: %v", err)
	}

	if f != mock {
		t.Errorf("got wrong fetcher instance: got %v, want %v", f, mock)
	}
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	reg := fetcher.NewRegistry()
	mock := &MockFetcher{}
	_ = reg.Register("mock", mock)

	err := reg.Register("mock", &MockFetcher{})
	if err == nil {
		t.Fatal("expected error on duplicate register, got nil")
	}
	expected := "fetcher already registered for type mock"
	if err.Error() != expected {
		t.Errorf("unexpected error message: got %q, want %q", err.Error(), expected)
	}
}

func TestRegistry_GetUnknownFetcher(t *testing.T) {
	reg := fetcher.NewRegistry()
	_, err := reg.Get("unknown")
	if err == nil {
		t.Fatal("expected error when getting unregistered fetcher, got nil")
	}
	expected := "no fetcher registered for type unknown"
	if err.Error() != expected {
		t.Errorf("unexpected error message: got %q, want %q", err.Error(), expected)
	}
}
