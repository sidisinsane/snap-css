package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sidisinsane/snap-css/internal/model"
)

// WriteReport always writes a report.json alongside the CSS output file.
// The report contains the emulation plan, stylesheet source tree,
// resolved paths, warnings, and extraction stats.
func WriteReport(report *model.Report, outputDir string) error {
	dir, err := URLToPath(outputDir, report.URL)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output dir %s: %w", dir, err)
	}

	path := filepath.Join(dir, "report.json")

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report for %s: %w", report.URL, err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	fmt.Printf("wrote %s\n", path)
	return nil
}
