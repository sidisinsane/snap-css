package extract

import (
	"strings"

	"github.com/sidisinsane/snap-css/internal/model"
)

// ExtractTokens filters a CSSBlock tree to custom properties only,
// and deduplicates TokenDiffs against what the traversal already captured.
func ExtractTokens(blocks []model.CSSBlock, tokenDiffs []model.CSSConditionBlock) ([]model.CSSBlock, []model.CSSConditionBlock) {
	filteredBlocks := filterTokenBlocks(blocks)

	// Build a set of already-authored token+condition combinations
	// from the block tree to avoid duplicating in TokenDiffs.
	authored := buildAuthoredTokenSet(filteredBlocks)

	var dedupedDiffs []model.CSSConditionBlock
	for _, diff := range tokenDiffs {
		var dedupedRules []model.CSSRule
		for _, rule := range diff.Rules {
			deduped := make(map[string]string)
			for prop, val := range rule.Props {
				key := diff.Condition + "|" + rule.Selector + "|" + prop
				if !authored[key] {
					deduped[prop] = val
				}
			}
			if len(deduped) > 0 {
				dedupedRules = append(dedupedRules, model.CSSRule{
					Selector: rule.Selector,
					Props:    deduped,
				})
			}
		}
		if len(dedupedRules) > 0 {
			dedupedDiffs = append(dedupedDiffs, model.CSSConditionBlock{
				Condition: diff.Condition,
				Rules:     dedupedRules,
			})
		}
	}

	return filteredBlocks, dedupedDiffs
}

// filterTokenBlocks returns a copy of the block tree containing only
// rules that have at least one custom property. Blocks that become
// empty after filtering are omitted.
func filterTokenBlocks(blocks []model.CSSBlock) []model.CSSBlock {
	var result []model.CSSBlock
	for _, block := range blocks {
		filtered := filterTokenBlock(block)
		if len(filtered.Rules) > 0 || len(filtered.Blocks) > 0 {
			result = append(result, filtered)
		}
	}
	return result
}

func filterTokenBlock(block model.CSSBlock) model.CSSBlock {
	out := model.CSSBlock{Condition: block.Condition}
	for _, rule := range block.Rules {
		filtered := make(map[string]string)
		for prop, val := range rule.Props {
			if strings.HasPrefix(prop, "--") {
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
		fc := filterTokenBlock(child)
		if len(fc.Rules) > 0 || len(fc.Blocks) > 0 {
			out.Blocks = append(out.Blocks, fc)
		}
	}
	return out
}

// buildAuthoredTokenSet builds a set of "condition|selector|property" keys
// from the filtered token block tree for deduplication against TokenDiffs.
func buildAuthoredTokenSet(blocks []model.CSSBlock) map[string]bool {
	set := make(map[string]bool)
	var walk func(b model.CSSBlock, condition string)
	walk = func(b model.CSSBlock, condition string) {
		cond := condition
		if b.Condition != "" {
			cond = b.Condition
		}
		for _, rule := range b.Rules {
			for prop := range rule.Props {
				set[cond+"|"+rule.Selector+"|"+prop] = true
			}
		}
		for _, child := range b.Blocks {
			walk(child, cond)
		}
	}
	for _, b := range blocks {
		walk(b, "")
	}
	return set
}
