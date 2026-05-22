package extract

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/sidisinsane/snap-css/internal/model"
)

// importPattern matches both @import "url" and @import url(...) forms.
var importPattern = regexp.MustCompile(`@import\s+(?:url\()?["']?([^"');]+)["']?\)?`)

// resolveImports scans a CSSFile's content for @import rules, classifies each
// as same-origin or external, and recursively resolves same-origin imports.
// cssFiles is passed so already-captured stylesheets are reused rather than
// re-fetched, preventing duplication when a file is both linked and imported.
func resolveImports(
	ctx context.Context,
	file *model.CSSFile,
	baseURL string,
	cssFiles map[string]*model.CSSFile,
	maxDepth, currentDepth int,
) error {
	if currentDepth >= maxDepth {
		fmt.Printf("WARN: max import depth (%d) reached for %s\n", maxDepth, file.URL)
		return nil
	}

	matches := importPattern.FindAllStringSubmatch(file.Content, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		importURL := strings.TrimSpace(m[1])

		resolved, err := resolveURL(file.URL, importURL)
		if err != nil {
			fmt.Printf("WARN: could not parse import URL %q in %s: %v\n", importURL, file.URL, err)
			continue
		}

		if isExternalImport(baseURL, resolved) {
			// Preserve verbatim; do not fetch.
			file.ExternalImports = append(file.ExternalImports, resolved)
			continue
		}

		// Same-origin: reuse if already captured by the network listener.
		if existing, ok := cssFiles[resolved]; ok && existing != nil {
			file.ResolvedImports = append(file.ResolvedImports, existing)
			continue
		}

		// Not yet captured — fetch explicitly.
		content, err := fetchURL(resolved)
		if err != nil {
			fmt.Printf("WARN: could not fetch same-origin import %s: %v\n", resolved, err)
			continue
		}

		child := &model.CSSFile{
			URL:     resolved,
			Content: content,
		}
		// Register in cssFiles so deeper recursion doesn't re-fetch it.
		cssFiles[resolved] = child
		file.ResolvedImports = append(file.ResolvedImports, child)

		if err := resolveImports(ctx, child, baseURL, cssFiles, maxDepth, currentDepth+1); err != nil {
			return err
		}
	}
	return nil
}

// isExternalImport returns true when the import URL belongs to a different host
// than the base URL. Relative imports (no host) are always same-origin.
// On parse failure we fail safe and treat the import as external.
func isExternalImport(baseURL, importURL string) bool {
	base, err1 := url.Parse(baseURL)
	imp, err2 := url.Parse(importURL)
	if err1 != nil || err2 != nil {
		return true // fail safe
	}
	// No host means relative — always same-origin.
	if imp.Host == "" {
		return false
	}
	return imp.Host != base.Host
}

// resolveURL resolves an import URL relative to the file that contains it.
func resolveURL(fileURL, importURL string) (string, error) {
	base, err := url.Parse(fileURL)
	if err != nil {
		return "", err
	}
	imp, err := url.Parse(importURL)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(imp).String(), nil
}

// fetchURL performs a simple HTTP GET for a same-origin CSS file.
func fetchURL(rawURL string) (string, error) {
	resp, err := http.Get(rawURL) //nolint:gosec // URL is already validated as same-origin
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
