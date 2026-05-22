package extract

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/sidisinsane/snap-css/internal/model"
)

// Run is the main entry point for extraction. The flow is:
//  1. Navigate and capture raw CSS via network listener
//  2. Resolve imports
//  3. Extract inline <style> blocks
//  4. Rewrite relative url() references to absolute
//  5. Run emulation passes — block tree capture + token diffs
//  6. Build report
func Run(ctx context.Context, targetURL string, opts model.Options) (*model.CSSResult, *model.Report, error) {
	result := &model.CSSResult{
		URL:        targetURL,
		CapturedAt: time.Now(),
	}
	report := &model.Report{
		URL:        targetURL,
		CapturedAt: result.CapturedAt,
	}

	// Step 1: navigate and capture raw CSS.
	cssFiles := make(map[string]*model.CSSFile)
	var cssOrder []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	err := chromedp.Run(ctx,
		network.Enable(),
		chromedp.ActionFunc(func(ctx context.Context) error {
			chromedp.ListenTarget(ctx, networkListener(ctx, cssFiles, &cssOrder, &mu, &wg))
			return nil
		}),
		chromedp.Navigate(targetURL),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return waitForIdle(ctx)
		}),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("navigation failed: %w", err)
	}
	wg.Wait()

	for _, url := range cssOrder {
		if f := cssFiles[url]; f != nil {
			result.Stylesheets = append(result.Stylesheets, f)
		}
	}

	// Step 2: resolve imports.
	for _, f := range result.Stylesheets {
		if err := resolveImports(ctx, f, targetURL, cssFiles, opts.MaxImportDepth, 0); err != nil {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("import resolution error for %s: %v", f.URL, err))
		}
	}

	// Step 3: extract inline <style> blocks.
	blocks, err := extractStyleBlocks(ctx)
	if err != nil {
		report.Warnings = append(report.Warnings,
			fmt.Sprintf("could not extract inline style blocks: %v", err))
	} else {
		result.StyleBlocks = blocks
	}

	// Step 4: rewrite relative url() references to absolute.
	resolvedPaths := RewriteURLs(result)
	report.ResolvedPaths = resolvedPaths

	// Step 5: run emulation passes.
	fmt.Printf("INFO: running %d emulation pass(es) for %s\n", len(canonicalConditions)+1, targetURL)
	cssBlocks, tokenDiffs, diffedConditions, err := RunEmulationPasses(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("emulation passes failed: %w", err)
	}

	result.Blocks = cssBlocks
	result.TokenDiffs = tokenDiffs

	if len(diffedConditions) > 0 {
		fmt.Printf("INFO: %d condition(s) produced token diffs:\n", len(diffedConditions))
		for _, c := range diffedConditions {
			fmt.Printf("      @media (%s: %s)\n", c.Name, c.Value)
		}
	} else {
		fmt.Println("INFO: no emulation conditions produced token diffs")
	}

	// Step 6: populate report.
	report.Emulation = buildReportEmulation(diffedConditions)
	report.Stylesheets = buildReportStylesheets(result.Stylesheets)
	report.Stats = model.ReportStats{
		StylesheetsCaptured: len(result.Stylesheets),
		RulesTotal:          countRulesInBlocks(cssBlocks),
		TokensFound:         CountTokensInBlocks(cssBlocks) + countTokensInDiffs(tokenDiffs),
		ConditionsDiffed:    len(tokenDiffs),
		PathsResolved:       len(resolvedPaths),
	}

	return result, report, nil
}

// waitForIdle waits briefly for the page to settle after navigation.
func waitForIdle(ctx context.Context) error {
	return chromedp.Sleep(2 * time.Second).Do(ctx)
}

// countRulesInBlocks counts total style rules across a CSSBlock tree.
func countRulesInBlocks(blocks []model.CSSBlock) int {
	count := 0
	var walk func(b model.CSSBlock)
	walk = func(b model.CSSBlock) {
		count += len(b.Rules)
		for _, child := range b.Blocks {
			walk(child)
		}
	}
	for _, b := range blocks {
		walk(b)
	}
	return count
}

// countTokensInDiffs counts custom properties across token diff blocks.
func countTokensInDiffs(diffs []model.CSSConditionBlock) int {
	count := 0
	for _, diff := range diffs {
		for _, rule := range diff.Rules {
			count += len(rule.Props)
		}
	}
	return count
}

// buildReportEmulation builds the report emulation section from diffed conditions.
func buildReportEmulation(diffed []EmulationFeature) model.ReportEmulation {
	baseline := make(map[string]string, len(canonicalBaseline))
	for _, f := range canonicalBaseline {
		baseline[f.Name] = f.Value
	}
	conditions := make([]string, 0, len(diffed))
	for _, c := range diffed {
		conditions = append(conditions, fmt.Sprintf("@media (%s: %s)", c.Name, c.Value))
	}
	return model.ReportEmulation{
		Baseline:   baseline,
		Conditions: conditions,
	}
}

// buildReportStylesheets converts the captured stylesheet tree into report entries.
func buildReportStylesheets(files []*model.CSSFile) []model.ReportStylesheet {
	result := make([]model.ReportStylesheet, 0, len(files))
	for _, f := range files {
		rs := model.ReportStylesheet{
			URL:             f.URL,
			ExternalImports: f.ExternalImports,
		}
		for _, imp := range f.ResolvedImports {
			rs.ResolvedImports = append(rs.ResolvedImports, imp.URL)
		}
		result = append(result, rs)
	}
	return result
}
