package extract

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
	"github.com/sidisinsane/snap-css/internal/model"
)

// EmulationFeature represents a single CSS media feature and the value to emulate.
type EmulationFeature struct {
	Name  string
	Value string
}

// canonicalBaseline defines the deterministic default value for each
// emulatable media feature, reflecting standard authoring conventions.
var canonicalBaseline = []EmulationFeature{
	{Name: "prefers-color-scheme", Value: "light"},
	{Name: "prefers-contrast", Value: "no-preference"},
	{Name: "prefers-reduced-motion", Value: "no-preference"},
	{Name: "forced-colors", Value: "none"},
}

// canonicalConditions lists all non-baseline values to test per feature.
var canonicalConditions = []EmulationFeature{
	{Name: "prefers-color-scheme", Value: "dark"},
	{Name: "prefers-contrast", Value: "more"},
	{Name: "prefers-contrast", Value: "less"},
	{Name: "prefers-reduced-motion", Value: "reduce"},
	{Name: "forced-colors", Value: "active"},
}

// applyEmulation sets the browser's emulated media features via CDP.
// Pass an empty slice to clear all emulation.
func applyEmulation(ctx context.Context, features []EmulationFeature) error {
	cdpFeatures := make([]*emulation.MediaFeature, len(features))
	for i, f := range features {
		cdpFeatures[i] = &emulation.MediaFeature{Name: f.Name, Value: f.Value}
	}
	return chromedp.Run(ctx,
		emulation.SetEmulatedMedia().WithFeatures(cdpFeatures),
	)
}

// rawRuleEntry is used for JSON unmarshalling of a single style rule.
type rawRuleEntry struct {
	Selector string            `json:"selector"`
	Props    map[string]string `json:"props"`
}

// rawBlockEntry is used for JSON unmarshalling of a conditional block.
type rawBlockEntry struct {
	Condition string         `json:"condition"`
	Rules     []rawRuleEntry `json:"rules"`
	Blocks    []rawBlockEntry `json:"blocks"`
}

