package model

import "time"

// Options holds the resolved CLI configuration passed to all workers.
type Options struct {
	URLs           []string
	OutputDir      string
	Concurrency    int
	Timeout        int // seconds, per URL
	MaxImportDepth int
}

// StyleBlock represents an inline <style> element.
type StyleBlock struct {
	Index   int
	Content string
}

// CSSRule represents a single parsed CSS style rule from the browser CSSOM.
type CSSRule struct {
	Selector string            // e.g. ":root", ".btn", "h1"
	Props    map[string]string // property → value, verbatim from cssText
}

// CSSBlock represents either a top-level group of rules or a conditional
// group rule (@media, @supports, @layer, @container) containing rules
// and optionally further nested blocks.
type CSSBlock struct {
	Condition string     // "@media (min-width: 40em)" or "" for top-level
	Rules     []CSSRule  // style rules directly in this block
	Blocks    []CSSBlock // nested conditional blocks
}

// CSSConditionBlock groups token-only rules under a specific condition.
// Used exclusively for TokenDiffs — the getComputedStyle diff results.
type CSSConditionBlock struct {
	Condition string    // e.g. "@media (prefers-color-scheme: dark)"
	Rules     []CSSRule // token rules that changed under this condition vs baseline
}

// CSSFile represents a single captured external stylesheet.
type CSSFile struct {
	URL             string
	Content         string
	ResolvedImports []*CSSFile // same-origin @imports, recursively resolved
	ExternalImports []string   // external @import URLs, not fetched
}

// CSSResult is the full extraction result for a single target URL.
type CSSResult struct {
	URL         string
	CapturedAt  time.Time
	Stylesheets []*CSSFile    // external stylesheets (raw, for URL rewriting)
	StyleBlocks []StyleBlock  // inline <style> blocks
	Blocks      []CSSBlock    // full authored CSS block tree from browser traversal
	TokenDiffs  []CSSConditionBlock // token-only diffs from getComputedStyle passes
}

// ResolvedPath records a relative url() reference that was rewritten to absolute.
type ResolvedPath struct {
	Original string `json:"original"`
	Resolved string `json:"resolved"`
	FoundIn  string `json:"foundIn"`
}

// ReportStylesheet is the stylesheet source tree entry in the report.
type ReportStylesheet struct {
	URL             string   `json:"url"`
	ResolvedImports []string `json:"resolvedImports,omitempty"`
	ExternalImports []string `json:"externalImports,omitempty"`
}

// ReportEmulation describes the emulation that was applied.
type ReportEmulation struct {
	Baseline   map[string]string `json:"baseline"`
	Conditions []string          `json:"conditions"`
}

// ReportStats holds summary counts for the report.
type ReportStats struct {
	StylesheetsCaptured int `json:"stylesheetsCaptured"`
	RulesTotal          int `json:"rulesTotal"`
	TokensFound         int `json:"tokensFound"`
	ConditionsDiffed    int `json:"conditionsDiffed"`
	PathsResolved       int `json:"pathsResolved"`
}

// ReportOutput lists the files produced for this URL.
type ReportOutput struct {
	Styles string `json:"styles"`
	Tokens string `json:"tokens,omitempty"`
}

// Report is the always-written JSON report for a single URL extraction.
type Report struct {
	URL           string             `json:"url"`
	CapturedAt    time.Time          `json:"capturedAt"`
	Output        ReportOutput       `json:"output"`
	Emulation     ReportEmulation    `json:"emulation"`
	Stylesheets   []ReportStylesheet `json:"stylesheets"`
	ResolvedPaths []ResolvedPath     `json:"resolvedPaths,omitempty"`
	Warnings      []string           `json:"warnings,omitempty"`
	Stats         ReportStats        `json:"stats"`
}

// Result wraps a CSSResult and Report with top-level success/failure metadata.
type Result struct {
	URL     string
	Success bool
	Error   string
	Data    *CSSResult
	Report  *Report
}
