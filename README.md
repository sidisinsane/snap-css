# snap-css

A CLI-tool that takes a snapshot of the CSS of any web page and saves it as
clean, ready-to-use files. Point it at a URL and it gives you back the complete
styles the browser is actually using — no manual digging through DevTools, no
hunting across multiple stylesheets.

If you work with design systems, audit third-party sites, or want to understand
how a page is styled without reading through its source, `snap-css` is for you.

---

## Installation

> Requires Chrome, Chromium, or a Chromium-based browser (Edge, Brave, etc.).
> Chrome or Chromium are recommended for reliability.

**Shell script (macOS/Linux):**

```bash
curl -o- https://raw.githubusercontent.com/sidisinsane/snap-css/main/install.sh | bash
```

**PowerShell (Windows):**

```powershell
irm https://raw.githubusercontent.com/sidisinsane/snap-css/main/install.ps1 | iex
```

**Archive download (macOS/Linux):**

```bash
# Download the release asset and its corresponding SHA256 checksum file
curl -OL https://github.com/sidisinsane/snap-css/releases/download/v0.1.0/snap-css_Darwin_arm64.tar.gz
curl -OL https://github.com/sidisinsane/snap-css/releases/download/v0.1.0/checksums.txt

# Verify the checksum
shasum -a 256 --check --ignore-missing checksums.txt

# Extract the archive
tar -xzf snap-css_Darwin_arm64.tar.gz

# Move the binary to a directory on your PATH
mv snap-css /usr/local/bin/
```

**Build from source:**

```bash
# Clone the repository
git clone https://github.com/sidisinsane/snap-css
cd snap-css

# Build the binary
go build -o snap-css .

# Make it executable and move it to a directory on your PATH
chmod +x snap-css
mv snap-css /usr/local/bin/
```

**Verify the installation:**

```bash
snap-css --help
```

---

## Getting Started

### Usage

```bash
# Single URL
snap-css --url https://example.com

# Multiple URLs from a file — one per line, # for comments
snap-css --urls-file urls.txt

# Custom output location
snap-css --url https://example.com --output-dir ./snapped
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `--url` | — | Single target URL |
| `--urls-file` | — | Path to newline-delimited file of URLs |
| `--output-dir` | `./output` | Root output directory |
| `--concurrency` | `3` | Max parallel browser contexts |
| `--timeout` | `30` | Per-URL timeout in seconds |
| `--max-import-depth` | `5` | Max `@import` recursion depth |

### What you get

For each URL, three files are written:

- **`styles.css`** — the complete stylesheet, exactly as the browser has it, in
  one place
- **`tokens.css`** — the design tokens (CSS custom properties), including any
  that change in dark mode or other conditions
- **`report.json`** — a summary of what was found and what was written

```text
output/
└── example.com/
    ├── styles.css
    ├── tokens.css    # omitted if no custom properties found
    └── report.json
```

If the URL has a path, the folder structure mirrors it:

```text
output/
└── example.com/
    └── shop/
        ├── styles.css
        ├── tokens.css
        └── report.json
```

## How it works

`snap-css` uses a headless browser to load each URL, then reads the CSS directly
from the browser rather than parsing source files. This means import chains,
minification, and dynamically injected styles are all handled transparently —
you get what the browser has, not what the files say.

### styles.css

A single flat file containing all the rules the browser loaded, with all
`@import` chains resolved inline. All conditional blocks — `@media`,
`@supports`, `@layer`, `@container` — are preserved with their wrappers.
Declarations are captured verbatim, so `padding-inline: var(--gutter)` stays as
authored rather than being expanded into longhands.

Stylesheets that don't apply to the current environment (e.g. `media="print"`)
are excluded. If design tokens were found, `styles.css` imports `tokens.css` at
the top.

### tokens.css

Custom properties only, extracted from the full stylesheet. `snap-css` runs the
page under each standard media condition and compares the resolved token values
against the baseline — so dark mode overrides, contrast preferences, and reduced
motion variants are all captured correctly, regardless of how they're authored.

Baseline conditions are always deterministic: `prefers-color-scheme: light`,
`prefers-contrast: no-preference`, `prefers-reduced-motion: no-preference`,
`forced-colors: none`.

```css
:root {
  --color-bg: #ece5d3;
  --color-fg: #14110d;
}

@media (prefers-color-scheme: dark) {
  :root {
    --color-bg: #14110d;
    --color-fg: #ece5d3;
  }
}
```

If no custom properties are found, `tokens.css` is not written.

### report.json

A machine-readable summary covering which files were produced, which emulation
conditions produced diffs, the source stylesheet tree, any relative URLs that
were rewritten to absolute, and basic stats.

```json
{
  "url": "https://example.com",
  "output": {
    "styles": "styles.css",
    "tokens": "tokens.css"
  },
  "emulation": {
    "baseline": { "prefers-color-scheme": "light", ... },
    "conditions": ["@media (prefers-color-scheme: dark)"]
  },
  "stats": {
    "stylesheetsCaptured": 3,
    "rulesTotal": 142,
    "tokensFound": 42,
    "conditionsDiffed": 1,
    "pathsResolved": 0
  }
}
```

## Notes

Relative `url()` references — background images, local fonts — are rewritten to
absolute URLs so the output files are portable. Every rewrite is listed in `report.json`.

Values in `styles.css` are authored values, not computed ones. The browser's
cascade is preserved through document order and explicit `@media` blocks rather
than by resolving it into a single canonical ruleset.