// fetchBlocksFromBrowser traverses document.styleSheets and returns a
// CSSBlock tree preserving the full authored structure including all
// conditional group rules (@media, @supports, @layer, @container).
// Uses rule.style.cssText to capture verbatim declarations.
// Sheet-level media conditions are filtered via matchMedia().
func fetchBlocksFromBrowser(ctx context.Context) ([]model.CSSBlock, error) {
	var resultJSON string

	err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				const topBlock = { condition: '', rules: [], blocks: [] };

				for (const sheet of document.styleSheets) {
					// Skip sheets whose media condition doesn't match the current
					// environment. Uses matchMedia() — future-proof against any type.
					const media = sheet.media && sheet.media.mediaText;
					if (media && media !== '' && media !== 'all') {
						if (!window.matchMedia(media).matches) continue;
					}
					try {
						collectFromRules(sheet.cssRules, topBlock);
					} catch(e) {
						// Cross-origin sheet — skip gracefully.
					}
				}

				function parseCSSText(cssText) {
					const props = {};
					const declarations = splitDeclarations(cssText);
					for (const decl of declarations) {
						const colon = decl.indexOf(':');
						if (colon === -1) continue;
						const prop = decl.slice(0, colon).trim();
						const val = decl.slice(colon + 1).trim();
						if (prop && val) props[prop] = val;
					}
					return props;
				}

				function splitDeclarations(cssText) {
					const result = [];
					let current = '';
					let depth = 0;
					let inString = false;
					let stringChar = '';

					for (let i = 0; i < cssText.length; i++) {
						const ch = cssText[i];
						if (inString) {
							current += ch;
							if (ch === stringChar && cssText[i - 1] !== '\\') inString = false;
							continue;
						}
						if (ch === '"' || ch === "'") {
							inString = true;
							stringChar = ch;
							current += ch;
							continue;
						}
						if (ch === '(') { depth++; current += ch; continue; }
						if (ch === ')') { depth--; current += ch; continue; }
						if (ch === ';' && depth === 0) {
							if (current.trim()) result.push(current.trim());
							current = '';
							continue;
						}
						current += ch;
					}
					if (current.trim()) result.push(current.trim());
					return result;
				}

				function collectFromRules(cssRules, targetBlock) {
					if (!cssRules) return;
					for (const rule of cssRules) {
						// CSSImportRule (type 3) — recurse into imported sheet.
						if (rule.type === 3 && rule.styleSheet) {
							try { collectFromRules(rule.styleSheet.cssRules, targetBlock); } catch(e) {}
							continue;
						}
						// CSSStyleRule (type 1) — capture verbatim declarations.
						if (rule.type === 1) {
							const props = parseCSSText(rule.style.cssText);
							if (Object.keys(props).length > 0) {
								targetBlock.rules.push({ selector: rule.selectorText, props });
							}
							continue;
						}
						// Conditional group rules — @media (4), @supports (12),
						// @layer (35), @container (?) — recurse into a new block.
						if (rule.cssRules) {
							const conditionText = rule.conditionText || rule.nameText || rule.media && rule.media.mediaText || '';
							const prefix = rule.constructor.name === 'CSSMediaRule' ? '@media' :
								rule.constructor.name === 'CSSSupportsRule' ? '@supports' :
								rule.constructor.name === 'CSSLayerBlockRule' ? '@layer' :
								rule.constructor.name === 'CSSContainerRule' ? '@container' : '@unknown';
							const condition = conditionText ? prefix + ' ' + conditionText : prefix;
							const childBlock = { condition, rules: [], blocks: [] };
							collectFromRules(rule.cssRules, childBlock);
							if (childBlock.rules.length > 0 || childBlock.blocks.length > 0) {
								targetBlock.blocks.push(childBlock);
							}
						}
					}
				}

				return JSON.stringify(topBlock);
			})()
		`, &resultJSON),
	)
	if err != nil {
		return nil, fmt.Errorf("document.styleSheets traversal failed: %w", err)
	}

	var raw rawBlockEntry
	if err := json.Unmarshal([]byte(resultJSON), &raw); err != nil {
		return nil, fmt.Errorf("parse styleSheets result: %w", err)
	}

	// The top-level entry has no condition — flatten its children into a slice.
	var blocks []model.CSSBlock
	top := convertBlock(raw)
	// Top-level rules become a single unnamed block.
	if len(top.Rules) > 0 {
		blocks = append(blocks, model.CSSBlock{Rules: top.Rules})
	}
	blocks = append(blocks, top.Blocks...)
	return blocks, nil
}

// convertBlock recursively converts a rawBlockEntry into a model.CSSBlock.
func convertBlock(raw rawBlockEntry) model.CSSBlock {
	block := model.CSSBlock{
		Condition: raw.Condition,
	}
	for _, r := range raw.Rules {
		block.Rules = append(block.Rules, model.CSSRule{
			Selector: r.Selector,
			Props:    r.Props,
		})
	}
	for _, b := range raw.Blocks {
		block.Blocks = append(block.Blocks, convertBlock(b))
	}
	return block
}

// fetchComputedTokens uses getComputedStyle to get resolved custom property
// values on the root element. IS affected by CDP emulation state.
// Step 1: collect all custom property names from cssRules (static).
// Step 2: resolve each via getComputedStyle (emulation-aware).
func fetchComputedTokens(ctx context.Context) (map[string]string, error) {
	var resultJSON string

	err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				const propNames = new Set();
				function collectProps(cssRules) {
					if (!cssRules) return;
					for (const rule of cssRules) {
						if (rule.type === 3 && rule.styleSheet) {
							try { collectProps(rule.styleSheet.cssRules); } catch(e) {}
							continue;
						}
						if (rule.type === 1) {
							for (const prop of rule.style) {
								if (prop.startsWith('--')) propNames.add(prop.trim());
							}
						}
						if (rule.cssRules) collectProps(rule.cssRules);
					}
				}
				for (const sheet of document.styleSheets) {
					try { collectProps(sheet.cssRules); } catch(e) {}
				}

				const computed = getComputedStyle(document.documentElement);
				const tokens = {};
				for (const prop of propNames) {
					const val = computed.getPropertyValue(prop).trim();
					if (val !== '') tokens[prop] = val;
				}

				return JSON.stringify(tokens);
			})()
		`, &resultJSON),
	)
	if err != nil {
		return nil, fmt.Errorf("fetchComputedTokens failed: %w", err)
	}

	var tokens map[string]string
	if err := json.Unmarshal([]byte(resultJSON), &tokens); err != nil {
		return nil, fmt.Errorf("parse computed tokens: %w", err)
	}
	return tokens, nil
}

