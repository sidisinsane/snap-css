package extract

import (
	"context"
	"fmt"
	"sync"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/sidisinsane/snap-css/internal/model"
)

// networkListener returns a chromedp event listener that captures CSS responses.
// It spawns a goroutine per CSS response to fetch the body without blocking the
// event loop. A WaitGroup is used to track in-flight fetches.
// cssOrder records insertion order so stylesheets can be emitted in document order.
func networkListener(
	ctx context.Context,
	cssFiles map[string]*model.CSSFile,
	cssOrder *[]string,
	mu *sync.Mutex,
	wg *sync.WaitGroup,
) func(interface{}) {
	return func(ev interface{}) {
		resp, ok := ev.(*network.EventResponseReceived)
		if !ok {
			return
		}
		if resp.Response.MimeType != "text/css" {
			return
		}

		reqID := resp.RequestID
		url := resp.Response.URL

		// Reserve the slot in document order before spawning the goroutine,
		// so order reflects network response sequence rather than fetch completion.
		mu.Lock()
		if _, exists := cssFiles[url]; !exists {
			cssFiles[url] = nil // placeholder to claim the slot
			*cssOrder = append(*cssOrder, url)
		}
		mu.Unlock()

		wg.Add(1)
		go func() {
			defer wg.Done()
			fetchCSSBody(ctx, reqID, url, cssFiles, mu)
		}()
	}
}

// fetchCSSBody retrieves the response body for a CSS request and stores it.
// It uses a sibling tab context derived from the parent browser context to avoid
// racing against the parent action chain.
func fetchCSSBody(
	ctx context.Context,
	reqID network.RequestID,
	url string,
	cssFiles map[string]*model.CSSFile,
	mu *sync.Mutex,
) {
	tCtx, cancel := chromedp.NewContext(ctx)
	defer cancel()

	body, err := network.GetResponseBody(reqID).Do(tCtx)
	if err != nil {
		fmt.Printf("WARN: could not fetch body for %s: %v\n", url, err)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	// Only write if we hold the placeholder (nil); skip if already populated.
	if f, exists := cssFiles[url]; exists && f == nil {
		cssFiles[url] = &model.CSSFile{
			URL:     url,
			Content: string(body),
		}
	}
}
