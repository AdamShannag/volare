package workerpool_test

import (
	"context"
	"errors"
	"github.com/AdamShannag/volare/pkg/workerpool"
	"sync/atomic"
	"testing"
	"time"
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