// RunEmulationPasses runs the full canonical emulation pass set:
//  1. Apply canonical baseline emulation
//  2. Capture full CSSBlock tree via fetchBlocksFromBrowser (once, static)
//  3. Capture baseline tokens via getComputedStyle
//  4. For each canonical condition, diff computed tokens against baseline
//  5. Return block tree + non-empty token diffs
func RunEmulationPasses(ctx context.Context) ([]model.CSSBlock, []model.CSSConditionBlock, []EmulationFeature, error) {
	// Apply canonical baseline.
	if err := applyEmulation(ctx, canonicalBaseline); err != nil {
		return nil, nil, nil, fmt.Errorf("apply baseline emulation: %w", err)
	}

	// Single block tree capture — cssRules are static, only need one pass.
	blocks, err := fetchBlocksFromBrowser(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("block tree capture failed: %w", err)
	}

	// Baseline token capture via getComputedStyle — emulation-aware.
	baselineTokens, err := fetchComputedTokens(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("baseline token pass: %w", err)
	}

	// Condition passes — token diffs only.
	var tokenDiffs []model.CSSConditionBlock
	var diffedConditions []EmulationFeature

	for _, condition := range canonicalConditions {
		condFeatures := append(append([]EmulationFeature{}, canonicalBaseline...), condition)
		if err := applyEmulation(ctx, condFeatures); err != nil {
			fmt.Printf("WARN: could not emulate %s:%s — %v\n", condition.Name, condition.Value, err)
			continue
		}

		passTokens, err := fetchComputedTokens(ctx)
		if err != nil {
			fmt.Printf("WARN: token pass failed for %s:%s — %v\n", condition.Name, condition.Value, err)
			continue
		}

		changedProps := make(map[string]string)
		for prop, val := range passTokens {
			if baselineTokens[prop] != val {
				changedProps[prop] = val
			}
		}
		for prop, val := range passTokens {
			if _, exists := baselineTokens[prop]; !exists {
				changedProps[prop] = val
			}
		}

		if len(changedProps) > 0 {
			tokenDiffs = append(tokenDiffs, model.CSSConditionBlock{
				Condition: fmt.Sprintf("@media (%s: %s)", condition.Name, condition.Value),
				Rules: []model.CSSRule{{
					Selector: ":root",
					Props:    changedProps,
				}},
			})
			diffedConditions = append(diffedConditions, condition)
		}
	}

	// Restore browser to clean state.
	_ = applyEmulation(ctx, nil)

	return blocks, tokenDiffs, diffedConditions, nil
}

// CountTokensInBlocks counts all CSS custom properties across a CSSBlock tree.
func CountTokensInBlocks(blocks []model.CSSBlock) int {
	count := 0
	var walk func(b model.CSSBlock)
	walk = func(b model.CSSBlock) {
		for _, rule := range b.Rules {
			for prop := range rule.Props {
				if strings.HasPrefix(prop, "--") {
					count++
				}
			}
		}
		for _, child := range b.Blocks {
			walk(child)
		}
	}
	for _, b := range blocks {
		walk(b)
	}
	return count
}
