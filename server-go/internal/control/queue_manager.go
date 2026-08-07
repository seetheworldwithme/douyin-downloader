package control

import (
	"context"
	"fmt"
	"sync"
)

// DownloadFunc processes a single item. It receives the context and the item
// and returns either a result or an error.
type DownloadFunc func(ctx context.Context, item interface{}) (interface{}, error)

// QueueManager bounds the concurrency of a batch of operations.
// Ported from control/queue_manager.py.
//
// In Python this used an asyncio.Semaphore; here we use a buffered channel of
// struct{} as the semaphore (a standard Go idiom).
type QueueManager struct {
	maxWorkers int
	sem        chan struct{}
}

// NewQueueManager constructs a manager that admits at most maxWorkers
// concurrent operations. maxWorkers <= 0 is clamped to 1.
func NewQueueManager(maxWorkers int) *QueueManager {
	if maxWorkers <= 0 {
		maxWorkers = 1
	}
	return &QueueManager{
		maxWorkers: maxWorkers,
		sem:        make(chan struct{}, maxWorkers),
	}
}

// acquire blocks until a semaphore slot is available or ctx is cancelled.
func (q *QueueManager) acquire(ctx context.Context) error {
	select {
	case q.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *QueueManager) release() {
	<-q.sem
}

// DownloadBatch runs downloadFunc over every item in items with at most
// maxWorkers concurrent invocations. It always returns one result per item,
// in input order. Failures surface as error values in the slice (mirroring the
// Python return_exceptions=True behaviour); a nil error element means success.
//
// Each result is either the function's return value (success) or an error
// (failure). Callers should type-switch each element:
//
//	results := qm.DownloadBatch(ctx, fn, items)
//	for i, r := range results {
//	    switch v := r.(type) {
//	    case error:
//	        // handle failure for items[i]
//	    default:
//	        // v is the success value
//	    }
//	}
func (q *QueueManager) DownloadBatch(ctx context.Context, downloadFunc DownloadFunc, items []interface{}) []interface{} {
	results := make([]interface{}, len(items))
	if len(items) == 0 {
		return results
	}

	var wg sync.WaitGroup
	wg.Add(len(items))

	for i, item := range items {
		go func(idx int, it interface{}) {
			defer wg.Done()
			if err := q.acquire(ctx); err != nil {
				results[idx] = err
				return
			}
			defer q.release()

			val, err := downloadFunc(ctx, it)
			if err != nil {
				fmt.Printf("[QueueManager] download failed for item at index %d: %v\n", idx, err)
				results[idx] = err
				return
			}
			results[idx] = val
		}(i, item)
	}

	wg.Wait()
	return results
}

// ProcessTasks runs each task (a closure taking only context) with at most
// maxWorkers concurrent invocations, mirroring process_tasks in the Python
// original. Failures surface as errors in the result slice, in input order.
type TaskFunc func(ctx context.Context) (interface{}, error)

func (q *QueueManager) ProcessTasks(ctx context.Context, tasks []TaskFunc) []interface{} {
	results := make([]interface{}, len(tasks))
	if len(tasks) == 0 {
		return results
	}

	var wg sync.WaitGroup
	wg.Add(len(tasks))

	for i, task := range tasks {
		go func(idx int, tf TaskFunc) {
			defer wg.Done()
			if err := q.acquire(ctx); err != nil {
				results[idx] = err
				return
			}
			defer q.release()

			val, err := tf(ctx)
			if err != nil {
				fmt.Printf("[QueueManager] task at index %d failed: %v\n", idx, err)
				results[idx] = err
				return
			}
			results[idx] = val
		}(i, task)
	}

	wg.Wait()
	return results
}
