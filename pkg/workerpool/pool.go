package workerpool

import (
	"context"
	"log/slog"
	"sync"
)

type JobFunc[T any] func(ctx context.Context, job T) error

type WorkerPool[T any] struct {
	workerCount int
	jobs        chan T
	errs        chan error
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	processor   JobFunc[T]
}

func New[T any](ctx context.Context, workerCount int, jobBuffer int, processor JobFunc[T]) *WorkerPool[T] {
	ctx, cancel := context.WithCancel(ctx)

	return &WorkerPool[T]{
		workerCount: workerCount,
		jobs:        make(chan T, jobBuffer),
		errs:        make(chan error, jobBuffer),
		ctx:         ctx,
		cancel:      cancel,
		processor:   processor,
	}
}

func (p *WorkerPool[T]) Start() {
	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

func (p *WorkerPool[T]) worker(id int) {
	defer p.wg.Done()
	for {
		select {
		case job, ok := <-p.jobs:
			if !ok {
				return
			}
			if err := p.processor(p.ctx, job); err != nil {
				slog.Error("error processing job", "workerID", id, "error", err)
				p.errs <- err
			}
		case <-p.ctx.Done():
			slog.Info("context canceled, exiting", "workerID", id)
			return
		}
	}
}

func (p *WorkerPool[T]) Submit(job T) error {
	select {
	case p.jobs <- job:
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

func (p *WorkerPool[T]) Stop() {
	close(p.jobs)
	p.wg.Wait()
	close(p.errs)
}

func (p *WorkerPool[T]) Errors() <-chan error {
	return p.errs
}

func (p *WorkerPool[T]) Cancel() {
	p.cancel()
}
