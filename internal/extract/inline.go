package extract

import (
	"context"
	"fmt"

	"github.com/chromedp/chromedp"
	"github.com/sidisinsane/snap-css/internal/model"
)

// extractStyleBlocks extracts all inline <style> elements from the page
// and returns their text content in document order.
func extractStyleBlocks(ctx context.Context) ([]model.StyleBlock, error) {
	var raw []string
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			Array.from(document.querySelectorAll('style')).map(el => el.textContent)
		`, &raw),
	)
	if err != nil {
		return nil, fmt.Errorf("evaluate style blocks: %w", err)
	}

	blocks := make([]model.StyleBlock, 0, len(raw))
	for i, content := range raw {
		if content != "" {
			blocks = append(blocks, model.StyleBlock{
				Index:   i + 1,
				Content: content,
			})
		}
	}
	return blocks, nil
}
