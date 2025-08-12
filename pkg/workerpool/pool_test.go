package workerpool_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AdamShannag/volare/pkg/workerpool"
)

func TestWorkerPoolProcessesJobs(t *testing.T) {
	t.Parallel()

	var processed int32

	ctx := context.Background()
	pool := workerpool.New(ctx, 3, 10, func(_ context.Context, job int) error {
		atomic.AddInt32(&processed, int32(job))
		return nil
	})

	pool.Start()
	for i := 0; i < 5; i++ {
		if err := pool.Submit(1); err != nil {
			t.Fatalf("failed to submit job: %v", err)
		}
	}
	pool.Stop()

	if processed != 5 {
		t.Errorf("expected 5 jobs to be processed, got %d", processed)
	}
}

func TestWorkerPoolHandlesErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := workerpool.New(ctx, 2, 5, func(_ context.Context, job int) error {
		if job == 42 {
			return errors.New("fail job")
		}
		return nil
	})

	pool.Start()
	_ = pool.Submit(1)
	_ = pool.Submit(42)
	pool.Stop()

	var gotErr error
	for err := range pool.Errors() {
		gotErr = err
		break
	}

	if gotErr == nil || gotErr.Error() != "fail job" {
		t.Errorf("expected error from job 42, got %v", gotErr)
	}
}

func TestWorkerPoolCancel(t *testing.T) {
	t.Parallel()

	var called int32
	started := make(chan struct{})

	ctx := context.Background()
	pool := workerpool.New(ctx, 1, 5, func(ctx context.Context, job int) error {
		atomic.AddInt32(&called, 1)
		close(started)
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	pool.Start()

	_ = pool.Submit(1)

	select {
	case <-started:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("job did not start in time")
	}

	pool.Cancel()
	pool.Stop()

	if called == 0 {
		t.Errorf("expected at least one job to be started")
	}
}

func TestWorkerPoolSubmitAfterCancel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := workerpool.New(ctx, 1, 2, func(ctx context.Context, job int) error {
		return nil
	})

	pool.Cancel()

	var err error
	ok := false
	for i := 0; i < 10; i++ {
		err = pool.Submit(1)
		if err != nil {
			ok = true
			break
		}
	}

	if !ok {
		t.Errorf("expected submit to fail after cancel, got nil error")
	}
}

func TestWorkerPoolMultipleErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := workerpool.New(ctx, 2, 5, func(_ context.Context, job int) error {
		return errors.New("error")
	})

	pool.Start()
	for i := 0; i < 3; i++ {
		_ = pool.Submit(i)
	}
	pool.Stop()

	var count int
	for range pool.Errors() {
		count++
	}
	if count != 3 {
		t.Errorf("expected 3 errors, got %d", count)
	}
}

func TestRunPool_Success(t *testing.T) {
	t.Parallel()

	var processed int32
	items := []int{1, 2, 3, 4, 5}
	workers := 3

	err := workerpool.RunPool(context.Background(), items, &workers, func(ctx context.Context, i int) error {
		atomic.AddInt32(&processed, 1)
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got := atomic.LoadInt32(&processed); got != int32(len(items)) {
		t.Fatalf("expected %d processed, got %d", len(items), got)
	}
}

func TestRunPool_ProcessorError(t *testing.T) {
	t.Parallel()

	items := []string{"a", "b", "c"}
	workers := 2

	err := workerpool.RunPool(context.Background(), items, &workers, func(ctx context.Context, s string) error {
		if s == "b" {
			return errors.New("processor failure")
		}
		return nil
	})
	if err == nil || err.Error() != "processing error: processor failure" {
		t.Fatalf("expected processor failure, got %v", err)
	}
}

func TestRunPool_EmptyItems(t *testing.T) {
	t.Parallel()

	err := workerpool.RunPool(context.Background(), []int{}, nil, func(ctx context.Context, i int) error {
		t.Fatal("processor should not be called")
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error on empty items, got %v", err)
	}
}

func TestRunPool_ContextCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	items := []int{1, 2, 3}
	workers := 2

	var processed int32
	_ = workerpool.RunPool(ctx, items, &workers, func(ctx context.Context, i int) error {
		if i == 2 {
			cancel()
			time.Sleep(10 * time.Millisecond)
		}
		atomic.AddInt32(&processed, 1)
		return nil
	})

	if got := atomic.LoadInt32(&processed); got == 0 {
		t.Fatal("expected some items processed before cancel")
	}
}

func TestRunPool_NilWorkers_UsesDefault(t *testing.T) {
	t.Parallel()

	var processed int32
	items := []int{1, 2}

	err := workerpool.RunPool(context.Background(), items, nil, func(ctx context.Context, i int) error {
		atomic.AddInt32(&processed, 1)
		return nil
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got := atomic.LoadInt32(&processed); got != int32(len(items)) {
		t.Fatalf("expected %d processed, got %d", len(items), got)
	}
}
