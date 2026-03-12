// detail_level.go provides shared constants and parsing for the detail_level
// parameter used across memory and SDD tools.
//
// Three verbosity levels enable progressive disclosure (Anthropic, 2025):
//   - summary: minimal tokens — IDs, titles, metadata only
//   - standard: default behavior — truncated content snippets
//   - full: complete untruncated content for deep analysis
package memory

import "fmt"

// Detail level constants.
const (
	DetailSummary  = "summary"
	DetailStandard = "standard"
	DetailFull     = "full"
)

// DetailLevelValues returns the enum values for MCP tool definitions.
// Use this to avoid duplicating the list across tool definitions.
func DetailLevelValues() []string {
	return []string{DetailSummary, DetailStandard, DetailFull}
}

// ParseDetailLevel normalizes a detail_level string, defaulting to "standard"
// for empty or unrecognized values.
func ParseDetailLevel(s string) string {
	switch s {
	case DetailSummary, DetailFull:
		return s
	default:
		return DetailStandard
	}
}

// SummaryFooter is appended to summary-mode responses to guide the AI
// toward progressive disclosure — fetch more detail only when needed.
const SummaryFooter = "\n---\n💡 Use detail_level: standard or full for more detail."

// NavigationHint returns a one-line footer when results are capped by a limit.
// Returns an empty string when all results fit (showing >= total) or total is 0.
// The hint parameter provides tool-specific guidance (e.g., "Use mem_get #ID for full content.").
func NavigationHint(showing, total int, hint string) string {
	if total <= 0 || showing >= total {
		return ""
	}
	if hint != "" {
		return fmt.Sprintf("\n📊 Showing %d of %d. %s", showing, total, hint)
	}
	return fmt.Sprintf("\n📊 Showing %d of %d.", showing, total)
}

// ─── Token Estimation ───────────────────────────────────────────────────────

// EstimateTokens approximates the token count for a text string using the
// chars/4 heuristic (standard approximation for GPT/Claude tokenizers).
// Returns 0 for empty strings, at least 1 for non-empty strings.
// This is O(1) — uses len() only, no iteration.
func EstimateTokens(text string) int {
	n := len(text)
	if n == 0 {
		return 0
	}
	tokens := n / 4
	if tokens == 0 {
		return 1
	}
	return tokens
}

// TokenFooter returns a one-line footer with the estimated token count
// for a tool response. Appended to all read-heavy tool responses to give
// the AI visibility into context cost.
func TokenFooter(estimatedTokens int) string {
	return fmt.Sprintf("\n📏 ~%s tokens", formatNumber(estimatedTokens))
}

// BudgetFooter returns a footer indicating that a response was truncated
// due to a token budget constraint. Includes tokens used, budget, and
// items shown vs total.
func BudgetFooter(tokensUsed, budget, shown, total int) string {
	return fmt.Sprintf("\n⚡ Budget: ~%s/%s tokens used. %d of %d items shown. Increase max_tokens or use detail_level=summary for more.",
		formatNumber(tokensUsed), formatNumber(budget), shown, total)
}

// formatNumber formats an integer with comma separators for readability.
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	// Insert commas from right to left.
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
