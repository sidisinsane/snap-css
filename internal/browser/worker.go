package browser

import (
	"context"
	"fmt"
	"sync"

	"github.com/sidisinsane/snap-css/internal/extract"
	"github.com/sidisinsane/snap-css/internal/model"
)

// RunPool processes all URLs from opts using a fixed-size worker pool.
// Each worker owns its own browser context for its lifetime.
func RunPool(opts model.Options) []model.Result {
	queue := make(chan string, len(opts.URLs))
	for _, u := range opts.URLs {
		queue <- u
	}
	close(queue)

	results := make(chan model.Result, len(opts.URLs))

	var wg sync.WaitGroup
	for i := 0; i < opts.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runWorker(queue, results, opts)
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var out []model.Result
	for r := range results {
		out = append(out, r)
	}
	return out
}

// runWorker pulls URLs from the queue and processes them sequentially
// using a single long-lived browser context.
func runWorker(queue <-chan string, results chan<- model.Result, opts model.Options) {
	workerCtx, workerCancel := NewContext(context.Background(), opts.Timeout*len(opts.URLs))
	defer workerCancel()

	for url := range queue {
		results <- processURL(workerCtx, url, opts)
	}
}

// processURL runs extraction for a single URL within a worker's browser context.
func processURL(workerCtx context.Context, url string, opts model.Options) model.Result {
	ctx, cancel := NewContext(workerCtx, opts.Timeout)
	defer cancel()

	data, report, err := extract.Run(ctx, url, opts)
	if err != nil {
		fmt.Printf("FAILED: %s — %v\n", url, err)
		return model.Result{
			URL:     url,
			Success: false,
			Error:   err.Error(),
		}
	}

	return model.Result{
		URL:     url,
		Success: true,
		Data:    data,
		Report:  report,
	}
}
