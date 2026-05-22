package extract

import (
	"fmt"
	"net/url"
	"regexp"

	"github.com/sidisinsane/snap-css/internal/model"
)

// urlRefPattern matches url() references in CSS, capturing the inner value.
var urlRefPattern = regexp.MustCompile(`url\(\s*["']?([^"')]+)["']?\s*\)`)

// RewriteURLs walks all CSSFiles in the result tree, rewriting relative url()
// references to absolute URLs using each file's own URL as the base.
// All rewrites are recorded and returned for inclusion in the report.
func RewriteURLs(result *model.CSSResult) []model.ResolvedPath {
	var resolved []model.ResolvedPath

	var rewriteFile func(f *model.CSSFile)
	rewriteFile = func(f *model.CSSFile) {
		rewrites, content := rewriteContent(f.Content, f.URL)
		if len(rewrites) > 0 {
			f.Content = content
			resolved = append(resolved, rewrites...)
		}
		for _, imp := range f.ResolvedImports {
			rewriteFile(imp)
		}
	}

	for _, f := range result.Stylesheets {
		rewriteFile(f)
	}
	for i, block := range result.StyleBlocks {
		rewrites, content := rewriteContent(block.Content, result.URL)
		if len(rewrites) > 0 {
			result.StyleBlocks[i].Content = content
			resolved = append(resolved, rewrites...)
		}
	}

	return resolved
}

// rewriteContent rewrites all relative url() references in a CSS string
// to absolute URLs, using fileURL as the base for resolution.
// Returns the list of rewrites made and the updated CSS string.
func rewriteContent(css, fileURL string) ([]model.ResolvedPath, string) {
	base, err := url.Parse(fileURL)
	if err != nil {
		return nil, css
	}

	var rewrites []model.ResolvedPath

	result := urlRefPattern.ReplaceAllStringFunc(css, func(match string) string {
		sub := urlRefPattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		ref := sub[1]

		// Skip already-absolute URLs and data URIs.
		if isAbsoluteOrData(ref) {
			return match
		}

		imp, err := url.Parse(ref)
		if err != nil {
			return match
		}

		abs := base.ResolveReference(imp).String()
		rewrites = append(rewrites, model.ResolvedPath{
			Original: ref,
			Resolved: abs,
			FoundIn:  fileURL,
		})

		// Preserve quote style from original match.
		return fmt.Sprintf("url(\"%s\")", abs)
	})

	return rewrites, result
}

// isAbsoluteOrData returns true for URLs that don't need rewriting.
func isAbsoluteOrData(ref string) bool {
	return len(ref) > 4 && (ref[:4] == "http" ||
		ref[:4] == "data" ||
		ref[:2] == "//" ||
		ref[:1] == "/")
}
