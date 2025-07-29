package fetcher

import (
	"context"
	"fmt"
	"github.com/AdamShannag/volare/pkg/types"
	"sync"
)

type Fetcher interface {
	Fetch(ctx context.Context, mountPath string, src types.Source) error
}

type Registry struct {
	mu       sync.RWMutex
	fetchers map[types.SourceType]Fetcher
}

func NewRegistry() *Registry {
	return &Registry{
		fetchers: make(map[types.SourceType]Fetcher),
	}
}

func (r *Registry) Register(sourceType types.SourceType, factory Fetcher) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.fetchers[sourceType]; exists {
		return fmt.Errorf("fetcher already registered for type %s", sourceType)
	}
	r.fetchers[sourceType] = factory
	return nil
}

func (r *Registry) Get(src types.SourceType) (Fetcher, error) {
	r.mu.RLock()
	factory, ok := r.fetchers[src]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no fetcher registered for type %s", src)
	}
	return factory, nil
}
