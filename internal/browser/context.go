package browser

import (
	"context"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

// DefaultViewportWidth is a representative desktop viewport width used to
// ensure viewport-dependent media queries (e.g. min-width) evaluate
// consistently regardless of the host machine's display configuration.
const DefaultViewportWidth = 1280

// DefaultViewportHeight is the corresponding viewport height.
const DefaultViewportHeight = 800

// NewContext creates a new chromedp browser context with a per-URL timeout
// and an explicit viewport size for consistent media query evaluation.
// The caller is responsible for calling the returned cancel function.
func NewContext(parent context.Context, timeoutSeconds int) (context.Context, context.CancelFunc) {
	allocCtx, allocCancel := chromedp.NewExecAllocator(parent, append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.WindowSize(DefaultViewportWidth, DefaultViewportHeight),
	)...)

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)

	timeout := time.Duration(timeoutSeconds) * time.Second
	ctx, timeoutCancel := context.WithTimeout(browserCtx, timeout)

	// Set the device metrics explicitly after context creation to ensure
	// the viewport is applied before any navigation occurs.
	_ = chromedp.Run(browserCtx,
		emulation.SetDeviceMetricsOverride(DefaultViewportWidth, DefaultViewportHeight, 1, false),
	)

	cancel := func() {
		timeoutCancel()
		browserCancel()
		allocCancel()
	}

	return ctx, cancel
}
