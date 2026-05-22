package cmd

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sidisinsane/snap-css/internal/browser"
	"github.com/sidisinsane/snap-css/internal/model"
	"github.com/sidisinsane/snap-css/internal/output"
)

// Execute parses flags and runs the extractor.
func Execute() {
	var (
		urlFlag    = flag.String("url", "", "Single target URL")
		urlsFile   = flag.String("urls-file", "", "Path to newline-delimited file of URLs")
		outputDir  = flag.String("output-dir", "./output", "Root output directory")
		concurrency = flag.Int("concurrency", 3, "Max parallel browser contexts")
		timeout    = flag.Int("timeout", 30, "Per-URL timeout in seconds")
		maxDepth   = flag.Int("max-import-depth", 5, "Max @import recursion depth")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "snap-css — capture complete CSS from web pages\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n  snap-css [flags]\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nAlways writes styles.css, tokens.css, and report.json per URL.\n")
	}

	flag.Parse()

	var urls []string
	if *urlFlag != "" {
		urls = append(urls, *urlFlag)
	}
	if *urlsFile != "" {
		batch, err := readURLsFile(*urlsFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: reading --urls-file: %v\n", err)
			os.Exit(1)
		}
		urls = append(urls, batch...)
	}
	if len(urls) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: provide at least one URL via --url or --urls-file\n\n")
		flag.Usage()
		os.Exit(1)
	}

	opts := model.Options{
		URLs:           urls,
		OutputDir:      *outputDir,
		Concurrency:    *concurrency,
		Timeout:        *timeout,
		MaxImportDepth: *maxDepth,
	}

	results := browser.RunPool(opts)

	success, failed := 0, 0
	for _, r := range results {
		if !r.Success {
			failed++
			continue
		}
		success++
		if err := output.WriteCSS(r.Data, r.Report, opts.OutputDir); err != nil {
			fmt.Printf("ERROR: write CSS for %s: %v\n", r.URL, err)
		}
		if err := output.WriteReport(r.Report, opts.OutputDir); err != nil {
			fmt.Printf("ERROR: write report for %s: %v\n", r.URL, err)
		}
	}

	fmt.Printf("\n%d/%d URLs processed successfully\n", success, len(urls))
	if failed > 0 {
		fmt.Printf("%d failed — see FAILED lines above\n", failed)
	}
}

// readURLsFile reads a newline-delimited list of URLs, skipping blank lines and comments.
func readURLsFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var urls []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		urls = append(urls, line)
	}
	return urls, scanner.Err()
}
