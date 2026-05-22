package output

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// URLToPath derives a human-readable filesystem path from a URL.
//
// Rules:
//   - Query strings and fragments are stripped (with a warning).
//   - The host becomes the top-level directory.
//   - Path segments become subdirectories.
//   - Trailing slashes are normalised away.
//
// Examples:
//
//	https://example.com              → <base>/example.com
//	https://example.com/shop         → <base>/example.com/shop
//	https://example.com/shop/cart/   → <base>/example.com/shop/cart
func URLToPath(baseDir, rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}

	if u.RawQuery != "" {
		fmt.Printf("WARN: query string dropped from URL for path derivation: %s\n", rawURL)
		u.RawQuery = ""
	}
	if u.Fragment != "" {
		u.Fragment = ""
	}

	// Split path into clean segments, stripping empty parts from leading/trailing slashes.
	rawSegments := strings.Split(strings.Trim(u.Path, "/"), "/")
	var segments []string
	for _, s := range rawSegments {
		if s != "" {
			segments = append(segments, s)
		}
	}

	parts := append([]string{baseDir, u.Host}, segments...)
	return filepath.Join(parts...), nil
}
