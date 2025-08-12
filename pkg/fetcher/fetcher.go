package fetcher

import (
	"context"
	"fmt"
	"sync"

	"github.com/AdamShannag/volare/pkg/types"
)

type Object struct {
	Processor func(context.Context, types.ObjectToDownload) error
	Objects   []types.ObjectToDownload
	Workers   *int
	Cleanup   func(context.Context) error
}

type RegistryItem struct {
	SourceType types.SourceType
	Factory    Fetcher
}

type Fetcher interface {
	Fetch(ctx context.Context, mountPath string, src types.Source) (*Object, error)
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

func (r *Registry) RegisterAll(items []RegistryItem) error {
	for _, item := range items {
		err := r.Register(item.SourceType, item.Factory)
		if err != nil {
			return err
		}
	}

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

func NewRegistryItem(sourceType types.SourceType, factory Fetcher) RegistryItem {
	return RegistryItem{sourceType, factory}
}
