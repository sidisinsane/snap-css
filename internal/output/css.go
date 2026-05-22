package output

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sidisinsane/snap-css/internal/extract"
	"github.com/sidisinsane/snap-css/internal/model"
)

// WriteCSS always writes styles.css and conditionally writes tokens.css.
// tokens.css is omitted when no custom properties are found.
// The @import in styles.css is only emitted when tokens.css exists.
// The report is updated to reflect which files were produced.
func WriteCSS(result *model.CSSResult, report *model.Report, outputDir string) error {
	dir, err := URLToPath(outputDir, result.URL)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output dir %s: %w", dir, err)
	}

	tokenBlocks, tokenDiffs := extract.ExtractTokens(result.Blocks, result.TokenDiffs)
	hasTokens := len(tokenBlocks) > 0 || len(tokenDiffs) > 0

	if err := writeStylesFile(result, dir, hasTokens); err != nil {
		return err
	}
	report.Output.Styles = "styles.css"

	if hasTokens {
		if err := writeTokensFile(result, dir, tokenBlocks, tokenDiffs); err != nil {
			return err
		}
		report.Output.Tokens = "tokens.css"
	} else {
		fmt.Printf("INFO: no tokens found for %s — tokens.css not written\n", result.URL)
	}
	return nil
}

// writeStylesFile writes styles.css — complete flattened stylesheet,
// custom properties stripped. @import "tokens.css" prepended only if tokens exist.
func writeStylesFile(result *model.CSSResult, dir string, hasTokens bool) error {
	path := filepath.Join(dir, "styles.css")
	content := buildStylesCSS(result, hasTokens)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Printf("wrote %s\n", path)
	return nil
}

// writeTokensFile writes tokens.css — custom properties only with condition wrappers,
// followed by deduplicated token diffs from emulation passes.
func writeTokensFile(result *model.CSSResult, dir string, tokenBlocks []model.CSSBlock, tokenDiffs []model.CSSConditionBlock) error {
	path := filepath.Join(dir, "tokens.css")
	content := buildTokensCSS(result, tokenBlocks, tokenDiffs)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Printf("wrote %s\n", path)
	return nil
}

// buildStylesCSS composes the complete flattened stylesheet.
func buildStylesCSS(result *model.CSSResult, hasTokens bool) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(
		"/* snap-css | %s | styles | captured: %s */\n\n",
		result.URL, result.CapturedAt.Format("2006-01-02T15:04:05Z"),
	))
	if hasTokens {
		b.WriteString("@import \"tokens.css\";\n\n")
	}
	writeBlocks(&b, stripTokenBlocks(result.Blocks), 0)
	writeStyleBlocks(&b, result.StyleBlocks)
	return b.String()
}

// buildTokensCSS composes the tokens-only stylesheet.
func buildTokensCSS(result *model.CSSResult, tokenBlocks []model.CSSBlock, tokenDiffs []model.CSSConditionBlock) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(
		"/* snap-css | %s | tokens | captured: %s */\n\n",
		result.URL, result.CapturedAt.Format("2006-01-02T15:04:05Z"),
	))
	writeBlocks(&b, tokenBlocks, 0)
	writeTokenDiffs(&b, tokenDiffs)
	return b.String()
}

// writeBlocks recursively emits a CSSBlock tree at the given indent level.
func writeBlocks(b *strings.Builder, blocks []model.CSSBlock, depth int) {
	indent := strings.Repeat("  ", depth)
	for _, block := range blocks {
		if block.Condition != "" {
			fmt.Fprintf(b, "%s%s {\n", indent, block.Condition)
			// Write inner content into a buffer so we can trim trailing whitespace
			// before the closing brace.
			var inner strings.Builder
			writeRules(&inner, block.Rules, depth+1)
			writeBlocks(&inner, block.Blocks, depth+1)
			fmt.Fprintf(b, "%s", strings.TrimRight(inner.String(), "\n"))
			fmt.Fprintf(b, "\n%s}\n\n", indent)
		} else {
			writeRules(b, block.Rules, depth)
			writeBlocks(b, block.Blocks, depth)
		}
	}
}

// writeRules emits a list of CSS rules at the given indent level.
func writeRules(b *strings.Builder, rules []model.CSSRule, depth int) {
	indent := strings.Repeat("  ", depth)
	propIndent := strings.Repeat("  ", depth+1)
	for _, rule := range rules {
		props := nonEmptyProps(rule.Props)
		if len(props) == 0 {
			continue
		}
		fmt.Fprintf(b, "%s%s {\n", indent, rule.Selector)
		for _, prop := range props {
			fmt.Fprintf(b, "%s%s: %s;\n", propIndent, prop, rule.Props[prop])
		}
		fmt.Fprintf(b, "%s}\n\n", indent)
	}
}

// writeTokenDiffs appends token diff blocks from emulation passes,
// preceded by a section comment.
func writeTokenDiffs(b *strings.Builder, diffs []model.CSSConditionBlock) {
	if len(diffs) == 0 {
		return
	}
	b.WriteString("\n/* =============================================================\n")
	b.WriteString("   Non-baseline tokens — emulation condition overrides\n")
	b.WriteString("   ============================================================= */\n")
	for _, diff := range diffs {
		fmt.Fprintf(b, "\n%s {\n", diff.Condition)
		for _, rule := range diff.Rules {
			props := nonEmptyProps(rule.Props)
			if len(props) == 0 {
				continue
			}
			fmt.Fprintf(b, "  %s {\n", rule.Selector)
			for _, prop := range props {
				fmt.Fprintf(b, "    %s: %s;\n", prop, rule.Props[prop])
			}
			b.WriteString("  }\n")
		}
		b.WriteString("}\n")
	}
}

// writeStyleBlocks emits inline <style> block contents with source markers.
func writeStyleBlocks(b *strings.Builder, blocks []model.StyleBlock) {
	for _, block := range blocks {
		fmt.Fprintf(b, "\n/* === inline <style> block #%d === */\n", block.Index)
		b.WriteString(strings.TrimSpace(block.Content))
		b.WriteString("\n")
	}
}

// stripTokenBlocks returns a copy of the block tree with all custom
// properties removed. Blocks that become empty are omitted.
func stripTokenBlocks(blocks []model.CSSBlock) []model.CSSBlock {
	var result []model.CSSBlock
	for _, block := range blocks {
		stripped := stripTokenBlock(block)
		if len(stripped.Rules) > 0 || len(stripped.Blocks) > 0 {
			result = append(result, stripped)
		}
	}
	return result
}

func stripTokenBlock(block model.CSSBlock) model.CSSBlock {
	out := model.CSSBlock{Condition: block.Condition}
	for _, rule := range block.Rules {
		filtered := make(map[string]string)
		for prop, val := range rule.Props {
			if !strings.HasPrefix(prop, "--") {
				filtered[prop] = val
			}
		}
		if len(filtered) > 0 {
			out.Rules = append(out.Rules, model.CSSRule{
				Selector: rule.Selector,
				Props:    filtered,
			})
		}
	}
	for _, child := range block.Blocks {
		sc := stripTokenBlock(child)
		if len(sc.Rules) > 0 || len(sc.Blocks) > 0 {
			out.Blocks = append(out.Blocks, sc)
		}
	}
	return out
}

// nonEmptyProps returns sorted keys for props whose values are non-empty.
func nonEmptyProps(props map[string]string) []string {
	var keys []string
	for k, v := range props {
		if strings.TrimSpace(v) != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}
